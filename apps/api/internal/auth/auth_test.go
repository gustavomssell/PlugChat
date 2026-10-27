package auth

import (
	"errors"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	manager := NewManager("secret", time.Hour)

	token, err := manager.Issue("user_1", "ws_1", "owner")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if claims.Subject != "user_1" || claims.WorkspaceID != "ws_1" || claims.Role != "owner" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestTokenTampered(t *testing.T) {
	manager := NewManager("secret", time.Hour)

	token, err := manager.Issue("user_1", "ws_1", "agent")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	tampered := token[:len(token)-1] + "x"
	if _, err := manager.Parse(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestTokenExpired(t *testing.T) {
	manager := NewManager("secret", -time.Minute)

	token, err := manager.Issue("user_1", "ws_1", "agent")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := manager.Parse(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("err = %v, want ErrExpiredToken", err)
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("demo1234")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if hash == "demo1234" || !CheckPassword(hash, "demo1234") {
		t.Fatal("password hash not applied")
	}

	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}
