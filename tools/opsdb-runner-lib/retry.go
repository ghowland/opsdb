package runnerlib

import (
	"errors"
	"math"
	"math/rand"
	"net"
	"time"
)

// RetryConfig controls retry behavior for operations that cross failure boundaries.
type RetryConfig struct {
	MaxAttempts      int           // maximum number of attempts (including first)
	BaseDelay        time.Duration // delay before first retry
	Multiplier       float64       // exponential backoff multiplier
	JitterFraction   float64       // fraction of delay to randomize (0.0-1.0)
	MaxTotalDuration time.Duration // hard ceiling on total retry time
}

// DefaultRetryConfig returns the default retry configuration:
// 3 attempts, 1s base delay, 2x multiplier, 25% jitter, 30s max total.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		BaseDelay:        1 * time.Second,
		Multiplier:       2.0,
		JitterFraction:   0.25,
		MaxTotalDuration: 30 * time.Second,
	}
}

// WithRetry calls fn, retrying on retryable errors with exponential backoff
// and jitter. Stops after max attempts or max total duration.
// Returns the error from the last attempt, or nil on success.
func WithRetry(config RetryConfig, fn func() error) error {
	start := time.Now()
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		lastErr = fn()

		// Success.
		if lastErr == nil {
			return nil
		}

		// Non-retryable errors stop immediately.
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// Last attempt — don't sleep, just return the error.
		if attempt == config.MaxAttempts-1 {
			return lastErr
		}

		// Compute delay with exponential backoff and jitter.
		delay := computeDelay(config, attempt)

		// Check if sleeping would exceed the total duration budget.
		elapsed := time.Since(start)
		if elapsed+delay > config.MaxTotalDuration {
			return lastErr
		}

		time.Sleep(delay)

		// Check total duration after sleeping.
		if time.Since(start) > config.MaxTotalDuration {
			return lastErr
		}
	}

	return lastErr
}

// WithIdempotencyKey wraps a function call with an idempotency key.
// The key is passed as a header in API calls to make retries safe
// for write operations. The actual idempotency enforcement is server-side;
// this function threads the key through to the API call.
func WithIdempotencyKey(key string, fn func(idempotencyKey string) error) error {
	return fn(key)
}

// IsRetryable classifies an error as retryable or not.
// Retryable: network errors, timeouts, 503, 429, 502.
// Not retryable: 400, 401, 403, 404, 409, validation errors, stale version.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// NetworkError from our API client — always retryable.
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return true
	}

	// Go standard library net errors — timeouts and temporary errors.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	// net.Error interface (covers timeouts from any net type).
	var goNetErr net.Error
	if errors.As(err, &goNetErr) {
		if goNetErr.Timeout() {
			return true
		}
	}

	// HTTPError — retryable only for specific status codes.
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 502: // Bad Gateway
			return true
		case 503: // Service Unavailable
			return true
		case 429: // Too Many Requests
			return true
		default:
			return false
		}
	}

	// All typed application errors are not retryable.
	var authErr *AuthorizationDeniedError
	if errors.As(err, &authErr) {
		return false
	}
	var valErr *ValidationFailedError
	if errors.As(err, &valErr) {
		return false
	}
	var staleErr *StaleVersionError
	if errors.As(err, &staleErr) {
		return false
	}
	var notFoundErr *NotFoundError
	if errors.As(err, &notFoundErr) {
		return false
	}
	var reportKeyErr *UndeclaredReportKeyError
	if errors.As(err, &reportKeyErr) {
		return false
	}

	// nonRetryableError wrapper from doRequestWithRetry.
	var nre *nonRetryableError
	if errors.As(err, &nre) {
		return false
	}

	// Unknown errors default to not retryable.
	return false
}

// computeDelay calculates the delay for a retry attempt with jitter.
// delay = baseDelay * (multiplier ^ attempt) + jitter
// Jitter is a random value in [-jitterFraction, +jitterFraction] * base delay.
// Result is clamped to a minimum of 0.
func computeDelay(config RetryConfig, attempt int) time.Duration {
	// Exponential backoff.
	multiplied := float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt))

	// Apply jitter: random value in range [-jitterFraction, +jitterFraction] of the computed delay.
	jitterRange := multiplied * config.JitterFraction
	jitter := jitterRange * (2*rand.Float64() - 1)

	delayNanos := multiplied + jitter

	// Clamp to non-negative.
	if delayNanos < 0 {
		delayNanos = 0
	}

	// Cap at a reasonable maximum to prevent overflow on high attempt counts.
	maxDelay := float64(config.MaxTotalDuration)
	if delayNanos > maxDelay {
		delayNanos = maxDelay
	}

	return time.Duration(int64(delayNanos))
}
