package notification

import (
	"testing"
	"time"
)

func TestRetryPolicy(t *testing.T) {
	policy, err := NewRetryPolicy([]time.Duration{
		10 * time.Second,
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("create retry policy: %v", err)
	}

	if policy.MaxAttempts() != 5 {
		t.Fatalf("expected 5 attempts, got %d", policy.MaxAttempts())
	}

	tests := []struct {
		failedAttempts int
		expectedDelay  time.Duration
	}{
		{1, 10 * time.Second},
		{2, 30 * time.Second},
		{3, 2 * time.Minute},
		{4, 10 * time.Minute},
	}

	for _, test := range tests {
		delay, err := policy.Delay(test.failedAttempts)
		if err != nil {
			t.Fatalf("get retry delay for attempt %d: %v", test.failedAttempts, err)
		}

		if delay != test.expectedDelay {
			t.Errorf("attempt %d: expected %s, got %s", test.failedAttempts, test.expectedDelay, delay)
		}
	}

	if policy.ShouldRetry(5) {
		t.Fatal("fifth failure must not be retried")
	}
}
