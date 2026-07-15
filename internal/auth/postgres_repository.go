package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhfrann/leadflow-api/internal/notification"
)

const (
	userEmailUniqueConstraint = "users_email_unique_idx"
)

type CredentialUser struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func (r *PostgresRepository) CreateUserWithWelcomeEmail(ctx context.Context, params CreateUserParams) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin registration transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var user User

	err = tx.QueryRow(
		ctx,
		`
			INSERT INTO users (
				email,
				password_hash
			)
			VALUES ($1, $2)
			RETURNING
				id,
				email,
				created_at
		`,
		params.Email,
		params.PasswordHash,
	).Scan(
		&user.ID,
		&user.Email,
		&user.CreatedAt,
	)
	if err != nil {
		if isEmailUniqueViolation(err) {
			return User{}, ErrEmailAlreadyExists
		}

		return User{}, fmt.Errorf("insert user: %w", err)
	}

	payload, err := json.Marshal(
		notification.UserRegisteredPayload{
			UserID: user.ID,
			Email:  user.Email,
		},
	)
	if err != nil {
		return User{}, fmt.Errorf("marshal welcome email payload: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`
			INSERT INTO email_outbox (
				event_type,
				recipient_email,
				template_name,
				payload
			)
			VALUES (
				'USER_REGISTERED',
				$1,
				'welcome',
				$2::jsonb
			)
		`,
		user.Email,
		string(payload),
	)
	if err != nil {
		return User{}, fmt.Errorf("insert welcome email outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit registration transaction: %w", err)
	}

	return user, nil
}

func (r *PostgresRepository) FindUserByEmail(ctx context.Context, email string) (CredentialUser, error) {
	var user CredentialUser

	err := r.pool.QueryRow(
		ctx,
		`
			SELECT
				id,
				email,
				password_hash,
				created_at
			FROM users
			WHERE email = $1
		`,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return CredentialUser{}, errUserNotFound
	}

	if err != nil {
		return CredentialUser{}, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func isEmailUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return false
	}

	return postgresError.Code == "23505" &&
		postgresError.ConstraintName ==
			userEmailUniqueConstraint
}
