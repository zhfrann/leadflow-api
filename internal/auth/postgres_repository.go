package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zhfrann/leadflow-api/internal/notification"
)

const (
	userEmailUniqueConstraint = "users_email_unique_idx"
)

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
