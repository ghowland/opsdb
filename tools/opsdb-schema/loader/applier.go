package loader

import (
	"fmt"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// ApplyResult holds the outcome of a schema apply operation.
type ApplyResult struct {
	StatementsExecuted  int
	EntitiesCreated     int
	FieldsAdded         int
	ConstraintsModified int
	IndexesCreated      int
	Duration            time.Duration
	DryRun              bool
}

// Apply executes generated DDL against Postgres within a transaction.
// Acquires advisory lock to prevent concurrent applies, begins transaction,
// executes statements in order, commits on success, rolls back on error.
func Apply(db *pg.DB, statements []DDLStatement, verbose bool) (*ApplyResult, error) {
	startTime := time.Now()
	result := &ApplyResult{}

	// Acquire advisory lock to prevent concurrent schema applies.
	err := pg.WaitForAdvisoryLock(db, pg.SchemaApplyLockID(), 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("could not acquire schema apply lock: %w", err)
	}
	defer func() {
		_ = pg.ReleaseAdvisoryLock(db, pg.SchemaApplyLockID())
	}()

	err = pg.WithSerializableTransaction(db, func(tx *pg.Tx) error {
		for i, stmt := range statements {
			if verbose {
				fmt.Printf("  [%d/%d] %s: %s\n", i+1, len(statements), stmt.Entity, stmt.Description)
				fmt.Printf("    %s\n", stmt.SQL)
			}

			_, execErr := pg.ExecInTx(tx, stmt.SQL)
			if execErr != nil {
				return fmt.Errorf("statement %d/%d failed (%s: %s): %w",
					i+1, len(statements), stmt.Entity, stmt.Description, execErr)
			}

			result.StatementsExecuted++
			classifyStatement(result, stmt)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("schema apply failed: %w", err)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// DryRun executes generated DDL within a transaction then rolls back.
// Validates that the DDL is executable without persisting any changes.
// Reports the same result as Apply would produce.
func DryRun(db *pg.DB, statements []DDLStatement) (*ApplyResult, error) {
	startTime := time.Now()
	result := &ApplyResult{DryRun: true}

	// Acquire advisory lock even for dry run to prevent interference
	// with a concurrent real apply.
	err := pg.WaitForAdvisoryLock(db, pg.SchemaApplyLockID(), 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("could not acquire schema apply lock for dry run: %w", err)
	}
	defer func() {
		_ = pg.ReleaseAdvisoryLock(db, pg.SchemaApplyLockID())
	}()

	err = pg.RollbackOnly(db, func(tx *pg.Tx) error {
		for i, stmt := range statements {
			_, execErr := pg.ExecInTx(tx, stmt.SQL)
			if execErr != nil {
				return fmt.Errorf("dry run statement %d/%d failed (%s: %s): %w",
					i+1, len(statements), stmt.Entity, stmt.Description, execErr)
			}

			result.StatementsExecuted++
			classifyStatement(result, stmt)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("dry run failed: %w", err)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// classifyStatement increments the appropriate counter on ApplyResult
// based on the statement's phase and description.
func classifyStatement(result *ApplyResult, stmt DDLStatement) {
	switch stmt.Phase {
	case 1:
		desc := strings.ToLower(stmt.Description)
		if strings.HasPrefix(desc, "create table") {
			result.EntitiesCreated++
		} else if strings.HasPrefix(desc, "add column") {
			result.FieldsAdded++
		}
	case 2:
		result.ConstraintsModified++
	case 3:
		result.IndexesCreated++
	case 4:
		// REVOKE for append-only counted as constraints for reporting.
		result.ConstraintsModified++
	}
}
