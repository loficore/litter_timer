package log

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatLine(t *testing.T) {
	got := formatLine("2026-08-02T15:04:05Z", "INFO", "msg")
	want := "[2026-08-02T15:04:05Z] [INFO]  msg"
	if got != want {
		t.Errorf("formatLine = %q, want %q", got, want)
	}
}

func TestOpenLogDirRotation(t *testing.T) {
	// Fixture (a): reopen-append when file < 10MB.
	t.Run("reopen_append_small_file", func(t *testing.T) {
		dir := t.TempDir()
		seedName := time.Now().Format("2006-01-02") + ".log"
		seedPath := filepath.Join(dir, seedName)
		if err := os.WriteFile(seedPath, make([]byte, 1024*1024), 0644); err != nil {
			t.Fatal(err)
		}
		f, err := openLogDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if filepath.Base(f.Name()) != seedName {
			t.Errorf("expected same file, got %s", f.Name())
		}
	})

	// Fixture (b): rotate when file >= 10MB + 1 byte.
	t.Run("rotate_large_file", func(t *testing.T) {
		dir := t.TempDir()
		seedName := time.Now().Format("2006-01-02") + ".log"
		seedPath := filepath.Join(dir, seedName)
		// Create a sparse file at 10MB + 1 byte.
		sf, err := os.Create(seedPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sf.Seek(10*1024*1024, 0); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		if _, err := sf.Write([]byte{0}); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		sf.Close()

		f, err := openLogDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if filepath.Base(f.Name()) == seedName {
			t.Errorf("expected new file, got same seed file %s", f.Name())
		}
		// New file should be empty (or near-empty).
		info, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > 10 {
			t.Errorf("new file size %d, expected near-empty", info.Size())
		}
	})
}

func TestOpenLogDirEmptyReturnsStderr(t *testing.T) {
	f, err := openLogDir("")
	if err != nil {
		t.Fatal(err)
	}
	if f != os.Stderr {
		t.Errorf("openLogDir(\"\") = %v, want os.Stderr", f)
	}
}

func TestInitSinkNonAndroid(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.log")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	handler := initSink(file)

	// Type assertion: must be *textHandler, not bare *slog.TextHandler.
	th, ok := handler.(*textHandler)
	if !ok {
		t.Fatalf("initSink returned %T, expected *textHandler", handler)
	}

	// Write a record and verify the file output contains the custom format.
	r := slog.NewRecord(time.Date(2026, 8, 2, 15, 4, 5, 0, time.UTC), slog.LevelInfo, "test message", 0)
	r.Add("key", "value")
	if err := th.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	// Read back the file contents.
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)

	// The textHandler.Handle writes attrs first, then formatLine + newline.
	// Expected: " key=value[2026-08-02T15:04:05Z] [INFO]  test message\n"
	wantSubstr := "[2026-08-02T15:04:05Z] [INFO]  test message"
	if !contains(got, wantSubstr) {
		t.Errorf("file content %q does not contain %q", got, wantSubstr)
	}
	if !contains(got, "key=value") {
		t.Errorf("file content %q does not contain attr key=value", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Daily rotation tests (TDD red phase) ---

func TestOpenLogDirDailyNaming(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	expectedName := today.Format("2006-01-02") + ".log"

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir(empty dir) created file %q, want %q", got, expectedName)
	}
}

func TestOpenLogDirDailyReopen(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	todayName := today.Format("2006-01-02") + ".log"
	yesterdayName := yesterday.Format("2006-01-02") + ".log"

	// Seed yesterday's file at 1MB — smaller than today's so old code picks it
	if err := os.WriteFile(filepath.Join(dir, yesterdayName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Seed today's file at 1MB + 100 bytes — larger so old code prefers yesterday's
	if err := os.WriteFile(filepath.Join(dir, todayName), make([]byte, 1024*1024+100), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != todayName {
		t.Errorf("openLogDir re-opened %q, want today's file %q", got, todayName)
	}
}

func TestOpenLogDirDailyRotate(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	yesterdayName := yesterday.Format("2006-01-02") + ".log"
	todayName := today.Format("2006-01-02") + ".log"

	// Seed yesterday's file at 1MB
	if err := os.WriteFile(filepath.Join(dir, yesterdayName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != todayName {
		t.Errorf("openLogDir created %q, want today's date-based file %q", got, todayName)
	}
}

func TestOpenLogDirDailySizeOverflow(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	expectedName := prefix + ".2.log"

	// Create today's base file at 10MB + 1 byte
	seedPath := filepath.Join(dir, baseName)
	sf, err := os.Create(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sf.Seek(10*1024*1024, 0); err != nil {
		sf.Close()
		t.Fatal(err)
	}
	if _, err := sf.Write([]byte{0}); err != nil {
		sf.Close()
		t.Fatal(err)
	}
	sf.Close()

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir created %q, want overflow suffix file %q", got, expectedName)
	}
}

func TestOpenLogDirDailyMultipleOverflow(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	suffix2Name := prefix + ".2.log"
	expectedName := prefix + ".3.log"

	// Create both base and .2.log at 10MB + 1
	for _, name := range []string{baseName, suffix2Name} {
		seedPath := filepath.Join(dir, name)
		sf, err := os.Create(seedPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sf.Seek(10*1024*1024, 0); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		if _, err := sf.Write([]byte{0}); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		sf.Close()
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir created %q, want next overflow suffix file %q", got, expectedName)
	}
}

func TestOpenLogDirDailyBasePreferred(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	suffix2Name := prefix + ".2.log"

	// Seed both base and .2.log: base at 1MB + 100 bytes (larger), .2.log at 1MB (smaller)
	// Old code sorts by size descending, picks smallest (.2.log) — wrong behavior.
	if err := os.WriteFile(filepath.Join(dir, baseName), make([]byte, 1024*1024+100), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, suffix2Name), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != baseName {
		t.Errorf("openLogDir opened %q, want base file %q (preferred when under cap)", got, baseName)
	}
}

func TestOpenLogDirDailyIgnoresMalformed(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"

	// Create the valid base file at 1MB
	if err := os.WriteFile(filepath.Join(dir, baseName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Malformed suffix (non-integer)
	if err := os.WriteFile(filepath.Join(dir, prefix+".abc.log"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Wrong extension
	if err := os.WriteFile(filepath.Join(dir, prefix+".txt"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	// Double extension (not .log)
	if err := os.WriteFile(filepath.Join(dir, prefix+".log.bak"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	// Must not panic and must return the valid base file.
	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != baseName {
		t.Errorf("openLogDir opened %q, want valid base file %q (malformed files ignored)", got, baseName)
	}
}