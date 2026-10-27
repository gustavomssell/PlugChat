package ratelimit

import (
	"testing"
	"time"
)

func TestAllowBurstThenRefill(t *testing.T) {
	limiter := New(Config{Limit: 2, Window: time.Minute})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if ok, _ := limiter.Allow("ip-a"); !ok {
		t.Fatal("primeira requisição deveria ser permitida")
	}
	if ok, _ := limiter.Allow("ip-a"); !ok {
		t.Fatal("segunda requisição deveria ser permitida")
	}

	ok, retryAfter := limiter.Allow("ip-a")
	if ok {
		t.Fatal("terceira requisição deveria ser bloqueada")
	}
	if retryAfter < time.Second || retryAfter > time.Minute {
		t.Fatalf("retryAfter = %v", retryAfter)
	}

	// Other keys have their own bucket.
	if ok, _ := limiter.Allow("ip-b"); !ok {
		t.Fatal("chave distinta deveria ter bucket próprio")
	}

	now = now.Add(30 * time.Second)
	if ok, _ := limiter.Allow("ip-a"); !ok {
		t.Fatal("meia janela deveria repor um token")
	}
	if ok, _ := limiter.Allow("ip-a"); ok {
		t.Fatal("apenas um token deveria ter sido reposto")
	}

	now = now.Add(time.Minute)
	if ok, _ := limiter.Allow("ip-a"); !ok {
		t.Fatal("janela completa deveria repor o bucket")
	}
}

func TestAllowUsesSmallBurst(t *testing.T) {
	limiter := New(Config{Limit: 60, Window: time.Minute, Burst: 2})
	for i := 0; i < 2; i++ {
		if ok, _ := limiter.Allow("ip"); !ok {
			t.Fatalf("requisição %d deveria passar", i+1)
		}
	}
	if ok, _ := limiter.Allow("ip"); ok {
		t.Fatal("rajada acima do burst deveria ser bloqueada")
	}
}

func TestRetryAfterIsAtLeastOneSecond(t *testing.T) {
	limiter := New(Config{Limit: 60, Window: time.Minute})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	for i := 0; i < 60; i++ {
		if ok, _ := limiter.Allow("ip"); !ok {
			t.Fatalf("requisição %d deveria passar", i+1)
		}
	}

	now = now.Add(900 * time.Millisecond)
	ok, retryAfter := limiter.Allow("ip")
	if ok {
		t.Fatal("bucket vazio deveria bloquear")
	}
	if retryAfter != time.Second {
		t.Fatalf("retryAfter = %v, want 1s", retryAfter)
	}
}

func TestSweepExpiresIdleBuckets(t *testing.T) {
	limiter := New(Config{Limit: 10, Window: time.Minute})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	if ok, _ := limiter.Allow("idle"); !ok {
		t.Fatal("primeira requisição deveria passar")
	}

	now = now.Add(11 * time.Minute)
	for i := 0; i < sweepInterval; i++ {
		limiter.Allow("active")
	}

	if _, exists := limiter.buckets["idle"]; exists {
		t.Fatal("bucket inativo deveria ter sido removido")
	}
	if _, exists := limiter.buckets["active"]; !exists {
		t.Fatal("bucket ativo não deveria ter sido removido")
	}
}

func TestResetDropsBuckets(t *testing.T) {
	limiter := New(Config{Limit: 1, Window: time.Minute})
	if ok, _ := limiter.Allow("ip"); !ok {
		t.Fatal("primeira requisição deveria passar")
	}
	if ok, _ := limiter.Allow("ip"); ok {
		t.Fatal("segunda requisição deveria falhar")
	}

	limiter.Reset()

	if ok, _ := limiter.Allow("ip"); !ok {
		t.Fatal("após reset o bucket deveria começar cheio")
	}
}
