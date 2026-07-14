package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhfrann/leadflow-api/internal/auth"
)

type stubHasher struct {
	receivedPassword string
	hash             string
	err              error
}

func (s *stubHasher) Hash(password string) (string, error) {
	s.receivedPassword = password
	return s.hash, s.err
}

type stubRepository struct {
	receivedParams auth.CreateUserParams
	user           auth.User
	err            error
	called         bool
}

func (s *stubRepository) CreateUserWithWelcomeEmail(_ context.Context, params auth.CreateUserParams) (auth.User, error) {
	s.called = true
	s.receivedParams = params

	return s.user, s.err
}

func TestRegisterNormalizesEmailAndHashesPassword(
	t *testing.T,
) {
	hasher := &stubHasher{
		hash: "hashed-password",
	}

	repository := &stubRepository{
		user: auth.User{
			ID:        1,
			Email:     "user@example.com",
			CreatedAt: time.Now(),
		},
	}

	service := auth.NewService(repository, hasher)

	user, err := service.Register(
		context.Background(),
		auth.RegisterInput{
			Email:    "  User@Example.com ",
			Password: "very-secure-password",
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.ID != 1 {
		t.Errorf("expected user ID 1, got %d", user.ID)
	}

	if hasher.receivedPassword != "very-secure-password" {
		t.Errorf(
			"expected original password to be hashed, got %q",
			hasher.receivedPassword,
		)
	}

	if repository.receivedParams.Email !=
		"user@example.com" {
		t.Errorf(
			"expected normalized email, got %q",
			repository.receivedParams.Email,
		)
	}

	if repository.receivedParams.PasswordHash !=
		"hashed-password" {
		t.Errorf(
			"expected password hash, got %q",
			repository.receivedParams.PasswordHash,
		)
	}
}

func TestRegisterRejectsInvalidEmail(t *testing.T) {
	repository := &stubRepository{}
	hasher := &stubHasher{}

	service := auth.NewService(repository, hasher)

	_, err := service.Register(
		context.Background(),
		auth.RegisterInput{
			Email:    "invalid-email",
			Password: "very-secure-password",
		},
	)

	if !errors.Is(err, auth.ErrInvalidEmail) {
		t.Fatalf(
			"expected ErrInvalidEmail, got %v",
			err,
		)
	}

	if repository.called {
		t.Fatal("repository must not be called")
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	repository := &stubRepository{}
	hasher := &stubHasher{}

	service := auth.NewService(repository, hasher)

	_, err := service.Register(
		context.Background(),
		auth.RegisterInput{
			Email:    "user@example.com",
			Password: "short",
		},
	)

	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Fatalf(
			"expected ErrPasswordTooShort, got %v",
			err,
		)
	}

	if repository.called {
		t.Fatal("repository must not be called")
	}
}

func TestRegisterPropagatesRepositoryError(
	t *testing.T,
) {
	repositoryError := errors.New("database failure")

	repository := &stubRepository{
		err: repositoryError,
	}

	hasher := &stubHasher{
		hash: "hashed-password",
	}

	service := auth.NewService(repository, hasher)

	_, err := service.Register(
		context.Background(),
		auth.RegisterInput{
			Email:    "user@example.com",
			Password: "very-secure-password",
		},
	)

	if !errors.Is(err, repositoryError) {
		t.Fatalf(
			"expected repository error, got %v",
			err,
		)
	}
}
