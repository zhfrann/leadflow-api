package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	mailx "github.com/zhfrann/leadflow-api/internal/platform/mail"
)

const userRegisteredEvent = "USER_REGISTERED"

type Worker struct {
	repository   Repository
	sender       mailx.Sender
	templates    *Templates
	logger       *slog.Logger
	workerID     string
	pollInterval time.Duration
	retryDelay   time.Duration
}

func NewWorker(
	repository Repository,
	sender mailx.Sender,
	templates *Templates,
	logger *slog.Logger,
	workerID string,
	pollInterval time.Duration,
	retryDelay time.Duration,
) *Worker {
	return &Worker{
		repository:   repository,
		sender:       sender,
		templates:    templates,
		logger:       logger,
		workerID:     workerID,
		pollInterval: pollInterval,
		retryDelay:   retryDelay,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info(
		"email worker started",
		"worker_id", w.workerID,
	)

	for {
		if ctx.Err() != nil {
			w.logger.Info("email worker stopping")
			return nil
		}

		processed, err := w.processOne(ctx)
		if err != nil {
			w.logger.Error(
				"process email outbox failed",
				"worker_id", w.workerID,
				"error", err,
			)
		}

		if processed {
			continue
		}

		timer := time.NewTimer(w.pollInterval)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}

			w.logger.Info("email worker stopping")
			return nil

		case <-timer.C:
		}
	}
}

func (w *Worker) processOne(
	ctx context.Context,
) (bool, error) {
	outbox, found, err := w.repository.ClaimPending(
		ctx,
		w.workerID,
	)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	if outbox.EventType != userRegisteredEvent {
		message := fmt.Sprintf(
			"unsupported event type %q",
			outbox.EventType,
		)

		if err := w.repository.MarkFailed(
			ctx,
			outbox.ID,
			w.workerID,
			message,
		); err != nil {
			return true, err
		}

		return true, nil
	}

	var payload UserRegisteredPayload

	if err := json.Unmarshal(
		outbox.Payload,
		&payload,
	); err != nil {
		message := fmt.Sprintf(
			"decode outbox payload: %v",
			err,
		)

		if markErr := w.repository.MarkFailed(
			ctx,
			outbox.ID,
			w.workerID,
			message,
		); markErr != nil {
			return true, markErr
		}

		return true, nil
	}

	emailMessage, err := w.templates.WelcomeMessage(
		outbox.RecipientEmail,
		payload,
	)
	if err != nil {
		if markErr := w.repository.MarkFailed(
			ctx,
			outbox.ID,
			w.workerID,
			err.Error(),
		); markErr != nil {
			return true, markErr
		}

		return true, nil
	}

	startedAt := time.Now()

	if err := w.sender.Send(ctx, emailMessage); err != nil {
		nextAttemptAt := time.Now().
			UTC().
			Add(w.retryDelay)

		if markErr := w.repository.MarkRetry(
			ctx,
			outbox.ID,
			w.workerID,
			nextAttemptAt,
			err.Error(),
		); markErr != nil {
			return true, fmt.Errorf(
				"send email failed and retry scheduling failed: %w",
				markErr,
			)
		}

		w.logger.Warn(
			"email delivery scheduled for retry",
			"outbox_id", outbox.ID,
			"event_type", outbox.EventType,
			"attempt_count", outbox.AttemptCount+1,
			"next_attempt_at", nextAttemptAt,
		)

		return true, nil
	}

	if err := w.repository.MarkSent(
		ctx,
		outbox.ID,
		w.workerID,
	); err != nil {
		return true, err
	}

	w.logger.Info(
		"email delivered",
		"outbox_id", outbox.ID,
		"event_type", outbox.EventType,
		"duration", time.Since(startedAt),
	)

	return true, nil
}
