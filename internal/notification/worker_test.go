package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	mailx "github.com/zhfrann/leadflow-api/internal/platform/mail"
)

type fakeRepository struct {
	message OutboxMessage
	found   bool

	sentID        int64
	retryID       int64
	retryAt       time.Time
	failedID      int64
	recoveredFrom time.Time
}

func (f *fakeRepository) ClaimPending(_ context.Context, _ string) (OutboxMessage, bool, error) {
	return f.message, f.found, nil
}

func (f *fakeRepository) MarkSent(_ context.Context, id int64, _ string) error {
	f.sentID = id
	return nil
}

func (f *fakeRepository) MarkRetry(_ context.Context, id int64, _ string, nextAttemptAt time.Time, _ string) error {
	f.retryID = id
	f.retryAt = nextAttemptAt

	return nil
}

func (f *fakeRepository) MarkFailed(_ context.Context, id int64, _ string, _ string) error {
	f.failedID = id
	return nil
}

func (f *fakeRepository) RecoverStuck(_ context.Context, lockedBefore time.Time) (int64, error) {
	f.recoveredFrom = lockedBefore
	return 1, nil
}

type fakeSender struct {
	err     error
	message mailx.Message
}

func (f *fakeSender) Send(_ context.Context, message mailx.Message) error {
	f.message = message
	return f.err
}

func newTestWorker(t *testing.T, repository Repository, sender mailx.Sender, policy RetryPolicy) *Worker {
	t.Helper()

	templates, err := NewTemplates()
	if err != nil {
		t.Fatalf("create templates: %v", err)
	}

	logger := slog.New(
		slog.NewTextHandler(io.Discard, nil),
	)

	return NewWorker(
		repository,
		sender,
		templates,
		logger,
		"test-worker",
		time.Second,
		policy,
		2*time.Minute,
		30*time.Second,
	)
}

func newUserRegisteredMessage(attemptCount int) OutboxMessage {
	payload, _ := json.Marshal(
		UserRegisteredPayload{
			UserID: 1,
			Email:  "user@example.com",
		},
	)

	return OutboxMessage{
		ID:             10,
		EventType:      userRegisteredEvent,
		RecipientEmail: "user@example.com",
		Payload:        payload,
		AttemptCount:   attemptCount,
	}
}

func TestWorkerSchedulesRetryAfterSendFailure(t *testing.T) {
	policy, err := NewRetryPolicy([]time.Duration{
		10 * time.Second,
		30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	repository := &fakeRepository{
		message: newUserRegisteredMessage(0),
		found:   true,
	}

	sender := &fakeSender{
		err: errors.New("SMTP unavailable"),
	}

	worker := newTestWorker(
		t,
		repository,
		sender,
		policy,
	)

	now := time.Date(
		2026,
		time.July,
		15,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	worker.now = func() time.Time {
		return now
	}

	processed, err := worker.processOne(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("process email: %v", err)
	}

	if !processed {
		t.Fatal("expected message to be processed")
	}

	if repository.retryID != 10 {
		t.Fatalf(
			"expected outbox 10 to retry, got %d",
			repository.retryID,
		)
	}

	expectedRetryAt := now.Add(10 * time.Second)

	if !repository.retryAt.Equal(expectedRetryAt) {
		t.Errorf(
			"expected retry at %s, got %s",
			expectedRetryAt,
			repository.retryAt,
		)
	}

	if repository.failedID != 0 {
		t.Fatal("message must not be marked failed")
	}
}

func TestWorkerMarksFailedAfterMaximumAttempts(t *testing.T) {
	policy, err := NewRetryPolicy([]time.Duration{
		10 * time.Second,
		30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	repository := &fakeRepository{
		// Total maximum attempts = 3.
		// Dua kegagalan telah terjadi sebelumnya.
		message: newUserRegisteredMessage(2),
		found:   true,
	}

	sender := &fakeSender{
		err: errors.New("SMTP unavailable"),
	}

	worker := newTestWorker(
		t,
		repository,
		sender,
		policy,
	)

	processed, err := worker.processOne(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("process email: %v", err)
	}

	if !processed {
		t.Fatal("expected message to be processed")
	}

	if repository.failedID != 10 {
		t.Fatalf(
			"expected outbox 10 to fail, got %d",
			repository.failedID,
		)
	}

	if repository.retryID != 0 {
		t.Fatal("message must not be retried")
	}
}
