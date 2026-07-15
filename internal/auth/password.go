package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type BcryptHasher struct {
	cost int
}

func (h *BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func (h *BcryptHasher) Compare(passwordHash string, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return fmt.Errorf("compare password: %w", err)
	}

	return nil
}

func (h *BcryptHasher) Verify(passwordHash string, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))

	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, fmt.Errorf(
			"verify password: %w",
			err,
		)
	}
}

func NewBcryptHasher(cost int) (*BcryptHasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("bcypt cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost)
	}

	return &BcryptHasher{cost: cost}, nil
}
