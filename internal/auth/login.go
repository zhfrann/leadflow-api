package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	User        User
	AccessToken string
	ExpiresAt   time.Time
}

type LoginRepository interface {
	FindUserByEmail(ctx context.Context, email string) (CredentialUser, error)
}

type PasswordVerifier interface {
	Verify(passwordHash string, password string) (bool, error)
}

type AccessTokenIssuer interface {
	IssueAccessToken(userID int64) (IssuedToken, error)
}

type LoginService struct {
	repository        LoginRepository
	passwordVerifier  PasswordVerifier
	tokenIssuer       AccessTokenIssuer
	dummyPasswordHash string
}

func NewLoginService(
	repository LoginRepository,
	passwordVerifier PasswordVerifier,
	tokenIssuer AccessTokenIssuer,
	dummyPasswordHash string,
) *LoginService {
	return &LoginService{
		repository:        repository,
		passwordVerifier:  passwordVerifier,
		tokenIssuer:       tokenIssuer,
		dummyPasswordHash: dummyPasswordHash,
	}
}

func (s *LoginService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email := normalizeEmail(input.Email)

	if !isValidEmail(email) ||
		strings.TrimSpace(input.Password) == "" {
		s.performDummyPasswordCheck(input.Password)

		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := s.repository.FindUserByEmail(
		ctx,
		email,
	)
	if errors.Is(err, errUserNotFound) {
		s.performDummyPasswordCheck(input.Password)

		return LoginResult{}, ErrInvalidCredentials
	}

	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"find login user: %w",
			err,
		)
	}

	matches, err := s.passwordVerifier.Verify(
		user.PasswordHash,
		input.Password,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"verify login password: %w",
			err,
		)
	}

	if !matches {
		return LoginResult{}, ErrInvalidCredentials
	}

	accessToken, err := s.tokenIssuer.IssueAccessToken(
		user.ID,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"issue access token: %w",
			err,
		)
	}

	return LoginResult{
		User: User{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
		AccessToken: accessToken.Value,
		ExpiresAt:   accessToken.ExpiresAt,
	}, nil
}

func (s *LoginService) performDummyPasswordCheck(password string) {
	_, _ = s.passwordVerifier.Verify(
		s.dummyPasswordHash,
		password,
	)
}
