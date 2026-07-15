package auth

import (
	"testing"
	"time"
)

func TestJWTManagerIssuesAndParsesAccessToken(t *testing.T) {
	manager, err := NewJWTManager(
		"test-secret-with-at-least-32-characters",
		"leadflow-api",
		"leadflow-client",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("create JWT manager: %v", err)
	}

	now := time.Date(
		2026,
		time.July,
		15,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	manager.now = func() time.Time {
		return now
	}

	token, err := manager.IssueAccessToken(10)
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	if token.Value == "" {
		t.Fatal("expected access token")
	}

	if !token.ExpiresAt.Equal(
		now.Add(15 * time.Minute),
	) {
		t.Fatalf(
			"unexpected expiration %s",
			token.ExpiresAt,
		)
	}

	userID, err := manager.ParseAccessToken(
		token.Value,
	)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if userID != 10 {
		t.Fatalf(
			"expected user ID 10, got %d",
			userID,
		)
	}
}

func TestJWTManagerRejectsExpiredToken(t *testing.T) {
	manager, err := NewJWTManager(
		"test-secret-with-at-least-32-characters",
		"leadflow-api",
		"leadflow-client",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(
		2026,
		time.July,
		15,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	manager.now = func() time.Time {
		return now
	}

	token, err := manager.IssueAccessToken(1)
	if err != nil {
		t.Fatal(err)
	}

	manager.now = func() time.Time {
		return now.Add(2 * time.Minute)
	}

	if _, err := manager.ParseAccessToken(
		token.Value,
	); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}
