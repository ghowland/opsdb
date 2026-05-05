// === internal/pg/advisory_lock.go ===
package pg

import "time"

// SchemaApplyLockID returns the fixed advisory lock ID used for schema apply.
// All schema apply operations use this same ID to prevent concurrent applies.
func SchemaApplyLockID() int64 {
	// Fixed lock ID for schema operations. Chosen to avoid collision with
	// application-level advisory locks.
	return 7283946501 // arbitrary fixed value
}

// AcquireAdvisoryLock attempts to acquire a session-level advisory lock.
// Returns true if acquired, false if already held by another session.
// Non-blocking.
func AcquireAdvisoryLock(db *DB, lockID int64) (bool, error) {
	// TODO: SELECT pg_try_advisory_lock($1)
	// TODO: return boolean result
	return false, nil
}

// WaitForAdvisoryLock blocks until the lock is acquired or timeout expires.
// Used by the apply command to wait for a concurrent apply to finish.
func WaitForAdvisoryLock(db *DB, lockID int64, timeout time.Duration) error {
	// TODO: loop with short sleep:
	//       try AcquireAdvisoryLock
	//       if acquired, return nil
	//       if timeout exceeded, return error
	return nil
}

// ReleaseAdvisoryLock releases a session-level advisory lock.
func ReleaseAdvisoryLock(db *DB, lockID int64) error {
	// TODO: SELECT pg_advisory_unlock($1)
	// TODO: check result is true (we held the lock)
	return nil
}

