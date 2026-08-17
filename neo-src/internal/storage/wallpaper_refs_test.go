package storage

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Test helper — small wrapper that opens a temp DB and seeds wallpaper rows.
// -----------------------------------------------------------------------------

// seedWallpaperRefs inserts a habit_set, a habit, and updates the settings
// row (id=1) to all reference the same wallpaper ref.  Returns the habit_set
// id and habit id.
func seedWallpaperRefs(t *testing.T, m *SqliteManager, ref string) (int64, int64) {
	t.Helper()
	db := m.DB()

	// Create a habit_set with the wallpaper ref.
	res, err := db.Exec(
		`INSERT INTO habit_sets (name, description, color, wallpaper) VALUES (?, ?, ?, ?)`,
		"test-set", "test desc", "#123456", ref,
	)
	if err != nil {
		t.Fatalf("seed habit_set: %v", err)
	}
	setID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed habit_set LastInsertId: %v", err)
	}

	// Create a habit under that set with the wallpaper ref.
	res, err = db.Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper) VALUES (?, ?, ?, ?, ?)`,
		setID, "test-habit", 600, "#654321", ref,
	)
	if err != nil {
		t.Fatalf("seed habit: %v", err)
	}
	habitID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed habit LastInsertId: %v", err)
	}

	// Update the default settings row (id=1) to reference the wallpaper.
	_, err = db.Exec(`UPDATE settings SET wallpaper = ? WHERE id = 1`, ref)
	if err != nil {
		t.Fatalf("seed settings wallpaper: %v", err)
	}

	return setID, habitID
}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestCountWallpaperRefsWithSeededData(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	count, err := m.CountWallpaperRefs(ref)
	if err != nil {
		t.Fatalf("CountWallpaperRefs: %v", err)
	}
	if count != 3 {
		t.Errorf("CountWallpaperRefs(%q) = %d, want 3", ref, count)
	}
}

func TestUnbindWallpaperClearsAllThree(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	affected, err := m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper: %v", err)
	}
	if affected != 3 {
		t.Errorf("UnbindWallpaper affected = %d, want 3", affected)
	}

	// Verify all three tables' wallpaper columns are now empty.
	var habitWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM habits WHERE name = 'test-habit'`,
	).Scan(&habitWP); err != nil {
		t.Fatalf("read habits wallpaper: %v", err)
	}
	if habitWP != "" {
		t.Errorf("habits wallpaper after unbind: got %q, want \"\"", habitWP)
	}

	var setWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'test-set'`,
	).Scan(&setWP); err != nil {
		t.Fatalf("read habit_sets wallpaper: %v", err)
	}
	if setWP != "" {
		t.Errorf("habit_sets wallpaper after unbind: got %q, want \"\"", setWP)
	}

	var settingsWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM settings WHERE id = 1`,
	).Scan(&settingsWP); err != nil {
		t.Fatalf("read settings wallpaper: %v", err)
	}
	if settingsWP != "" {
		t.Errorf("settings wallpaper after unbind: got %q, want \"\"", settingsWP)
	}
}

func TestUnbindWallpaperIdempotent(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	// First unbind — should return 3.
	affected, err := m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper #1: %v", err)
	}
	if affected != 3 {
		t.Errorf("UnbindWallpaper #1 = %d, want 3", affected)
	}

	// Second unbind — should return 0 (idempotent).
	affected, err = m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper #2: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper #2 = %d, want 0 (idempotent)", affected)
	}

	// Count should also be 0.
	count, err := m.CountWallpaperRefs(ref)
	if err != nil {
		t.Fatalf("CountWallpaperRefs after unbind: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs after unbind = %d, want 0", count)
	}
}

func TestCountWallpaperRefsEmptyDB(t *testing.T) {
	m := openTempSqlite(t)

	// Fresh DB — no rows should reference any wallpaper.
	count, err := m.CountWallpaperRefs("local:nonexistent.png")
	if err != nil {
		t.Fatalf("CountWallpaperRefs on empty DB: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs on empty DB = %d, want 0", count)
	}
}

func TestUnbindWallpaperEmptyDB(t *testing.T) {
	m := openTempSqlite(t)

	affected, err := m.UnbindWallpaper("local:nonexistent.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper on empty DB: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper on empty DB = %d, want 0", affected)
	}
}

// TestLegacyWallpaperNotCounted verifies that legacy Zig-era values like
// `/wallpapers/book.jpg` (which lack the `local:` prefix) are NOT matched
// by CountWallpaperRefs or UnbindWallpaper.  The SQL uses exact-match
// `wallpaper = ?`, so only the exact same string is affected.
func TestLegacyWallpaperNotCounted(t *testing.T) {
	m := openTempSqlite(t)
	db := m.DB()

	// Seed a habit_set and habit with a legacy wallpaper path.
	_, err := db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"legacy-set", "#000", "/wallpapers/book.jpg",
	)
	if err != nil {
		t.Fatalf("seed legacy habit_set: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper)
		 SELECT id, ?, ?, ?, ? FROM habit_sets WHERE name = 'legacy-set'`,
		"legacy-habit", 600, "#111", "/wallpapers/book.jpg",
	)
	if err != nil {
		t.Fatalf("seed legacy habit: %v", err)
	}

	// Counting a local: ref should return 0 — the legacy rows are NOT matched.
	count, err := m.CountWallpaperRefs("local:a.png")
	if err != nil {
		t.Fatalf("CountWallpaperRefs for local: ref: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs(local:a.png) with legacy rows = %d, want 0", count)
	}

	// Unbinding a local: ref should also leave legacy rows untouched.
	affected, err := m.UnbindWallpaper("local:a.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper for local: ref: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper(local:a.png) with legacy rows = %d, want 0", affected)
	}

	// Verify the legacy rows still have their wallpaper.
	var wp string
	if err := db.QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'legacy-set'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read legacy habit_set: %v", err)
	}
	if wp != "/wallpapers/book.jpg" {
		t.Errorf("legacy habit_set wallpaper after unbind: got %q, want /wallpapers/book.jpg", wp)
	}

	if err := db.QueryRow(
		`SELECT wallpaper FROM habits WHERE name = 'legacy-habit'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read legacy habit: %v", err)
	}
	if wp != "/wallpapers/book.jpg" {
		t.Errorf("legacy habit wallpaper after unbind: got %q, want /wallpapers/book.jpg", wp)
	}
}

// TestUnbindWallpaperOnlyAffectsMatchingRef verifies that unbinding one
// wallpaper ref does not clear other wallpaper refs.
func TestUnbindWallpaperOnlyAffectsMatchingRef(t *testing.T) {
	m := openTempSqlite(t)
	db := m.DB()

	// Seed one set with ref A and one with ref B.
	_, err := db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"set-a", "#aaa", "local:a.png",
	)
	if err != nil {
		t.Fatalf("seed set-a: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"set-b", "#bbb", "local:b.png",
	)
	if err != nil {
		t.Fatalf("seed set-b: %v", err)
	}

	// Unbind only ref A.
	affected, err := m.UnbindWallpaper("local:a.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper: %v", err)
	}
	if affected != 1 {
		t.Errorf("UnbindWallpaper affected = %d, want 1", affected)
	}

	// Ref B should still be there.
	var wp string
	if err := db.QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'set-b'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read set-b: %v", err)
	}
	if wp != "local:b.png" {
		t.Errorf("set-b wallpaper after unbind of a: got %q, want local:b.png", wp)
	}
}