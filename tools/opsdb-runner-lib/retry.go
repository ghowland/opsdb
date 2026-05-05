//# tools/opsdb-runner-lib/retry.go

go
package runnerlib

import (
	"fmt"
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
	// TODO: record start time
	// TODO: for attempt := 0; attempt < config.MaxAttempts; attempt++:
	//   call fn()
	//   if nil: return nil (success)
	//   if not IsRetryable(err): return err immediately (non-retryable)
	//   if last attempt: return err (exhausted)
	//   compute delay: config.BaseDelay * (config.Multiplier ^ attempt)
	//   apply jitter: delay += random(-jitterFraction, +jitterFraction) * delay
	//   check if time.Since(start) + delay > config.MaxTotalDuration: return err
	//   sleep for delay
	// TODO: return last error
	return fmt.Errorf("not implemented")
}

// WithIdempotencyKey wraps a function call with an idempotency key.
// The key is passed as a header in API calls to make retries safe
// for write operations.
func WithIdempotencyKey(key string, fn func(idempotencyKey string) error) error {
	// TODO: call fn(key)
	// TODO: return fn's error
	// NOTE: the actual idempotency enforcement is server-side;
	// this function just threads the key through to the API call
	return fn(key)
}

// IsRetryable classifies an error as retryable or not.
// Retryable: network errors, 503 Service Unavailable, 429 Too Many Requests.
// Not retryable: 400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found,
// 409 Conflict (stale version), validation errors.
func IsRetryable(err error) bool {
	// TODO: check if err is *NetworkError: retryable
	// TODO: check if err is *net.OpError or net.Error (timeout): retryable
	// TODO: check if err is *HTTPError:
	//   503: retryable
	//   429: retryable
	//   502: retryable
	//   all others: not retryable
	// TODO: check if err is *AuthorizationDeniedError: not retryable
	// TODO: check if err is *ValidationFailedError: not retryable
	// TODO: check if err is *StaleVersionError: not retryable
	// TODO: check if err is *NotFoundError: not retryable
	// TODO: default: not retryable
	_ = net.OpError{}
	return false
}

// computeDelay calculates the delay for a retry attempt with jitter.
func computeDelay(config RetryConfig, attempt int) time.Duration {
	// TODO: base = config.BaseDelay * (config.Multiplier ^ attempt)
	// TODO: jitter = base * config.JitterFraction * (2*rand.Float64() - 1)
	// TODO: return base + jitter, clamped to >= 0
	_ = math.Pow
	_ = rand.Float64
	return config.BaseDelay
}

// HTTPError represents an HTTP error response from the API.
type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
	Detail     map[string]interface{}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s: %s", e.StatusCode, e.Code, e.Message)
}


