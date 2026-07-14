package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxMessage struct {
	ID             int64
	EventType      string
	RecipientEmail string
	Payload        json.RawMessage
	AttemptCount   int
}

type Repository interface {
	ClaimPending(
		ctx context.Context,
		workerID string,
	) (OutboxMessage, bool, error)

	MarkSent(
		ctx context.Context,
		outboxID int64,
		workerID string,
	) error

	MarkRetry(
		ctx context.Context,
		outboxID int64,
		workerID string,
		nextAttemptAt time.Time,
		lastError string,
	) error

	MarkFailed(
		ctx context.Context,
		outboxID int64,
		workerID string,
		lastError string,
	) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(
	pool *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) ClaimPending(
	ctx context.Context,
	workerID string,
) (OutboxMessage, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return OutboxMessage{}, false, fmt.Errorf(
			"begin outbox claim transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var message OutboxMessage

	err = tx.QueryRow(
		ctx,
		`
			SELECT
				id,
				event_type,
				recipient_email,
				payload,
				attempt_count
			FROM email_outbox
			WHERE status = 'PENDING'
			  AND next_attempt_at <= NOW()
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`,
	).Scan(
		&message.ID,
		&message.EventType,
		&message.RecipientEmail,
		&message.Payload,
		&message.AttemptCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OutboxMessage{}, false, nil
	}

	if err != nil {
		return OutboxMessage{}, false, fmt.Errorf(
			"select pending outbox: %w",
			err,
		)
	}

	_, err = tx.Exec(
		ctx,
		`
			UPDATE email_outbox
			SET
				status = 'PROCESSING',
				processing_started_at = NOW(),
				locked_at = NOW(),
				locked_by = $2,
				updated_at = NOW()
			WHERE id = $1
		`,
		message.ID,
		workerID,
	)
	if err != nil {
		return OutboxMessage{}, false, fmt.Errorf(
			"mark outbox processing: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return OutboxMessage{}, false, fmt.Errorf(
			"commit outbox claim: %w",
			err,
		)
	}

	return message, true, nil
}

func (r *PostgresRepository) MarkSent(
	ctx context.Context,
	outboxID int64,
	workerID string,
) error {
	commandTag, err := r.pool.Exec(
		ctx,
		`
			UPDATE email_outbox
			SET
				status = 'SENT',
				sent_at = NOW(),
				last_error = NULL,
				processing_started_at = NULL,
				locked_at = NULL,
				locked_by = NULL,
				updated_at = NOW()
			WHERE id = $1
			  AND status = 'PROCESSING'
			  AND locked_by = $2
		`,
		outboxID,
		workerID,
	)
	if err != nil {
		return fmt.Errorf("mark outbox sent: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("outbox message is no longer owned")
	}

	return nil
}

func (r *PostgresRepository) MarkRetry(
	ctx context.Context,
	outboxID int64,
	workerID string,
	nextAttemptAt time.Time,
	lastError string,
) error {
	commandTag, err := r.pool.Exec(
		ctx,
		`
			UPDATE email_outbox
			SET
				status = 'PENDING',
				attempt_count = attempt_count + 1,
				next_attempt_at = $3,
				last_error = LEFT($4, 1000),
				processing_started_at = NULL,
				locked_at = NULL,
				locked_by = NULL,
				updated_at = NOW()
			WHERE id = $1
			  AND status = 'PROCESSING'
			  AND locked_by = $2
		`,
		outboxID,
		workerID,
		nextAttemptAt,
		lastError,
	)
	if err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("outbox message is no longer owned")
	}

	return nil
}

func (r *PostgresRepository) MarkFailed(
	ctx context.Context,
	outboxID int64,
	workerID string,
	lastError string,
) error {
	commandTag, err := r.pool.Exec(
		ctx,
		`
			UPDATE email_outbox
			SET
				status = 'FAILED',
				attempt_count = attempt_count + 1,
				last_error = LEFT($3, 1000),
				processing_started_at = NULL,
				locked_at = NULL,
				locked_by = NULL,
				updated_at = NOW()
			WHERE id = $1
			  AND status = 'PROCESSING'
			  AND locked_by = $2
		`,
		outboxID,
		workerID,
		lastError,
	)
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("outbox message is no longer owned")
	}

	return nil
}
