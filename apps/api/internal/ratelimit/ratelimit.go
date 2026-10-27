// Package ratelimit implements an in-memory token bucket limiter.
//
// It is intentionally small and dependency free: one Limiter keeps a bucket
// per key and refills it continuously. The limiter is safe for concurrent use
// and expires idle buckets so long-running processes do not grow without
// bound.
//
// The current implementation is per process. Horizontal scaling needs a
// shared store (Redis or the database) before limits become global.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Config describes a token bucket policy.
type Config struct {
	// Limit is the number of requests allowed per Window.
	Limit int
	// Window is the period used to refill Limit tokens.
	Window time.Duration
	// Burst is the maximum number of tokens a key can accumulate.
	// It defaults to Limit when zero.
	Burst int
}

// Limiter is a concurrency-safe token bucket limiter keyed by arbitrary
// strings (IP address, channel key, conversation ID, ...).
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	idleTTL time.Duration
	ops     int

	now func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

const sweepInterval = 512

// New creates a limiter. Invalid values are normalized so a misconfigured
// policy degrades into a conservative default instead of panicking.
func New(cfg Config) *Limiter {
	if cfg.Limit <= 0 {
		cfg.Limit = 1
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.Limit
	}

	windowSeconds := cfg.Window.Seconds()
	idleTTL := 2 * cfg.Window
	if idleTTL < 10*time.Minute {
		idleTTL = 10 * time.Minute
	}

	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(cfg.Limit) / windowSeconds,
		burst:   float64(cfg.Burst),
		idleTTL: idleTTL,
		now:     time.Now,
	}
}

// Allow reports whether a request for key may proceed. When it may not, the
// returned duration is the time until a token becomes available (at least one
// second, suitable for the Retry-After header).
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := l.now()

	l.mu.Lock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		b.last = now
	}

	allowed := b.tokens >= 1
	var retryAfter time.Duration

	if allowed {
		b.tokens--
	} else {
		missing := 1 - b.tokens
		retryAfter = time.Duration(math.Ceil(missing/l.rate)) * time.Second
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
	}

	l.ops++
	if l.ops >= sweepInterval {
		l.ops = 0
		l.sweep(now)
	}

	l.mu.Unlock()

	return allowed, retryAfter
}

// Reset drops every bucket. Mostly useful in tests and when a channel is
// removed.
func (l *Limiter) Reset() {
	l.mu.Lock()
	l.buckets = make(map[string]*bucket)
	l.mu.Unlock()
}

// sweep removes buckets that have been idle for longer than idleTTL. Callers
// must hold l.mu.
func (l *Limiter) sweep(now time.Time) {
	for key, bucket := range l.buckets {
		if now.Sub(bucket.last) > l.idleTTL {
			delete(l.buckets, key)
		}
	}
}
