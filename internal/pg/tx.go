
// === internal/pg/tx.go ===
package pg

import "database/sql"

// Tx wraps a sql.Tx with OpsDB helpers.
type Tx struct {
	tx *sql.Tx
}

// WithTransaction begins a transaction, calls fn, commits on success,
// rolls back on error or panic.
func WithTransaction(db *DB, fn func(tx *Tx) error) error {
	// TODO: db.pool.Begin()
	// TODO: defer rollback on panic
	// TODO: call fn(tx)
	// TODO: if error, rollback and return error
	// TODO: commit and return nil
	return nil
}

// ExecInTx executes a statement within a transaction.
func ExecInTx(tx *Tx, query string, args ...interface{}) (sql.Result, error) {
	// TODO: tx.tx.Exec(query, args...)
	return nil, nil
}

// QueryInTx executes a query within a transaction and returns rows.
func QueryInTx(tx *Tx, query string, args ...interface{}) (*sql.Rows, error) {
	// TODO: tx.tx.Query(query, args...)
	return nil, nil
}

// QueryRowInTx executes a query expected to return one row.
func QueryRowInTx(tx *Tx, query string, args ...interface{}) *sql.Row {
	// TODO: tx.tx.QueryRow(query, args...)
	return nil
}


