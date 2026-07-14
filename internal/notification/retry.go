package notification

import (
	"fmt"
	"time"
)

type RetryPolicy struct {
	delays []time.Duration
}

func (p RetryPolicy) MaxAttempts() int {
	return len(p.delays) + 1
}

func (p RetryPolicy) ShouldRetry(failedAttempts int) bool {
	return failedAttempts < p.MaxAttempts()
}

func (p RetryPolicy) Delay(failedAttempts int) (time.Duration, error) {
	if failedAttempts < 1 ||
		failedAttempts >= p.MaxAttempts() {
		return 0, fmt.Errorf("failed attempts %d has no retry delay", failedAttempts)
	}

	return p.delays[failedAttempts-1], nil
}

func NewRetryPolicy(delays []time.Duration) (RetryPolicy, error) {
	if len(delays) == 0 {
		return RetryPolicy{}, fmt.Errorf("retry delays must not be empty")
	}

	copiedDelays := make([]time.Duration, len(delays))

	for index, delay := range delays {
		if delay <= 0 {
			return RetryPolicy{}, fmt.Errorf("retry delay %d must be greater than zero", index+1)
		}

		copiedDelays[index] = delay
	}

	return RetryPolicy{
		delays: copiedDelays,
	}, nil
}
