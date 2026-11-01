package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// reorder rewrites sort_order for the given ids (in the order supplied) inside
// one transaction. table and scope are package-internal constants, never user
// input; scopeArgs carry any user-influenced values as bind parameters. It
// fails if any id does not exist within the scope, so a stale admin page can't
// silently reorder the wrong rows.
func reorder(ctx context.Context, db *sql.DB, table, scope string, scopeArgs []any, ids []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin reorder: %w", err)
	}
	defer tx.Rollback()

	stmt := fmt.Sprintf(`UPDATE %s SET sort_order = ?, updated_at = ? WHERE id = ? AND %s`, table, scope)
	ts := now()
	for i, id := range ids {
		args := append([]any{i + 1, ts, id}, scopeArgs...)
		res, err := tx.ExecContext(ctx, stmt, args...)
		if err != nil {
			return fmt.Errorf("storage: reorder %s: %w", table, err)
		}
		if err := requireAffected(res, table); err != nil {
			return fmt.Errorf("storage: reorder %s id %d: %w", table, id, err)
		}
	}
	return tx.Commit()
}

// nextSortOrder returns max(sort_order)+1 for rows matching scope.
func nextSortOrder(ctx context.Context, db *sql.DB, table, scope string, scopeArgs ...any) (int, error) {
	var max sql.NullInt64
	q := fmt.Sprintf(`SELECT MAX(sort_order) FROM %s WHERE %s`, table, scope)
	if err := db.QueryRowContext(ctx, q, scopeArgs...).Scan(&max); err != nil {
		return 0, fmt.Errorf("storage: next sort order (%s): %w", table, err)
	}
	return int(max.Int64) + 1, nil
}
