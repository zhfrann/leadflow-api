package auth_test

import (
	"testing"

	"github.com/zhfrann/leadflow-api/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasherHashesAndComparesPassword(t *testing.T) {
	hasher, err := auth.NewBcryptHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("create bcrypt hasher: %v", err)
	}

	password := "very-secure-password"

	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == password {
		t.Fatal("password hash must differ from plaintext")
	}

	if err := hasher.Compare(hash, password); err != nil {
		t.Fatalf("expected password to match: %v", err)
	}
}

func TestBcryptHasherRejectsIncorrectPassword(t *testing.T) {
	hasher, err := auth.NewBcryptHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("create bcrypt hasher: %v", err)
	}

	hash, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if err := hasher.Compare(hash, "incorrect-password"); err == nil {
		t.Fatal("expected password comparison to fail")
	}
}
