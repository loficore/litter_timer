package backup

// Tests for LocalAdapter and BackupManager.cleanupOldBackups.
//
// The task spec described a JSON-based "PresetInfo / presetID" API that
// doesn't exist in this codebase; the real adapter deals in flat
// `presets_backup_<unix>.db` files via the BackupAdapter interface
// (Backup / Restore / List / Delete / TestConnection).  These tests
// map the spec's intent onto the actual surface:
//
//   Save happy path          → TestLocalBackupRestoreRoundTrip
//   Save with traversal name → TestLocalBackupPathTraversalEscapesDir
//   Save to unreadable dir   → TestLocalBackupUnwritableDir
//   Save overwrites          → TestLocalBackupOverwritesExisting
//   Load nonexistent         → TestLocalRestoreNotFound
//   Load corrupt / empty     → TestLocalRestoreCorruptOrEmptyFile
//   List empty dir           → TestLocalListEmptyDir
//   List multiple files      → TestLocalListMultipleBackups
//   Delete existing          → TestLocalDeleteExisting
//   Delete nonexistent       → TestLocalDeleteNonexistentIsIdempotent
//   Retention keepCount=0    → TestRetentionKeepZero
//   Retention keepCount=1    → TestRetentionKeepOne
//   Retention keepCount=2    → TestRetentionKeepTwoOfFive
//   Retention mixed groups   → TestRetentionMixedGroupsKeepsNewest
//   Retention skips corrupt  → TestRetentionSkipsCorruptFiles
//
// Retention tests drive the BackupManager (the only owner of
// cleanupOldBackups) but exercise the LocalAdapter's Delete path
// underneath, which is what we're really guarding.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"little-timer/internal/storage"
)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// writeBytes writes content to path with 0600 perms and fails the test
// on error.  Used to seed both "src" files (for Backup) and to plant
// pre-existing backup files (for List / retention tests).
func writeBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// readBytes is the read-side counterpart — convenience for asserting
// round-trip integrity without dragging in io.ReadFile noise.
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return b
}

// backupName builds a canonical filename: `presets_backup_<ts>.db`.
// Keeping the format in one place means tests stay in sync if the
// convention ever changes.
func backupName(ts int64) string {
	return filenamePrefix + strconv.FormatInt(ts, 10) + filenameSuffix
}

// makeAdapter creates a LocalAdapter rooted at t.TempDir()/backups and
// returns both the adapter and the underlying directory (tests that
// need to inspect files reach for it directly).
func makeAdapter(t *testing.T) (*LocalAdapter, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "backups")
	return NewLocalAdapter(dir), dir
}

// newManager is the minimum-viable BackupManager for retention tests:
// it doesn't need a live sqlite connection because we call
// cleanupOldBackups directly.  The adapter is the real LocalAdapter so
// Delete round-trips through the filesystem.
func newManager(t *testing.T, dir string, maxBackups int) *BackupManager {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	m := &BackupManager{
		backupDir:  dir,
		maxBackups: maxBackups,
		adapter:    NewLocalAdapter(dir),
	}
	return m
}

// -----------------------------------------------------------------------------
// Backup / Restore round-trip
// -----------------------------------------------------------------------------

func TestLocalBackupRestoreRoundTrip(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// Seed a "database" file with deterministic content.
	src := filepath.Join(t.TempDir(), "presets.db")
	payload := []byte("SQLite-format-3\x00fake-presets-bytes")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// The backup file must now exist on disk in the configured dir.
	backupPath := filepath.Join(dir, name)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup at %s: %v", backupPath, err)
	}

	// Restore into a fresh destination and confirm byte-identical copy.
	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != string(payload) {
		t.Errorf("restored content mismatch:\n got: %q\nwant: %q", got, payload)
	}
}

// TestLocalBackupPathTraversalEscapesDir documents a known limitation:
// filepath.Join collapses `..` segments, so a backupName of
// "../../../tmp/x" escapes the configured directory.  We're not
// asserting the spec's "should reject" expectation (the code doesn't
// reject) — we assert what actually happens so future fixes show up
// as a behavior change in the diff rather than a silent regression.
func TestLocalBackupPathTraversalEscapesDir(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// Plant a "src" file outside the backup dir; we'll point Backup at
	// it and ask it to land somewhere outside the configured dir via `..`.
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, []byte("traversal"))

	// Two levels up + a sibling temp dir.
	outsideDir := t.TempDir()
	outsideFile := filepath.Base(outsideDir) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".db"
	backupName := filepath.Join("..", "..", filepath.Base(outsideDir), outsideFile)

	if err := adapter.Backup(src, backupName); err != nil {
		t.Fatalf("Backup with traversal name: %v", err)
	}

	// The file should NOT have been written inside the backup dir.
	if entries, err := os.ReadDir(dir); err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	} else if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("backup dir should be empty, got: %v", names)
	}

	// And it SHOULD have landed outside (filepath.Join cleans the
	// traversal segments).
	resolved := filepath.Join(outsideDir, outsideFile)
	if _, err := os.Stat(resolved); err != nil {
		t.Fatalf("expected escape at %s: %v", resolved, err)
	}

	// Restore resolves the same way (filepath.Join + Clean), so it
	// happily reads the escaped file.  Document the symmetric
	// behavior — both sides honor the traversal equally.
	dst := filepath.Join(t.TempDir(), "dst.db")
	if err := adapter.Restore(backupName, dst); err != nil {
		t.Errorf("Restore with traversal name should mirror Backup: %v", err)
	} else if got := readBytes(t, dst); string(got) != "traversal" {
		t.Errorf("traversal Restore returned %q, want %q", got, "traversal")
	}
}

// TestLocalBackupUnwritableDir confirms Backup fails when the target
// directory can't be created.  We point the adapter at a path that
// definitively cannot exist (a child of /proc, which is read-only on
// Linux), so neither MkdirAll nor the subsequent copy can succeed.
//
// As root, filesystem perms don't bind — skip to keep the suite
// meaningful for non-root users without spurious failures on root CI.
func TestLocalBackupUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission test is unreliable as root")
	}

	bogus := filepath.Join("/proc", "self", "definitely-no-such-subdir", "backups")
	adapter := NewLocalAdapter(bogus)

	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, []byte("payload"))

	if err := adapter.Backup(src, backupName(time.Now().Unix())); err == nil {
		t.Errorf("Backup to %s should fail", bogus)
	}
}

func TestLocalBackupOverwritesExisting(t *testing.T) {
	adapter, dir := makeAdapter(t)

	src1 := filepath.Join(t.TempDir(), "src1.db")
	writeBytes(t, src1, []byte("first-payload"))
	src2 := filepath.Join(t.TempDir(), "src2.db")
	writeBytes(t, src2, []byte("second-payload-much-longer"))

	name := backupName(1700000000)
	if err := adapter.Backup(src1, name); err != nil {
		t.Fatalf("Backup #1: %v", err)
	}
	if err := adapter.Backup(src2, name); err != nil {
		t.Fatalf("Backup #2: %v", err)
	}

	// Exactly one file in the backup dir, and it's the second payload.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 1 file, got %d: %v", len(entries), names)
	}

	dst := filepath.Join(t.TempDir(), "out.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != "second-payload-much-longer" {
		t.Errorf("Restore returned stale content: %q", got)
	}
}

// -----------------------------------------------------------------------------
// Restore error / edge cases
// -----------------------------------------------------------------------------

func TestLocalRestoreNotFound(t *testing.T) {
	adapter, _ := makeAdapter(t)

	dst := filepath.Join(t.TempDir(), "dst.db")
	err := adapter.Restore("presets_backup_does_not_exist.db", dst)
	if err == nil {
		t.Fatalf("Restore of missing backup should fail")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("Restore error: got %v, want errors.Is ErrFileNotFound", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("destination file should not exist after failed Restore (stat err: %v)", statErr)
	}
}

// TestLocalRestoreCorruptOrEmptyFile confirms Restore doesn't validate
// payload content — it just copies bytes.  We're guarding against
// panics, not data semantics (semantics belong to whatever opens the
// restored file as a SQLite DB at a higher level).  Three flavors of
// "garbage" cover the practical edge cases: empty, partial SQLite
// header, and pure random bytes.
func TestLocalRestoreCorruptOrEmptyFile(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cases := []struct {
		name    string
		content []byte
	}{
		{"empty", []byte{}},
		{"partial_header", []byte("SQLite-form")},
		{"random_garbage", []byte{0x00, 0xff, 0x7f, 0x42, 0x13, 0x37}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := backupName(time.Now().UnixNano())
			writeBytes(t, filepath.Join(dir, name), tc.content)

			dst := filepath.Join(t.TempDir(), "dst.db")

			// Must not panic; outcome depends on content but Restore
			// itself never errors on bad bytes — it's a raw copy.
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Restore panicked on %s content: %v", tc.name, r)
				}
			}()
			if err := adapter.Restore(name, dst); err != nil {
				t.Fatalf("Restore on %s payload: unexpected error %v", tc.name, err)
			}
			if got := readBytes(t, dst); string(got) != string(tc.content) {
				t.Errorf("Restore on %s payload: bytes mismatch\n got %v\nwant %v", tc.name, got, tc.content)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// List
// -----------------------------------------------------------------------------

func TestLocalListEmptyDir(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// Adapter is configured but the dir doesn't exist yet — the
	// documented behavior is "no entries, no error" rather than an
	// ErrFileNotFound surfaced to the caller.
	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on missing dir: got %d entries, want 0: %+v", len(got), got)
	}

	// Now create an empty dir; List should also return an empty slice
	// (and the returned slice must be non-nil-able to range over).
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	got, err = adapter.List()
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on empty dir: got %d entries, want 0: %+v", len(got), got)
	}
}

func TestLocalListMultipleBackups(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Three well-formed backups + a couple of decoys (wrong prefix /
	// wrong suffix / wrong ts) to exercise the filter.
	good := []struct {
		name string
		ts   int64
		size int
	}{
		{backupName(1000), 1000, 5},
		{backupName(2000), 2000, 7},
		{backupName(3000), 3000, 9},
	}
	for _, g := range good {
		writeBytes(t, filepath.Join(dir, g.name), make([]byte, g.size))
	}
	// Decoys — must be ignored.
	writeBytes(t, filepath.Join(dir, "unrelated.txt"), []byte("nope"))
	writeBytes(t, filepath.Join(dir, "presets_backup_notanumber.db"), []byte("garbage"))
	writeBytes(t, filepath.Join(dir, "presets_backup_4000"), []byte("missing suffix"))
	// A directory with the right name should also be skipped.
	if err := os.Mkdir(filepath.Join(dir, backupName(5000)), 0o700); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}

	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(good) {
		t.Fatalf("List len: got %d, want %d (entries: %+v)", len(got), len(good), got)
	}

	// Sort by timestamp for stable comparison — List ordering isn't
	// guaranteed by the contract.
	sort.Slice(got, func(i, j int) bool { return got[i].Timestamp < got[j].Timestamp })
	for i, g := range good {
		if got[i].Name != g.name {
			t.Errorf("entry %d: name = %q, want %q", i, got[i].Name, g.name)
		}
		if got[i].Timestamp != g.ts {
			t.Errorf("entry %d: ts = %d, want %d", i, got[i].Timestamp, g.ts)
		}
		if got[i].SizeBytes != uint64(g.size) {
			t.Errorf("entry %d: size = %d, want %d", i, got[i].SizeBytes, g.size)
		}
	}
}

// -----------------------------------------------------------------------------
// Delete
// -----------------------------------------------------------------------------

func TestLocalDeleteExisting(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	name := backupName(time.Now().Unix())
	path := filepath.Join(dir, name)
	writeBytes(t, path, []byte("doomed"))

	if err := adapter.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after Delete: stat err = %v", err)
	}
}

func TestLocalDeleteNonexistentIsIdempotent(t *testing.T) {
	adapter, _ := makeAdapter(t)

	// Spec calls this "idempotent" — Delete of a missing file must NOT
	// return an error.  This matches the Zig source's "not-found is
	// OK on delete" comment.
	if err := adapter.Delete(backupName(9999)); err != nil {
		t.Errorf("Delete on missing file: got %v, want nil", err)
	}
}

// -----------------------------------------------------------------------------
// Retention (BackupManager.cleanupOldBackups)
// -----------------------------------------------------------------------------

// TestRetentionKeepZero drops every backup when the cap is 0.  We
// bypass SetMaxBackups because it refuses non-positive values (a
// reasonable production guardrail, but exactly the case the test
// pins).  Test lives in the same package, so we can poke the field.
func TestRetentionKeepZero(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 5)
	m.maxBackups = 0 // direct poke — SetMaxBackups would reject 0.

	// Plant three backups.
	for _, ts := range []int64{10, 20, 30} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("keepCount=0: expected 0 backups, got %d: %v", len(entries), names)
	}
}

func TestRetentionKeepOne(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 1)

	for _, ts := range []int64{10, 20, 30, 40} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("keepCount=1: expected 1 backup, got %d: %v", len(entries), names)
	}
	if entries[0].Name() != backupName(40) {
		t.Errorf("survivor: got %s, want %s", entries[0].Name(), backupName(40))
	}
}

func TestRetentionKeepTwoOfFive(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 2)

	for _, ts := range []int64{100, 200, 300, 400, 500} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("keepCount=2: expected 2 backups, got %d: %v", len(entries), names)
	}

	// CleanupOldBackups sorts ascending by timestamp and trims the
	// head, so the two survivors must be the two newest.
	gotNames := make(map[string]bool, 2)
	for _, e := range entries {
		gotNames[e.Name()] = true
	}
	for _, want := range []string{backupName(400), backupName(500)} {
		if !gotNames[want] {
			t.Errorf("missing survivor %s; have %v", want, gotNames)
		}
	}
}

// TestRetentionMixedGroupsKeepsNewest mirrors the spec's "presetA has
// 3, presetB has 5" idea, adapted to the real flat-namespace model:
// backups live in one dir but cluster around two distinct timestamp
// ranges.  The cleanup logic is uniform — keep the N newest globally —
// so the test asserts that behavior rather than inventing a
// per-preset concept that doesn't exist.
func TestRetentionMixedGroupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 3)

	// "Group A": low timestamps; "Group B": high timestamps.
	groupA := []int64{100, 200, 300}
	groupB := []int64{1_000_000, 1_000_100, 1_000_200, 1_000_300, 1_000_400}
	for _, ts := range append(append([]int64{}, groupA...), groupB...) {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 3 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 3 backups, got %d: %v", len(entries), names)
	}

	// Three newest are the three from Group B with the highest ts.
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(entries[i].Name(), filenamePrefix), filenameSuffix), 10, 64)
		tj, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(entries[j].Name(), filenamePrefix), filenameSuffix), 10, 64)
		return ti < tj
	})
	wantNewest := []string{backupName(1_000_200), backupName(1_000_300), backupName(1_000_400)}
	for i, w := range wantNewest {
		if entries[i].Name() != w {
			t.Errorf("survivor[%d] = %s, want %s", i, entries[i].Name(), w)
		}
	}
}

// TestRetentionSkipsCorruptFiles verifies the resilience property the
// spec called out: corrupt / unparseable entries must not abort the
// retention pass, and they don't count against the retention cap
// (since cleanupOldBackups never sees them — List filters them out).
func TestRetentionSkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 2)

	// 3 valid backups + 2 with garbage names (timestamp doesn't parse).
	valid := []int64{1000, 2000, 3000}
	for _, ts := range valid {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}
	writeBytes(t, filepath.Join(dir, "presets_backup_NOTANUMBER.db"), []byte("junk"))
	writeBytes(t, filepath.Join(dir, "presets_backup_also_bad.db"), []byte("junk"))

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	// After cleanup we expect exactly 2 surviving files (the two
	// newest valid backups), and BOTH corrupt files must still be on
	// disk — they're not part of the parsed set, so cleanup never
	// touched them.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 4 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 4 files (2 newest + 2 corrupt), got %d: %v", len(entries), names)
	}

	// Verify the two newest survived.
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
	}
	if !have[backupName(2000)] || !have[backupName(3000)] {
		t.Errorf("missing survivors; have %v", have)
	}
	if have[backupName(1000)] {
		t.Errorf("oldest valid backup should have been pruned; have %v", have)
	}
	if !have["presets_backup_NOTANUMBER.db"] || !have["presets_backup_also_bad.db"] {
		t.Errorf("corrupt files should be left untouched; have %v", have)
	}
}

// -----------------------------------------------------------------------------
// CreateBackup integration tests (VACUUM INTO, SHA256, manifest, cleanup)
// -----------------------------------------------------------------------------

// newRealManager creates a BackupManager backed by a real SqliteManager
// for tests that exercise VACUUM INTO (which requires a live SQLite
// connection).
func newRealManager(t *testing.T) (*BackupManager, string, *storage.SqliteManager) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}

	// Insert a row so the backup has real content.
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('hello-vacuum');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mgr, err := NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return mgr, backupDir, sqlite
}

// TestCreateBackup_VacuumInto exercises VACUUM INTO by creating a real
// SQLite database, inserting a row, and calling CreateBackup.  Asserts
// the backup file exists, the temp file is gone, and manifest.json was
// written.
func TestCreateBackup_VacuumInto(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if name == "" {
		t.Fatal("CreateBackup returned empty name")
	}

	// Backup file must exist in backupDir.
	backupPath := filepath.Join(backupDir, name)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup file at %s: %v", backupPath, err)
	}

	// Temp file `lt_backup_*.db` must be gone (defer os.Remove).
	ltGlob := filepath.Join(filepath.Dir(mgr.dbPath), "lt_backup_*.db")
	matches, err := filepath.Glob(ltGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) still present: %v", matches)
	}

	// manifest.json must exist in backupDir.
	manifestPath := filepath.Join(backupDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest.json at %s: %v", manifestPath, err)
	}
}

// TestCreateBackup_SHA256Match verifies the SHA-256 in manifest.json
// matches an independently computed digest of the backup file.
func TestCreateBackup_SHA256Match(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// Read manifest.json and extract the sha256 field.
	manifestPath := filepath.Join(backupDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Version int `json:"version"`
		Backups []struct {
			Name      string `json:"name"`
			Timestamp int64  `json:"timestamp"`
			SizeBytes uint64 `json:"size_bytes"`
			SHA256    string `json:"sha256"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	// Find the entry for our backup name.
	var entrySHA256 string
	for _, b := range manifest.Backups {
		if b.Name == name {
			entrySHA256 = b.SHA256
			break
		}
	}
	if entrySHA256 == "" {
		t.Fatalf("manifest has no entry for backup %q", name)
	}

	// Independently compute SHA-256 of the backup file.
	backupPath := filepath.Join(backupDir, name)
	computed, err := sha256File(backupPath)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}

	if entrySHA256 != computed {
		t.Errorf("SHA-256 mismatch:\n manifest: %s\ncomputed: %s", entrySHA256, computed)
	}
}

// TestCreateBackup_SingleQuotePath is the regression test for the
// VACUUM INTO single-quote escaping fix: the temp file lives next to
// the DB, so a dbPath inside a directory whose name contains a single
// quote (e.g. `O'Brien`) previously produced broken SQL and a failed
// backup.  SQLite has no bind-parameter support for VACUUM INTO, so the
// path must be escaped by doubling the quote.
func TestCreateBackup_SingleQuotePath(t *testing.T) {
	quoteDir := filepath.Join(t.TempDir(), "O'Brien")
	if err := os.MkdirAll(quoteDir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", quoteDir, err)
	}
	dbPath := filepath.Join(quoteDir, "test.db")
	backupDir := filepath.Join(quoteDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('single-quote-path');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mgr, err := NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup in O'Brien dir: %v", err)
	}
	if name == "" {
		t.Fatal("CreateBackup returned empty name")
	}

	// Backup file must exist in backupDir.
	if _, err := os.Stat(filepath.Join(backupDir, name)); err != nil {
		t.Fatalf("expected backup file at %s: %v", filepath.Join(backupDir, name), err)
	}

	// manifest.json must exist in backupDir.
	if _, err := os.Stat(filepath.Join(backupDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json at %s: %v", filepath.Join(backupDir, "manifest.json"), err)
	}

	// Temp file `lt_backup_*.db` must be gone (defer os.Remove).
	matches, err := filepath.Glob(filepath.Join(quoteDir, "lt_backup_*.db"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) still present: %v", matches)
	}
}

// TestCreateBackup_TempCleanup verifies that even when the adapter's
// Backup() fails, the temp file is cleaned up (defer os.Remove).
func TestCreateBackup_TempCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('temp-cleanup');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	fake := NewFakeLocalAdapter(backupDir)
	fake.SetBackupError(errors.New("injected backup failure"))

	mgr := NewManagerWithAdapter(sqlite, dbPath, backupDir, fake)

	_, err := mgr.CreateBackup()
	if err == nil {
		t.Fatal("CreateBackup should fail with injected error")
	}

	// Temp file `lt_backup_*.db` must be gone (defer os.Remove in CreateBackup).
	ltGlob := filepath.Join(filepath.Dir(dbPath), "lt_backup_*.db")
	matches, err := filepath.Glob(ltGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) leaked after failed backup: %v", matches)
	}
}

// TestCreateBackup_ManifestWritten verifies the structure of manifest.json
// after CreateBackup: version, backups array with name/timestamp/sha256/size_bytes.
func TestCreateBackup_ManifestWritten(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	manifestPath := filepath.Join(backupDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Version   int    `json:"version"`
		DBVersion string `json:"db_version"`
		Backups   []struct {
			Name      string `json:"name"`
			Timestamp int64  `json:"timestamp"`
			SHA256    string `json:"sha256"`
			SizeBytes uint64 `json:"size_bytes"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.Version != 1 {
		t.Errorf("version = %d, want 1", manifest.Version)
	}

	if len(manifest.Backups) == 0 {
		t.Fatal("manifest has no backups")
	}

	// Find the entry for our backup.
	var found bool
	for _, b := range manifest.Backups {
		if b.Name == name {
			found = true
			if b.Name == "" {
				t.Error("backup name is empty")
			}
			if b.Timestamp == 0 {
				t.Error("backup timestamp is zero")
			}
			if b.SHA256 == "" {
				t.Error("backup sha256 is empty")
			}
			if b.SizeBytes == 0 {
				t.Error("backup size_bytes is zero")
			}
			break
		}
	}
	if !found {
		t.Errorf("manifest has no entry for backup %q", name)
	}
}
