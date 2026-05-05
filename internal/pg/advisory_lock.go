package pg

import (
	"context"
	"fmt"
	"time"
)

// SchemaApplyLockID returns the fixed advisory lock ID used for schema apply.
// All schema apply operations use this same ID to prevent concurrent applies.
// Chosen to avoid collision with application-level advisory locks.
func SchemaApplyLockID() int64 {
	return 7283946501
}

// AcquireAdvisoryLock attempts to acquire a session-level advisory lock.
// Returns true if acquired, false if already held by another session.
// Non-blocking — returns immediately regardless of lock state.
func AcquireAdvisoryLock(db *DB, lockID int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var acquired bool
	err := db.Pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("advisory lock acquire: %w", err)
	}
	return acquired, nil
}

// WaitForAdvisoryLock blocks until the lock is acquired or timeout expires.
// Polls with a short sleep interval. Returns nil on acquisition, error on timeout
// or database failure.
func WaitForAdvisoryLock(db *DB, lockID int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	pollInterval := 250 * time.Millisecond

	for {
		acquired, err := AcquireAdvisoryLock(db, lockID)
		if err != nil {
			return fmt.Errorf("advisory lock wait: %w", err)
		}
		if acquired {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("advisory lock %d not acquired within %s: another operation is in progress", lockID, timeout)
		}

		remaining := time.Until(deadline)
		if remaining < pollInterval {
			time.Sleep(remaining)
		} else {
			time.Sleep(pollInterval)
		}
	}
}

// ReleaseAdvisoryLock releases a session-level advisory lock.
// Returns error if the lock was not held by this session (should not happen
// in normal operation — indicates a programming error).
func ReleaseAdvisoryLock(db *DB, lockID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var released bool
	err := db.Pool.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", lockID).Scan(&released)
	if err != nil {
		return fmt.Errorf("advisory lock release: %w", err)
	}
	if !released {
		return fmt.Errorf("advisory lock %d was not held by this session", lockID)
	}
	return nil
}
