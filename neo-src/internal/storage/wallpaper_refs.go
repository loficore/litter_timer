// Package storage — wallpaper reference-counting and unbind.
//
// CountWallpaperRefs and UnbindWallpaper operate across the three tables
// that carry a `wallpaper` column: habits, habit_sets, and settings.
// All queries use exact-match `wallpaper = ?` so that only gallery-managed
// refs (prefixed `local:`) are affected; legacy Zig-era values like
// `/wallpapers/book.jpg` are left untouched.
package storage

import (
	"fmt"
)

// CountWallpaperRefs counts how many rows across habits, habit_sets, and
// settings reference the given localRef.  Only exact-match `wallpaper = ?`
// is used — legacy values like `/wallpapers/book.jpg` (which lack the
// `local:` prefix) are not counted because the caller always passes a
// `local:`-prefixed string.
func (m *SqliteManager) CountWallpaperRefs(localRef string) (int64, error) {
	if m.db == nil {
		return 0, ErrDatabaseNotConnected
	}

	const query = `
		SELECT
			(SELECT COUNT(*) FROM habits WHERE wallpaper = ?) +
			(SELECT COUNT(*) FROM habit_sets WHERE wallpaper = ?) +
			(SELECT COUNT(*) FROM settings WHERE wallpaper = ?) AS refs;
	`

	var refs int64
	err := m.db.QueryRow(query, localRef, localRef, localRef).Scan(&refs)
	if err != nil {
		return 0, fmt.Errorf("CountWallpaperRefs: %w", err)
	}
	return refs, nil
}

// UnbindWallpaper clears the wallpaper column in all three tables (habits,
// habit_sets, settings) for every row matching localRef.  The operation is
// transactional: any error triggers a rollback.  Returns the total number of
// rows affected (sum of RowsAffected from all three UPDATEs).  Repeated
// calls are idempotent (return 0).
func (m *SqliteManager) UnbindWallpaper(localRef string) (int64, error) {
	if m.db == nil {
		return 0, ErrDatabaseNotConnected
	}

	tx, err := m.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op if already committed

	var total int64

	// UPDATE habits
	res, err := tx.Exec(`UPDATE habits SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: habits: %w", err)
	}
	n, _ := res.RowsAffected()
	total += n

	// UPDATE habit_sets
	res, err = tx.Exec(`UPDATE habit_sets SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: habit_sets: %w", err)
	}
	n, _ = res.RowsAffected()
	total += n

	// UPDATE settings
	res, err = tx.Exec(`UPDATE settings SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: settings: %w", err)
	}
	n, _ = res.RowsAffected()
	total += n

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: commit: %w", err)
	}

	return total, nil
}