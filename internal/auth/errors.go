package auth

import "errors"

var (
	ErrInvalidEmail       = errors.New("email is invalid")
	ErrPasswordTooShort   = errors.New("password must contain at least 12 characters")
	ErrPasswordTooLong    = errors.New("password must not exceed 72 bytes")
	ErrEmailAlreadyExists = errors.New("email is already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")

	errUserNotFound = errors.New("user not found")
)
