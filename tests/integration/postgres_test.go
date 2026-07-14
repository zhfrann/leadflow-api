//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhfrann/leadflow-api/internal/auth"
	"github.com/zhfrann/leadflow-api/internal/notification"
	"golang.org/x/crypto/bcrypt"
)

func TestRegistrationIntegration(t *testing.T) {
	pool := openTestPool(t)

	repository := auth.NewPostgresRepository(pool)

	hasher, err := auth.NewBcryptHasher(bcrypt.MinCost)
	if err != nil {
		t.Fatalf("create bcrypt hasher: %v", err)
	}

	service := auth.NewService(repository, hasher)

	t.Run("creates user and welcome outbox atomically", func(t *testing.T) {
		resetDatabase(t, pool)

		ctx := context.Background()

		user, err := service.Register(
			ctx,
			auth.RegisterInput{
				Email:    "  User@Example.com ",
				Password: "very-secure-password",
			},
		)
		if err != nil {
			t.Fatalf("register user: %v", err)
		}

		if user.Email != "user@example.com" {
			t.Fatalf(
				"expected normalized email, got %q",
				user.Email,
			)
		}

		var (
			storedEmail        string
			storedPasswordHash string
		)

		err = pool.QueryRow(
			ctx,
			`
				SELECT email, password_hash
				FROM users
				WHERE id = $1
			`,
			user.ID,
		).Scan(
			&storedEmail,
			&storedPasswordHash,
		)
		if err != nil {
			t.Fatalf("query created user: %v", err)
		}

		if storedEmail != "user@example.com" {
			t.Errorf(
				"expected stored email user@example.com, got %q",
				storedEmail,
			)
		}

		if storedPasswordHash == "very-secure-password" {
			t.Fatal("password must not be stored as plaintext")
		}

		if err := hasher.Compare(
			storedPasswordHash,
			"very-secure-password",
		); err != nil {
			t.Fatalf("stored password hash does not match: %v", err)
		}

		var (
			eventType      string
			recipientEmail string
			templateName   string
			status         string
			payloadUserID  int64
			payloadEmail   string
		)

		err = pool.QueryRow(
			ctx,
			`
				SELECT
					event_type,
					recipient_email,
					template_name,
					status,
					(payload ->> 'user_id')::BIGINT,
					payload ->> 'email'
				FROM email_outbox
				WHERE recipient_email = $1
			`,
			user.Email,
		).Scan(
			&eventType,
			&recipientEmail,
			&templateName,
			&status,
			&payloadUserID,
			&payloadEmail,
		)
		if err != nil {
			t.Fatalf("query welcome outbox: %v", err)
		}

		if eventType != "USER_REGISTERED" {
			t.Errorf("unexpected event type %q", eventType)
		}

		if recipientEmail != user.Email {
			t.Errorf(
				"expected recipient %q, got %q",
				user.Email,
				recipientEmail,
			)
		}

		if templateName != "welcome" {
			t.Errorf(
				"expected welcome template, got %q",
				templateName,
			)
		}

		if status != "PENDING" {
			t.Errorf(
				"expected PENDING status, got %q",
				status,
			)
		}

		if payloadUserID != user.ID {
			t.Errorf(
				"expected payload user ID %d, got %d",
				user.ID,
				payloadUserID,
			)
		}

		if payloadEmail != user.Email {
			t.Errorf(
				"expected payload email %q, got %q",
				user.Email,
				payloadEmail,
			)
		}
	})

	t.Run("rejects duplicate email case insensitively", func(t *testing.T) {
		resetDatabase(t, pool)

		ctx := context.Background()

		_, err := service.Register(
			ctx,
			auth.RegisterInput{
				Email:    "user@example.com",
				Password: "very-secure-password",
			},
		)
		if err != nil {
			t.Fatalf("register first user: %v", err)
		}

		_, err = service.Register(
			ctx,
			auth.RegisterInput{
				Email:    "USER@example.com",
				Password: "another-secure-password",
			},
		)

		if !errors.Is(err, auth.ErrEmailAlreadyExists) {
			t.Fatalf(
				"expected ErrEmailAlreadyExists, got %v",
				err,
			)
		}

		if countRows(t, pool, "users") != 1 {
			t.Fatal("expected exactly one user")
		}

		if countRows(t, pool, "email_outbox") != 1 {
			t.Fatal("expected exactly one outbox message")
		}
	})

	t.Run("rolls back user when outbox insert fails", func(t *testing.T) {
		resetDatabase(t, pool)

		ctx := context.Background()

		_, err := pool.Exec(
			ctx,
			`
				ALTER TABLE email_outbox
				RENAME TO email_outbox_unavailable
			`,
		)
		if err != nil {
			t.Fatalf("temporarily rename email_outbox: %v", err)
		}

		t.Cleanup(func() {
			_, restoreErr := pool.Exec(
				context.Background(),
				`
					ALTER TABLE email_outbox_unavailable
					RENAME TO email_outbox
				`,
			)
			if restoreErr != nil {
				t.Errorf(
					"restore email_outbox table: %v",
					restoreErr,
				)
			}
		})

		_, err = repository.CreateUserWithWelcomeEmail(
			ctx,
			auth.CreateUserParams{
				Email:        "rollback@example.com",
				PasswordHash: "hashed-password",
			},
		)
		if err == nil {
			t.Fatal("expected outbox insert to fail")
		}

		if countRows(t, pool, "users") != 0 {
			t.Fatal(
				"user must be rolled back when outbox insert fails",
			)
		}
	})
}

func TestOutboxRepositoryIntegration(t *testing.T) {
	pool := openTestPool(t)

	repository := notification.NewPostgresRepository(pool)

	t.Run("concurrent workers do not claim the same message", func(t *testing.T) {
		resetDatabase(t, pool)

		outboxID := insertPendingOutbox(
			t,
			pool,
			"concurrent@example.com",
		)

		type claimResult struct {
			message  notification.OutboxMessage
			found    bool
			err      error
			workerID string
		}

		start := make(chan struct{})
		results := make(chan claimResult, 2)

		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		for _, workerID := range []string{
			"worker-a",
			"worker-b",
		} {
			go func(workerID string) {
				<-start

				message, found, err :=
					repository.ClaimPending(
						ctx,
						workerID,
					)

				results <- claimResult{
					message:  message,
					found:    found,
					err:      err,
					workerID: workerID,
				}
			}(workerID)
		}

		close(start)

		foundCount := 0

		for range 2 {
			select {
			case result := <-results:
				if result.err != nil {
					t.Fatalf(
						"worker %s claim failed: %v",
						result.workerID,
						result.err,
					)
				}

				if result.found {
					foundCount++

					if result.message.ID != outboxID {
						t.Errorf(
							"expected outbox ID %d, got %d",
							outboxID,
							result.message.ID,
						)
					}
				}

			case <-ctx.Done():
				t.Fatal("timed out waiting for workers")
			}
		}

		if foundCount != 1 {
			t.Fatalf(
				"expected exactly one successful claim, got %d",
				foundCount,
			)
		}

		var (
			status   string
			lockedBy string
		)

		err := pool.QueryRow(
			context.Background(),
			`
				SELECT status, locked_by
				FROM email_outbox
				WHERE id = $1
			`,
			outboxID,
		).Scan(
			&status,
			&lockedBy,
		)
		if err != nil {
			t.Fatalf("query claimed outbox: %v", err)
		}

		if status != "PROCESSING" {
			t.Errorf(
				"expected PROCESSING status, got %q",
				status,
			)
		}

		if lockedBy != "worker-a" &&
			lockedBy != "worker-b" {
			t.Errorf("unexpected worker lock %q", lockedBy)
		}
	})

	t.Run("retry returns message to pending", func(t *testing.T) {
		resetDatabase(t, pool)

		outboxID := insertPendingOutbox(
			t,
			pool,
			"retry@example.com",
		)

		_, found, err := repository.ClaimPending(
			context.Background(),
			"worker-a",
		)
		if err != nil {
			t.Fatalf("claim outbox: %v", err)
		}

		if !found {
			t.Fatal("expected pending outbox message")
		}

		retryAt := time.Now().
			UTC().
			Add(time.Minute).
			Truncate(time.Microsecond)

		err = repository.MarkRetry(
			context.Background(),
			outboxID,
			"worker-a",
			retryAt,
			"SMTP temporarily unavailable",
		)
		if err != nil {
			t.Fatalf("mark outbox retry: %v", err)
		}

		var (
			status           string
			attemptCount     int
			nextAttemptAt    time.Time
			lockedByIsNull   bool
			processingIsNull bool
		)

		err = pool.QueryRow(
			context.Background(),
			`
				SELECT
					status,
					attempt_count,
					next_attempt_at,
					locked_by IS NULL,
					processing_started_at IS NULL
				FROM email_outbox
				WHERE id = $1
			`,
			outboxID,
		).Scan(
			&status,
			&attemptCount,
			&nextAttemptAt,
			&lockedByIsNull,
			&processingIsNull,
		)
		if err != nil {
			t.Fatalf("query retried outbox: %v", err)
		}

		if status != "PENDING" {
			t.Errorf(
				"expected PENDING status, got %q",
				status,
			)
		}

		if attemptCount != 1 {
			t.Errorf(
				"expected attempt count 1, got %d",
				attemptCount,
			)
		}

		if !nextAttemptAt.Equal(retryAt) {
			t.Errorf(
				"expected retry time %s, got %s",
				retryAt,
				nextAttemptAt,
			)
		}

		if !lockedByIsNull || !processingIsNull {
			t.Fatal("retry must release processing lock")
		}
	})

	t.Run("recovers only expired processing messages", func(t *testing.T) {
		resetDatabase(t, pool)

		now := time.Now().
			UTC().
			Truncate(time.Microsecond)

		oldID := insertProcessingOutbox(
			t,
			pool,
			"old@example.com",
			"crashed-worker",
			now.Add(-10*time.Minute),
		)

		freshID := insertProcessingOutbox(
			t,
			pool,
			"fresh@example.com",
			"active-worker",
			now.Add(-30*time.Second),
		)

		recovered, err := repository.RecoverStuck(
			context.Background(),
			now.Add(-2*time.Minute),
		)
		if err != nil {
			t.Fatalf("recover stuck outbox: %v", err)
		}

		if recovered != 1 {
			t.Fatalf(
				"expected one recovered message, got %d",
				recovered,
			)
		}

		if status := outboxStatus(t, pool, oldID); status != "PENDING" {
			t.Errorf(
				"expected old message PENDING, got %q",
				status,
			)
		}

		if status := outboxStatus(t, pool, freshID); status != "PROCESSING" {
			t.Errorf(
				"expected fresh message PROCESSING, got %q",
				status,
			)
		}
	})
}

func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create PostgreSQL test pool: %v", err)
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL test database: %v", err)
	}

	return pool
}

func resetDatabase(
	t *testing.T,
	pool *pgxpool.Pool,
) {
	t.Helper()

	_, err := pool.Exec(
		context.Background(),
		`
			TRUNCATE TABLE
				email_outbox,
				contacts,
				users
			RESTART IDENTITY
			CASCADE
		`,
	)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}

func countRows(
	t *testing.T,
	pool *pgxpool.Pool,
	tableName string,
) int {
	t.Helper()

	allowedTables := map[string]bool{
		"users":        true,
		"email_outbox": true,
	}

	if !allowedTables[tableName] {
		t.Fatalf("table %q is not allowed", tableName)
	}

	query := "SELECT COUNT(*) FROM " + tableName

	var count int

	if err := pool.QueryRow(
		context.Background(),
		query,
	).Scan(&count); err != nil {
		t.Fatalf("count rows in %s: %v", tableName, err)
	}

	return count
}

func insertPendingOutbox(
	t *testing.T,
	pool *pgxpool.Pool,
	recipient string,
) int64 {
	t.Helper()

	payload, err := json.Marshal(
		notification.UserRegisteredPayload{
			UserID: 1,
			Email:  recipient,
		},
	)
	if err != nil {
		t.Fatalf("marshal test payload: %v", err)
	}

	var id int64

	err = pool.QueryRow(
		context.Background(),
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
				$2::JSONB
			)
			RETURNING id
		`,
		recipient,
		string(payload),
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert pending outbox: %v", err)
	}

	return id
}

func insertProcessingOutbox(
	t *testing.T,
	pool *pgxpool.Pool,
	recipient string,
	workerID string,
	lockedAt time.Time,
) int64 {
	t.Helper()

	payload, err := json.Marshal(
		notification.UserRegisteredPayload{
			UserID: 1,
			Email:  recipient,
		},
	)
	if err != nil {
		t.Fatalf("marshal test payload: %v", err)
	}

	var id int64

	err = pool.QueryRow(
		context.Background(),
		`
			INSERT INTO email_outbox (
				event_type,
				recipient_email,
				template_name,
				payload,
				status,
				processing_started_at,
				locked_at,
				locked_by
			)
			VALUES (
				'USER_REGISTERED',
				$1,
				'welcome',
				$2::JSONB,
				'PROCESSING',
				$3,
				$3,
				$4
			)
			RETURNING id
		`,
		recipient,
		string(payload),
		lockedAt,
		workerID,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert processing outbox: %v", err)
	}

	return id
}

func outboxStatus(
	t *testing.T,
	pool *pgxpool.Pool,
	id int64,
) string {
	t.Helper()

	var status string

	err := pool.QueryRow(
		context.Background(),
		`
			SELECT status
			FROM email_outbox
			WHERE id = $1
		`,
		id,
	).Scan(&status)
	if err != nil {
		t.Fatalf("query outbox status: %v", err)
	}

	return status
}
