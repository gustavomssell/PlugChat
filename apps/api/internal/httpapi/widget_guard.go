package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chatfree/api/internal/ratelimit"
)

// maxWidgetBodyBytes caps the JSON body accepted by public widget endpoints.
// The largest legitimate payload is a visitor message, which is additionally
// limited by rune count in the chat service.
const maxWidgetBodyBytes = 16 << 10

var errMultipleJSONValues = errors.New("corpo deve conter um único objeto JSON")

// WidgetPolicy controls the abuse protection applied to public widget routes.
// All limits are per process and per key (IP, channel or conversation).
type WidgetPolicy struct {
	// TrustProxy enables X-Forwarded-For and X-Real-IP. Only enable it when
	// the API sits behind a trusted reverse proxy; otherwise clients can spoof
	// their IP and bypass every per-IP limit.
	TrustProxy bool

	ConfigPerMinute                  int
	StartsPerTenMinutesPerIP         int
	StartsPerHourPerChannel          int
	ReadsPerMinutePerConversation    int
	MessagesPerMinutePerIP           int
	MessagesPerMinutePerConversation int
}

// DefaultWidgetPolicy returns conservative limits for a single API instance.
func DefaultWidgetPolicy() WidgetPolicy {
	return WidgetPolicy{
		ConfigPerMinute:                  120,
		StartsPerTenMinutesPerIP:         10,
		StartsPerHourPerChannel:          120,
		ReadsPerMinutePerConversation:    120,
		MessagesPerMinutePerIP:           20,
		MessagesPerMinutePerConversation: 60,
	}
}

func (p WidgetPolicy) normalized() WidgetPolicy {
	defaults := DefaultWidgetPolicy()

	if p.ConfigPerMinute <= 0 {
		p.ConfigPerMinute = defaults.ConfigPerMinute
	}
	if p.StartsPerTenMinutesPerIP <= 0 {
		p.StartsPerTenMinutesPerIP = defaults.StartsPerTenMinutesPerIP
	}
	if p.StartsPerHourPerChannel <= 0 {
		p.StartsPerHourPerChannel = defaults.StartsPerHourPerChannel
	}
	if p.ReadsPerMinutePerConversation <= 0 {
		p.ReadsPerMinutePerConversation = defaults.ReadsPerMinutePerConversation
	}
	if p.MessagesPerMinutePerIP <= 0 {
		p.MessagesPerMinutePerIP = defaults.MessagesPerMinutePerIP
	}
	if p.MessagesPerMinutePerConversation <= 0 {
		p.MessagesPerMinutePerConversation = defaults.MessagesPerMinutePerConversation
	}

	return p
}

type widgetGuard struct {
	policy WidgetPolicy

	config        *ratelimit.Limiter
	starts        *ratelimit.Limiter
	channelStarts *ratelimit.Limiter
	reads         *ratelimit.Limiter
	messages      *ratelimit.Limiter
	conversation  *ratelimit.Limiter
}

func newWidgetGuard(policy WidgetPolicy) *widgetGuard {
	policy = policy.normalized()

	return &widgetGuard{
		policy:        policy,
		config:        ratelimit.New(ratelimit.Config{Limit: policy.ConfigPerMinute, Window: time.Minute}),
		starts:        ratelimit.New(ratelimit.Config{Limit: policy.StartsPerTenMinutesPerIP, Window: 10 * time.Minute}),
		channelStarts: ratelimit.New(ratelimit.Config{Limit: policy.StartsPerHourPerChannel, Window: time.Hour}),
		reads:         ratelimit.New(ratelimit.Config{Limit: policy.ReadsPerMinutePerConversation, Window: time.Minute}),
		messages:      ratelimit.New(ratelimit.Config{Limit: policy.MessagesPerMinutePerIP, Window: time.Minute}),
		conversation:  ratelimit.New(ratelimit.Config{Limit: policy.MessagesPerMinutePerConversation, Window: time.Minute}),
	}
}

func (g *widgetGuard) allowConfig(w http.ResponseWriter, r *http.Request, channelKey string) bool {
	return enforceRateLimit(w, g.config, "config|"+shortKey(channelKey)+"|"+g.clientIP(r))
}

// allowStartIP limits how many conversations a single visitor address may open.
// It runs before any database access so abusive payloads never reach Postgres.
func (g *widgetGuard) allowStartIP(w http.ResponseWriter, r *http.Request, channelKey string) bool {
	return enforceRateLimit(w, g.starts, "start|"+shortKey(channelKey)+"|"+g.clientIP(r))
}

// allowStartChannel is a channel-wide safety valve. It runs only after the
// origin has been accepted, so a request with a forbidden origin cannot spend
// the budget of legitimate visitors.
func (g *widgetGuard) allowStartChannel(w http.ResponseWriter, channelKey string) bool {
	return enforceRateLimit(w, g.channelStarts, "starts|"+shortKey(channelKey))
}

func (g *widgetGuard) allowRead(w http.ResponseWriter, r *http.Request, conversationID string) bool {
	return enforceRateLimit(w, g.reads, "read|"+shortKey(conversationID)+"|"+g.clientIP(r))
}

func (g *widgetGuard) allowMessage(w http.ResponseWriter, r *http.Request, conversationID string) bool {
	ip := g.clientIP(r)
	conversationID = shortKey(conversationID)
	if !enforceRateLimit(w, g.messages, "message|"+conversationID+"|"+ip) {
		return false
	}
	return enforceRateLimit(w, g.conversation, "messages|"+conversationID)
}

// shortKey keeps limiter map keys bounded even when a client sends a very long
// path segment.
func shortKey(value string) string {
	const maxKeyLength = 128
	if len(value) > maxKeyLength {
		return value[:maxKeyLength]
	}
	return value
}

func (g *widgetGuard) clientIP(r *http.Request) string {
	return clientIP(r, g.policy.TrustProxy)
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			for _, part := range strings.Split(forwarded, ",") {
				if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
					return ip.String()
				}
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if host = strings.TrimSpace(r.RemoteAddr); host != "" {
		return host
	}
	return "unknown"
}

func enforceRateLimit(w http.ResponseWriter, limiter *ratelimit.Limiter, key string) bool {
	allowed, retryAfter := limiter.Allow(key)
	if allowed {
		return true
	}

	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "muitas requisições para este widget, tente novamente em instantes")
	return false
}

// readWidgetJSON reads a size-limited JSON body and rejects trailing values.
func readWidgetJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxWidgetBodyBytes))

	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errMultipleJSONValues
	}

	return nil
}

func writeWidgetBodyError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "corpo da requisição muito grande")
		return
	}

	writeError(w, http.StatusBadRequest, "corpo inválido")
}
