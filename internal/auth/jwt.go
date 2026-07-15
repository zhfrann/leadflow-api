package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const minimumJWTSecretLength = 32

type IssuedToken struct {
	Value     string
	ExpiresAt time.Time
}

type AccessTokenClaims struct {
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

func NewJWTManager(
	secret string,
	issuer string,
	audience string,
	ttl time.Duration,
) (*JWTManager, error) {
	if len([]byte(secret)) < minimumJWTSecretLength {
		return nil, fmt.Errorf("JWT secret must contain at least %d bytes", minimumJWTSecretLength)
	}

	if strings.TrimSpace(issuer) == "" {
		return nil, fmt.Errorf("JWT issuer must not be empty")
	}

	if strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("JWT audience must not be empty")
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("JWT access TTL must be greater than zero")
	}

	return &JWTManager{
		secret:   []byte(secret),
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
		now:      time.Now,
	}, nil
}

func (m *JWTManager) IssueAccessToken(userID int64) (IssuedToken, error) {
	if userID <= 0 {
		return IssuedToken{}, fmt.Errorf("user ID must be greater than zero")
	}

	now := m.now().UTC().Truncate(time.Second)
	expiresAt := now.Add(m.ttl)

	tokenID, err := newTokenID()
	if err != nil {
		return IssuedToken{}, err
	}

	claims := AccessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        tokenID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(m.secret)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("sign access token: %w", err)
	}

	return IssuedToken{
		Value:     signedToken,
		ExpiresAt: expiresAt,
	}, nil
}

func (m *JWTManager) ParseAccessToken(tokenValue string) (int64, error) {
	claims := &AccessTokenClaims{}

	token, err := jwt.ParseWithClaims(
		tokenValue,
		claims,
		func(_ *jwt.Token) (any, error) {
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return 0, fmt.Errorf(
			"parse access token: %w",
			err,
		)
	}

	if !token.Valid {
		return 0, fmt.Errorf("access token is invalid")
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, fmt.Errorf("access token contains invalid subject")
	}

	return userID, nil
}

func newTokenID() (string, error) {
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate token ID: %w", err)
	}

	return hex.EncodeToString(randomBytes), nil
}
