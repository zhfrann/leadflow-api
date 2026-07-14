package auth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const minimumPasswordLength = 12

type User struct {
	ID        int64
	Email     string
	CreatedAt time.Time
}

type RegisterInput struct {
	Email    string
	Password string
}

type CreateUserParams struct {
	Email        string
	PasswordHash string
}

type RegisterRepository interface {
	CreateUserWithWelcomeEmail(ctx context.Context, params CreateUserParams) (User, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type Service struct {
	repository RegisterRepository
	hasher     PasswordHasher
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	email := normalizeEmail(input.Email)

	if !isValidEmail(email) {
		return User{}, ErrInvalidEmail
	}

	if utf8.RuneCountInString(input.Password) < minimumPasswordLength {
		return User{}, ErrPasswordTooShort
	}

	if len([]byte(input.Password)) > 72 {
		return User{}, ErrPasswordTooLong
	}

	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repository.CreateUserWithWelcomeEmail(ctx, CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return User{}, fmt.Errorf("regitser user: %w", err)
	}

	return user, nil
}

func NewService(repository RegisterRepository, hasher PasswordHasher) *Service {
	return &Service{
		repository: repository,
		hasher:     hasher,
	}
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isValidEmail(value string) bool {
	if value == "" || len(value) > 320 {
		return false
	}

	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}

	return (address.Address == value)
}
