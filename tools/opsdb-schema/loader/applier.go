//# tools/opsdb-schema/loader/applier.go

go
package loader

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// ApplyResult holds the outcome of a schema apply operation.
type ApplyResult struct {
	StatementsExecuted int
	EntitiesCreated    int
	FieldsAdded        int
	ConstraintsModified int
	IndexesCreated     int
	Duration           time.Duration
	DryRun             bool
}

// Apply executes generated DDL against Postgres within a transaction.
// Acquires advisory lock to prevent concurrent applies, begins transaction,
// executes statements in order, commits on success, rolls back on error.
func Apply(db *pg.DB, statements []DDLStatement, verbose bool) (*ApplyResult, error) {
	startTime := time.Now()
	result := &ApplyResult{}

	// TODO: acquire advisory lock via pg.WaitForAdvisoryLock(db, pg.SchemaApplyLockID(), 30*time.Second)
	//   if timeout: return error "another schema apply is in progress"
	// TODO: defer pg.ReleaseAdvisoryLock(db, pg.SchemaApplyLockID())

	// TODO: begin transaction via pg.WithTransaction(db, func(tx *pg.Tx) error {
	//   for each statement in statements:
	//     if verbose: print statement.SQL to stdout
	//     _, err := pg.ExecInTx(tx, statement.SQL)
	//     if err: return fmt.Errorf("failed at %s: %w", statement.Description, err)
	//     result.StatementsExecuted++
	//     count by type:
	//       if statement.Description starts with "create table": result.EntitiesCreated++
	//       if statement.Description starts with "add column": result.FieldsAdded++
	//       if statement.Phase == 2: result.ConstraintsModified++
	//       if statement.Phase == 3: result.IndexesCreated++
	//   return nil
	// })

	// TODO: if transaction error: return nil, error
	// TODO: result.Duration = time.Since(startTime)
	// TODO: return result

	_ = startTime
	return result, fmt.Errorf("not implemented")
}

// DryRun executes generated DDL within a transaction then rolls back.
// Validates that the DDL is executable without persisting any changes.
// Reports the same result as Apply would produce.
func DryRun(db *pg.DB, statements []DDLStatement) (*ApplyResult, error) {
	startTime := time.Now()
	result := &ApplyResult{DryRun: true}

	// TODO: acquire advisory lock (same as Apply)
	// TODO: defer release

	// TODO: begin transaction manually (not using WithTransaction since we always rollback)
	//   tx, err := db.Begin()
	//   defer tx.Rollback() // always rollback for dry run
	//
	//   for each statement in statements:
	//     _, err := tx.Exec(statement.SQL)
	//     if err: return nil, fmt.Errorf("dry run failed at %s: %w", statement.Description, err)
	//     result.StatementsExecuted++
	//     count by type (same as Apply)
	//
	//   tx.Rollback() // explicit rollback, DDL validated but not persisted

	// TODO: result.Duration = time.Since(startTime)
	// TODO: return result

	_ = startTime
	return result, fmt.Errorf("not implemented")
}


