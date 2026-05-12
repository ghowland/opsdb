package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tx wraps a pgx transaction with OpsDB helpers.
type Tx struct {
	tx  pgx.Tx
	ctx context.Context
}

// DefaultTxTimeout is the maximum duration a transaction can remain open.
const DefaultTxTimeout = 60 * time.Second

// WithTransaction begins a transaction, calls fn, commits on success,
// rolls back on error or panic. The transaction uses serializable isolation
// for schema DDL operations and read-committed for normal operations.
func WithTransaction(db *DB, fn func(tx *Tx) error) error {
	return WithTransactionContext(context.Background(), db, fn)
}

// WithTransactionContext begins a transaction with an explicit context.
func WithTransactionContext(ctx context.Context, db *DB, fn func(tx *Tx) error) error {
	txCtx, cancel := context.WithTimeout(ctx, DefaultTxTimeout)
	defer cancel()

	pgxTx, err := db.Pool.Begin(txCtx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	tx := &Tx{tx: pgxTx, ctx: txCtx}

	defer func() {
		if r := recover(); r != nil {
			_ = pgxTx.Rollback(txCtx)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		rbErr := pgxTx.Rollback(txCtx)
		if rbErr != nil {
			return fmt.Errorf("transaction failed: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := pgxTx.Commit(txCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// WithSerializableTransaction begins a transaction with serializable isolation.
// Used for schema DDL operations where concurrent modification must be prevented.
func WithSerializableTransaction(db *DB, fn func(tx *Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()

	pgxTx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return fmt.Errorf("begin serializable transaction: %w", err)
	}

	tx := &Tx{tx: pgxTx, ctx: ctx}

	defer func() {
		if r := recover(); r != nil {
			_ = pgxTx.Rollback(ctx)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		rbErr := pgxTx.Rollback(ctx)
		if rbErr != nil {
			return fmt.Errorf("transaction failed: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit serializable transaction: %w", err)
	}
	return nil
}

// RollbackOnly begins a transaction, calls fn, then always rolls back.
// Used for dry-run validation of DDL — executes everything to test
// correctness then discards all changes.
func RollbackOnly(db *DB, fn func(tx *Tx) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()

	pgxTx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin dry-run transaction: %w", err)
	}

	tx := &Tx{tx: pgxTx, ctx: ctx}

	defer func() {
		_ = pgxTx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	// Always rollback — this is the dry-run contract.
	_ = pgxTx.Rollback(ctx)
	return nil
}

// ExecInTx executes a statement within a transaction.
// Returns the command tag (rows affected, etc.) or error.
func ExecInTx(tx *Tx, query string, args ...interface{}) (pgconn.CommandTag, error) {
	tag, err := tx.tx.Exec(tx.ctx, query, args...)
	if err != nil {
		return tag, fmt.Errorf("exec in tx: %w", err)
	}
	return tag, nil
}

// QueryInTx executes a query within a transaction and returns rows.
// Caller is responsible for closing the returned Rows.
func QueryInTx(tx *Tx, query string, args ...interface{}) (pgx.Rows, error) {
	rows, err := tx.tx.Query(tx.ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query in tx: %w", err)
	}
	return rows, nil
}

// QueryRowInTx executes a query expected to return at most one row.
// Use .Scan() on the returned Row to read values.
func QueryRowInTx(tx *Tx, query string, args ...interface{}) pgx.Row {
	return tx.tx.QueryRow(tx.ctx, query, args...)
}

// QueryRowInDB executes a query that returns at most one row, on the database directly.
func QueryRowInDB(db *DB, query string, args ...interface{}) *sql.Row {
	return db.pool.QueryRow(query, args...)
}

// Context returns the transaction's context. Used by callers that need
// to pass context to sub-operations within the transaction scope.
func (tx *Tx) Context() context.Context {
	return tx.ctx
}

// Underlying returns the raw pgx.Tx for cases where direct access is needed.
// Prefer ExecInTx/QueryInTx/QueryRowInTx for normal operations.
func (tx *Tx) Underlying() pgx.Tx {
	return tx.tx
}

// QuoteIdentifier wraps a SQL identifier in double quotes with proper
// escaping. Prevents SQL injection when table or column names are
// constructed dynamically from schema metadata.
func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
