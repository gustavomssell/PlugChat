package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chatfree/api/internal/model"
)

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "trailing slash", input: "https://Example.com/", want: "https://example.com", wantOK: true},
		{name: "missing scheme", input: "example.com", want: "https://example.com", wantOK: true},
		{name: "localhost with port", input: "http://localhost:5173/", want: "http://localhost:5173", wantOK: true},
		{name: "default https port", input: "https://example.com:443", want: "https://example.com", wantOK: true},
		{name: "default http port", input: "http://example.com:80", want: "http://example.com", wantOK: true},
		{name: "wildcard", input: "*", want: "*", wantOK: true},
		{name: "path", input: "https://example.com/chat", wantOK: false},
		{name: "query", input: "https://example.com?x=1", wantOK: false},
		{name: "fragment", input: "https://example.com#top", wantOK: false},
		{name: "null origin", input: "null", wantOK: false},
		{name: "unsupported scheme", input: "ftp://example.com", wantOK: false},
		{name: "wildcard host", input: "https://*.example.com", wantOK: false},
		{name: "user info", input: "https://user:pass@example.com", wantOK: false},
		{name: "empty", input: "   ", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := normalizeOrigin(test.input)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if ok && got != test.want {
				t.Fatalf("origin = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSanitizeOrigins(t *testing.T) {
	origins, message := sanitizeOrigins([]string{"https://Example.com/", "https://example.com", "example.org", ""})
	if message != "" {
		t.Fatalf("message = %q", message)
	}
	if len(origins) != 2 || origins[0] != "https://example.com" || origins[1] != "https://example.org" {
		t.Fatalf("origins = %+v", origins)
	}

	wildcard, message := sanitizeOrigins([]string{"https://example.com", "*"})
	if message != "" || len(wildcard) != 1 || wildcard[0] != "*" {
		t.Fatalf("wildcard = %+v message = %q", wildcard, message)
	}

	if _, message := sanitizeOrigins([]string{"https://example.com/path"}); message == "" {
		t.Fatal("origem com caminho deveria ser rejeitada")
	}
}

func TestOriginAllowed(t *testing.T) {
	channel := model.Channel{AllowedOrigins: []string{"https://allowed.test"}}

	request := func(origin string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/widget/config/key", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}

	if originAllowed(channel, request("https://allowed.test")) != true {
		t.Fatal("origem permitida deveria passar")
	}
	if originAllowed(channel, request("https://allowed.test/")) != true {
		t.Fatal("barra final deveria ser normalizada")
	}
	if originAllowed(channel, request("https://blocked.test")) != false {
		t.Fatal("origem bloqueada deveria falhar")
	}
	if originAllowed(channel, request("")) != false {
		t.Fatal("origem ausente deveria falhar quando há allowlist")
	}

	open := model.Channel{}
	if !originAllowed(open, request("")) {
		t.Fatal("canal sem allowlist deveria aceitar qualquer origem")
	}
	wildcard := model.Channel{AllowedOrigins: []string{"*"}}
	if !originAllowed(wildcard, request("")) {
		t.Fatal("curinga deveria aceitar origem ausente")
	}
}

func TestClientIP(t *testing.T) {
	newRequest := func(remote string, headers map[string]string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		for key, value := range headers {
			r.Header.Set(key, value)
		}
		return r
	}

	if got := clientIP(newRequest("203.0.113.9:5123", map[string]string{"X-Forwarded-For": "198.51.100.7"}), false); got != "203.0.113.9" {
		t.Fatalf("sem proxy ip = %q", got)
	}
	if got := clientIP(newRequest("10.0.0.2:5123", map[string]string{"X-Forwarded-For": "198.51.100.7, 10.0.0.1"}), true); got != "198.51.100.7" {
		t.Fatalf("com proxy ip = %q", got)
	}
	if got := clientIP(newRequest("10.0.0.2:5123", map[string]string{"X-Forwarded-For": "garbage"}), true); got != "10.0.0.2" {
		t.Fatalf("forwarded inválido deveria usar RemoteAddr, got %q", got)
	}
	if got := clientIP(newRequest("10.0.0.2:5123", map[string]string{"X-Real-IP": "198.51.100.8"}), true); got != "198.51.100.8" {
		t.Fatalf("x-real-ip = %q", got)
	}
}

func TestWidgetGuardRejectsExcess(t *testing.T) {
	guard := newWidgetGuard(WidgetPolicy{ConfigPerMinute: 1})
	request := httptest.NewRequest(http.MethodGet, "/api/widget/config/key", nil)
	request.RemoteAddr = "203.0.113.9:5123"

	first := httptest.NewRecorder()
	if !guard.allowConfig(first, request, "key") {
		t.Fatal("primeira requisição deveria passar")
	}

	second := httptest.NewRecorder()
	if guard.allowConfig(second, request, "key") {
		t.Fatal("segunda requisição deveria ser bloqueada")
	}
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After deveria estar presente")
	}
}

func TestReadWidgetJSON(t *testing.T) {
	valid := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"text":"oi"}`))
	var body struct {
		Text string `json:"text"`
	}
	if err := readWidgetJSON(httptest.NewRecorder(), valid, &body); err != nil {
		t.Fatalf("corpo válido: %v", err)
	}
	if body.Text != "oi" {
		t.Fatalf("text = %q", body.Text)
	}

	multiple := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"text":"oi"}{"text":"tchau"}`))
	if err := readWidgetJSON(httptest.NewRecorder(), multiple, &body); !errors.Is(err, errMultipleJSONValues) {
		t.Fatalf("múltiplos valores: %v", err)
	}

	largeBody := `{"text":"` + strings.Repeat("a", maxWidgetBodyBytes) + `"}`
	large := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(largeBody))
	err := readWidgetJSON(httptest.NewRecorder(), large, &body)
	var tooLarge *http.MaxBytesError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("corpo grande: %v", err)
	}
}
