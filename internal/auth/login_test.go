package auth

import (
	"context"
	"testing"
	"time"
)

type stubLoginRepository struct {
	user CredentialUser
	err  error
}

func (s stubLoginRepository) FindUserByEmail(_ context.Context, _ string) (CredentialUser, error) {
	return s.user, s.err
}

type stubPasswordVerifier struct {
	matches bool
	err     error
}

func (s stubPasswordVerifier) Verify(_ string, _ string) (bool, error) {
	return s.matches, s.err
}

type stubTokenIssuer struct {
	token IssuedToken
	err   error
}

func (s stubTokenIssuer) IssueAccessToken(_ int64) (IssuedToken, error) {
	return s.token, s.err
}

func TestLoginReturnsAccessToken(t *testing.T) {
	expiresAt := time.Now().Add(15 * time.Minute)

	service := NewLoginService(
		stubLoginRepository{
			user: CredentialUser{
				ID:           1,
				Email:        "user@example.com",
				PasswordHash: "hash",
				CreatedAt:    time.Now(),
			},
		},
		stubPasswordVerifier{
			matches: true,
		},
		stubTokenIssuer{
			token: IssuedToken{
				Value:     "access-token",
				ExpiresAt: expiresAt,
			},
		},
		"dummy-hash",
	)

	result, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "User@Example.com",
			Password: "very-secure-password",
		},
	)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if result.AccessToken != "access-token" {
		t.Fatalf(
			"unexpected access token %q",
			result.AccessToken,
		)
	}

	if result.User.ID != 1 {
		t.Fatalf(
			"expected user ID 1, got %d",
			result.User.ID,
		)
	}
}

func TestLoginRejectsIncorrectPassword(t *testing.T) {
	service := NewLoginService(
		stubLoginRepository{
			user: CredentialUser{
				ID:           1,
				Email:        "user@example.com",
				PasswordHash: "hash",
			},
		},
		stubPasswordVerifier{
			matches: false,
		},
		stubTokenIssuer{},
		"dummy-hash",
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "user@example.com",
			Password: "incorrect-password",
		},
	)

	if err != ErrInvalidCredentials {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}
