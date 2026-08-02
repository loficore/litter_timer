package storage

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"little-timer/internal/domain"
)

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

// openTempSqlite spins up a fresh SqliteManager at t.TempDir()/test.db,
// runs Migrate, and registers cleanup.  Each test gets its own file so
// they don't share state.
func openTempSqlite(t *testing.T) *SqliteManager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	m := NewSqliteManager().Init(path)
	if err := m.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// expectedTables mirrors the v0 schema tables (excluding `schema_version`
// and `backup_config`, which the manager verifies but doesn't create on
// fresh-DB migration; they exist on first migrateToV7/v8 from an older
// schema, never on greenfield).
var expectedTables = []string{
	"health_check",
	"habit_sets",
	"habits",
	"sessions",
	"timer_sessions",
	"settings",
}

// -----------------------------------------------------------------------------
// Schema tests
// -----------------------------------------------------------------------------

func TestFreshDatabaseCreatesSchema(t *testing.T) {
	m := openTempSqlite(t)

	for _, table := range expectedTables {
		var name string
		err := m.DB().QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?;`,
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
			continue
		}
		if name != table {
			t.Errorf("expected table %q, got %q", table, name)
		}
	}
}

func TestSchemaVersionIs8(t *testing.T) {
	m := openTempSqlite(t)

	var version int
	if err := m.DB().QueryRow(`SELECT MAX(version) FROM schema_version;`).Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 8 {
		t.Errorf("schema version: got %d, want 8", version)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("CurrentSchemaVersion constant: got %d, want 8", CurrentSchemaVersion)
	}
}

func TestSettingsTableConstraints(t *testing.T) {
	m := openTempSqlite(t)

	// Default-row INSERT happens during migration; verify the seed is
	// present and matches the Zig defaults (timezone=8, language='ZH',
	// duration_seconds=1500, …).
	var (
		timezone    int64
		language    string
		defaultMode string
		durationSec int64
	)
	err := m.DB().QueryRow(
		`SELECT timezone, language, default_mode, duration_seconds FROM settings WHERE id = 1;`,
	).Scan(&timezone, &language, &defaultMode, &durationSec)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if timezone != 8 {
		t.Errorf("default timezone: got %d, want 8", timezone)
	}
	if language != "ZH" {
		t.Errorf("default language: got %q, want \"ZH\"", language)
	}
	if defaultMode != "countdown" {
		t.Errorf("default default_mode: got %q, want \"countdown\"", defaultMode)
	}
	if durationSec != 1500 {
		t.Errorf("default duration_seconds: got %d, want 1500", durationSec)
	}

	// CHECK constraints: out-of-range timezone should reject the write.
	if _, err := m.DB().Exec(
		`UPDATE settings SET timezone = 99 WHERE id = 1;`,
	); err == nil {
		t.Errorf("expected CHECK(timezone BETWEEN -12 AND 14) to reject 99")
	} else if !strings.Contains(err.Error(), "CHECK") && !strings.Contains(err.Error(), "constraint") {
		// sqlite returns "CHECK constraint failed" — accept either wording.
		t.Logf("rejection error (acceptable): %v", err)
	}
}

func TestIndexesAreCreated(t *testing.T) {
	m := openTempSqlite(t)

	wantIndexes := map[string]bool{
		"idx_habits_set_id":             false,
		"idx_habits_name":               false,
		"idx_sessions_habit_id":         false,
		"idx_sessions_date":             false,
		"idx_settings_timezone":         false,
		"idx_settings_language":         false,
		"idx_health_check_status":       false,
		"idx_timer_sessions_habit_id":   false,
		"idx_timer_sessions_is_running": false,
	}
	rows, err := m.DB().Query(
		`SELECT name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%';`,
	)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := wantIndexes[name]; ok {
			wantIndexes[name] = true
		}
	}
	for name, found := range wantIndexes {
		if !found {
			t.Errorf("index %q not created", name)
		}
	}
}

func TestFilePermissionIs0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "perm.db")
	m := NewSqliteManager().Init(path)
	if err := m.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode: got %o, want 0600", got)
	}
}

// -----------------------------------------------------------------------------
// Settings round-trip
// -----------------------------------------------------------------------------

func TestSettingsRoundTrip(t *testing.T) {
	m := openTempSqlite(t)

	want := domain.SettingsConfig{
		Basic: domain.SettingsBasic{
			Timezone:    8,
			Language:    "EN",
			DefaultMode: domain.DefaultModeStopwatch,
			ThemeMode:   "light",
			Wallpaper:   "/tmp/wall.jpg",
		},
		ClockDefaults: domain.ClockTaskConfig{
			Countdown: domain.CountdownConfig{
				DurationSeconds:     600,
				Loop:                true,
				LoopCount:           4,
				LoopIntervalSeconds: 30,
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: 7200,
			},
		},
		Logging: domain.SettingsLogging{
			Level:           "DEBUG",
			EnableTimestamp: false,
			TickIntervalMs:  500,
		},
	}

	if err := m.SaveSettings(want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	got, err := m.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}

	if got.Basic.Timezone != want.Basic.Timezone {
		t.Errorf("Timezone: got %d, want %d", got.Basic.Timezone, want.Basic.Timezone)
	}
	if got.Basic.Language != want.Basic.Language {
		t.Errorf("Language: got %q, want %q", got.Basic.Language, want.Basic.Language)
	}
	if got.Basic.DefaultMode != want.Basic.DefaultMode {
		t.Errorf("DefaultMode: got %v, want %v", got.Basic.DefaultMode, want.Basic.DefaultMode)
	}
	if got.Basic.ThemeMode != want.Basic.ThemeMode {
		t.Errorf("ThemeMode: got %q, want %q", got.Basic.ThemeMode, want.Basic.ThemeMode)
	}
	if got.Basic.Wallpaper != want.Basic.Wallpaper {
		t.Errorf("Wallpaper: got %q, want %q", got.Basic.Wallpaper, want.Basic.Wallpaper)
	}
	if got.ClockDefaults.Countdown.DurationSeconds != want.ClockDefaults.Countdown.DurationSeconds {
		t.Errorf("Countdown.DurationSeconds: got %d, want %d",
			got.ClockDefaults.Countdown.DurationSeconds,
			want.ClockDefaults.Countdown.DurationSeconds)
	}
	if got.ClockDefaults.Countdown.Loop != want.ClockDefaults.Countdown.Loop {
		t.Errorf("Countdown.Loop: got %v, want %v",
			got.ClockDefaults.Countdown.Loop, want.ClockDefaults.Countdown.Loop)
	}
	if got.ClockDefaults.Countdown.LoopCount != want.ClockDefaults.Countdown.LoopCount {
		t.Errorf("Countdown.LoopCount: got %d, want %d",
			got.ClockDefaults.Countdown.LoopCount, want.ClockDefaults.Countdown.LoopCount)
	}
	if got.ClockDefaults.Stopwatch.MaxSeconds != want.ClockDefaults.Stopwatch.MaxSeconds {
		t.Errorf("Stopwatch.MaxSeconds: got %d, want %d",
			got.ClockDefaults.Stopwatch.MaxSeconds,
			want.ClockDefaults.Stopwatch.MaxSeconds)
	}
	if got.Logging.Level != want.Logging.Level {
		t.Errorf("Logging.Level: got %q, want %q", got.Logging.Level, want.Logging.Level)
	}
	if got.Logging.EnableTimestamp != want.Logging.EnableTimestamp {
		t.Errorf("Logging.EnableTimestamp: got %v, want %v",
			got.Logging.EnableTimestamp, want.Logging.EnableTimestamp)
	}
	if got.Logging.TickIntervalMs != want.Logging.TickIntervalMs {
		t.Errorf("Logging.TickIntervalMs: got %d, want %d",
			got.Logging.TickIntervalMs, want.Logging.TickIntervalMs)
	}
}

func TestSettingsSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	want := domain.NewDefaultSettingsConfig()
	want.Basic.Language = "JP"
	want.Basic.Wallpaper = "wallpapers/sakura.png"
	want.ClockDefaults.Countdown.DurationSeconds = 3000

	// First lifecycle: write.
	{
		m := NewSqliteManager().Init(path)
		if err := m.Open(); err != nil {
			t.Fatalf("Open #1: %v", err)
		}
		if err := m.Migrate(); err != nil {
			t.Fatalf("Migrate #1: %v", err)
		}
		if err := m.SaveSettings(want); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}
		if err := m.Close(); err != nil {
			t.Fatalf("Close #1: %v", err)
		}
	}

	// Second lifecycle: reopen, read back.
	m := NewSqliteManager().Init(path)
	if err := m.Open(); err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	if err := m.Migrate(); err != nil {
		t.Fatalf("Migrate #2: %v", err)
	}

	got, err := m.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}

	if got.Basic.Language != want.Basic.Language {
		t.Errorf("Language after reopen: got %q, want %q",
			got.Basic.Language, want.Basic.Language)
	}
	if got.Basic.Wallpaper != want.Basic.Wallpaper {
		t.Errorf("Wallpaper after reopen: got %q, want %q",
			got.Basic.Wallpaper, want.Basic.Wallpaper)
	}
	if got.ClockDefaults.Countdown.DurationSeconds != want.ClockDefaults.Countdown.DurationSeconds {
		t.Errorf("Countdown.DurationSeconds after reopen: got %d, want %d",
			got.ClockDefaults.Countdown.DurationSeconds,
			want.ClockDefaults.Countdown.DurationSeconds)
	}

	// Schema version should still be 8 after reopen.
	var v int
	if err := m.DB().QueryRow(`SELECT MAX(version) FROM schema_version;`).Scan(&v); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if v != 8 {
		t.Errorf("schema version after reopen: got %d, want 8", v)
	}
}

// -----------------------------------------------------------------------------
// Habit CRUD
// -----------------------------------------------------------------------------

func TestHabitCRUDLifecycle(t *testing.T) {
	m := openTempSqlite(t)

	// Create a habit_set.
	setID, err := m.HabitSets().Create("Daily Reading", "books", "#abcdef")
	if err != nil {
		t.Fatalf("HabitSets.Create: %v", err)
	}
	if setID <= 0 {
		t.Errorf("HabitSets.Create: got id %d, want > 0", setID)
	}

	// Create a habit under it.
	habitID, err := m.Habits().Create(setID, "Read 30 min", 1800, "#123456")
	if err != nil {
		t.Fatalf("Habits.Create: %v", err)
	}
	if habitID <= 0 {
		t.Errorf("Habits.Create: got id %d, want > 0", habitID)
	}

	// List → should contain exactly one row matching.
	const limit, offset = 100, 0
	sets, err := m.HabitSets().List(limit, offset)
	if err != nil {
		t.Fatalf("HabitSets.List: %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("HabitSets.List len: got %d, want 1", len(sets))
	} else if sets[0].Name != "Daily Reading" || sets[0].Color != "#abcdef" {
		t.Errorf("HabitSets.List row: got %+v, want Daily Reading / #abcdef", sets[0])
	}

	// Habits scoped to the set.
	habits, err := m.Habits().ListBySet(setID, limit, offset)
	if err != nil {
		t.Fatalf("Habits.ListBySet: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("Habits.ListBySet len: got %d, want 1", len(habits))
	}
	if habits[0].Name != "Read 30 min" || habits[0].GoalSeconds != 1800 {
		t.Errorf("Habits row: got %+v, want Read 30 min / 1800", habits[0])
	}

	// Update the habit.
	if err := m.Habits().Update(habitID, "Read 45 min", 2700, "#654321", "/wallpapers/book.jpg"); err != nil {
		t.Fatalf("Habits.Update: %v", err)
	}
	got, err := m.Habits().GetByID(habitID)
	if err != nil {
		t.Fatalf("Habits.GetByID: %v", err)
	}
	if got.Name != "Read 45 min" || got.GoalSeconds != 2700 || got.Wallpaper != "/wallpapers/book.jpg" {
		t.Errorf("Habits.GetByID after Update: got %+v", got)
	}

	// Record a session for the habit.
	today := "2026-06-28"
	sessionID, err := m.Timers().CreateSession(habitID, 2700, 1, today)
	if err != nil {
		t.Fatalf("Timers.CreateSession: %v", err)
	}
	if sessionID <= 0 {
		t.Errorf("Timers.CreateSession: got id %d, want > 0", sessionID)
	}

	totals, err := m.Timers().ListSessionsByDate(today, limit, offset)
	if err != nil {
		t.Fatalf("Timers.ListSessionsByDate: %v", err)
	}
	if len(totals) != 1 || totals[0].DurationSeconds != 2700 {
		t.Errorf("ListSessionsByDate: got %+v, want 1 row of 2700s", totals)
	}

	totalSec, err := m.Timers().TodaySecondsForHabit(habitID, today)
	if err != nil {
		t.Fatalf("TodaySecondsForHabit: %v", err)
	}
	if totalSec != 2700 {
		t.Errorf("TodaySecondsForHabit: got %d, want 2700", totalSec)
	}

	// Delete the habit — sessions should cascade.
	if err := m.Habits().Delete(habitID); err != nil {
		t.Fatalf("Habits.Delete: %v", err)
	}
	if _, err := m.Habits().GetByID(habitID); err != ErrHabitNotFound {
		t.Errorf("GetByID after Delete: got %v, want ErrHabitNotFound", err)
	}

	// Cascade check: the session row is gone too.
	after, err := m.Timers().ListSessionsByDate(today, limit, offset)
	if err != nil {
		t.Fatalf("ListSessionsByDate after delete: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected cascade delete to remove session, got %d rows", len(after))
	}
}

func TestHabitNameExistsInSet(t *testing.T) {
	m := openTempSqlite(t)
	setID, err := m.HabitSets().Create("Set A", "", "#000")
	if err != nil {
		t.Fatalf("HabitSets.Create: %v", err)
	}
	otherSetID, err := m.HabitSets().Create("Set B", "", "#000")
	if err != nil {
		t.Fatalf("HabitSets.Create other set: %v", err)
	}

	habitID, err := m.Habits().Create(setID, "Read", 600, "#fff")
	if err != nil {
		t.Fatalf("Habits.Create: %v", err)
	}
	if _, err := m.Habits().Create(setID, "Write", 600, "#fff"); err != nil {
		t.Fatalf("Habits.Create other habit: %v", err)
	}
	tests := []struct {
		name      string
		setID     int64
		queryName string
		excludeID *int64
		want      bool
	}{
		{name: "same set existing name", setID: setID, queryName: "Read", want: true},
		{name: "different set does not conflict", setID: otherSetID, queryName: "Read", want: false},
		{name: "self update is excluded", setID: setID, queryName: "Read", excludeID: &habitID, want: false},
		{name: "different existing ID still conflicts", setID: setID, queryName: "Write", excludeID: &habitID, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Habits().NameExistsInSet(tt.setID, tt.queryName, tt.excludeID)
			if err != nil {
				t.Fatalf("NameExistsInSet: %v", err)
			}
			if got != tt.want {
				t.Errorf("NameExistsInSet = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimerSessionCRUD(t *testing.T) {
	m := openTempSqlite(t)

	setID, _ := m.HabitSets().Create("Work", "", "#000")
	habitID, _ := m.Habits().Create(setID, "Coding", 1500, "#fff")

	hid := habitID
	sessionID, err := m.Timers().CreateTimerSession(&hid, "countdown", 1500, 300, 4)
	if err != nil {
		t.Fatalf("CreateTimerSession: %v", err)
	}

	got, err := m.Timers().GetTimerSessionByID(sessionID)
	if err != nil {
		t.Fatalf("GetTimerSessionByID: %v", err)
	}
	if got.Mode != "countdown" {
		t.Errorf("Mode: got %q, want \"countdown\"", got.Mode)
	}
	if got.WorkDuration != 1500 || got.RestDuration != 300 || got.LoopCount != 4 {
		t.Errorf("durations: got w=%d r=%d loop=%d, want 1500/300/4",
			got.WorkDuration, got.RestDuration, got.LoopCount)
	}
	if !got.IsRunning || got.IsFinished || got.IsPaused {
		t.Errorf("initial flags: running=%v finished=%v paused=%v, want T/F/F",
			got.IsRunning, got.IsFinished, got.IsPaused)
	}

	// Update → marks as paused, advances current_round.
	remaining := int64(900)
	lastSync := int64(1234567890)
	if err := m.Timers().UpdateTimerSession(
		sessionID,
		600, // elapsed
		&remaining,
		0, // paused_total
		nil,
		&lastSync,
		false, // is_running
		true,  // is_paused
		false, // is_finished
		2,     // current_round
		false, // in_rest
	); err != nil {
		t.Fatalf("UpdateTimerSession: %v", err)
	}

	updated, err := m.Timers().GetTimerSessionByID(sessionID)
	if err != nil {
		t.Fatalf("GetTimerSessionByID after update: %v", err)
	}
	if updated.ElapsedSeconds != 600 || updated.CurrentRound != 2 {
		t.Errorf("after update: elapsed=%d round=%d, want 600/2",
			updated.ElapsedSeconds, updated.CurrentRound)
	}
	if updated.IsRunning || !updated.IsPaused || updated.IsFinished {
		t.Errorf("flags after update: running=%v paused=%v finished=%v, want F/T/F",
			updated.IsRunning, updated.IsPaused, updated.IsFinished)
	}
	if updated.RemainingSeconds == nil || *updated.RemainingSeconds != 900 {
		t.Errorf("RemainingSeconds: got %v, want 900", updated.RemainingSeconds)
	}

	// Active session query should return this row.
	active, err := m.Timers().GetActiveTimerSession()
	if err != nil {
		t.Fatalf("GetActiveTimerSession: %v", err)
	}
	if active.ID != sessionID {
		t.Errorf("active session: got id %d, want %d", active.ID, sessionID)
	}

	// Finish + verify the active query no longer returns it.
	if err := m.Timers().FinishTimerSession(sessionID); err != nil {
		t.Fatalf("FinishTimerSession: %v", err)
	}
	if _, err := m.Timers().GetActiveTimerSession(); err != ErrHabitNotFound {
		t.Errorf("GetActiveTimerSession after finish: got %v, want ErrHabitNotFound", err)
	}
}

// -----------------------------------------------------------------------------
// Health check
// -----------------------------------------------------------------------------

func TestHealthCheckIsHealthy(t *testing.T) {
	m := openTempSqlite(t)

	if err := m.Health().Initialize(); err != nil {
		t.Fatalf("Health.Initialize: %v", err)
	}
	if err := m.PerformHealthCheck(); err != nil {
		t.Fatalf("PerformHealthCheck: %v", err)
	}

	ok, err := m.IsHealthy()
	if err != nil {
		t.Fatalf("IsHealthy: %v", err)
	}
	if !ok {
		t.Errorf("expected healthy after fresh DB")
	}

	info, err := m.GetHealthInfo()
	if err != nil {
		t.Fatalf("GetHealthInfo: %v", err)
	}
	if info.Status != "healthy" {
		t.Errorf("Status: got %q, want \"healthy\"", info.Status)
	}
}

// -----------------------------------------------------------------------------
// Unwritable-dir guard — pre-existing behavior, unchanged.
// -----------------------------------------------------------------------------

func TestOpenUnwritableDir(t *testing.T) {
	m := NewSqliteManager().Init("/proc/nonexistent/little_timer.db")
	err := m.Open()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrDatabaseOpenFailed) {
		t.Errorf("expected ErrDatabaseOpenFailed, got: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
}

func TestHabitSetsColumnsMatchZigSchema(t *testing.T) {
	m := openTempSqlite(t)

	cols, err := tableColumns(m.DB(), "habit_sets")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	want := []string{
		"id", "name", "description", "color", "wallpaper", "created_at",
	}
	got := make([]string, len(cols))
	for i, c := range cols {
		got[i] = c.Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("habit_sets columns: got %v, want %v", got, want)
	}
}

func TestSettingsColumnsMatchZigSchema(t *testing.T) {
	m := openTempSqlite(t)

	cols, err := tableColumns(m.DB(), "settings")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}

	want := []string{
		"id", "timezone", "language", "default_mode", "theme_mode", "wallpaper",
		"duration_seconds", "countdown_loop", "countdown_loop_count",
		"countdown_loop_interval", "stopwatch_max_seconds",
		"log_level", "log_enable_timestamp", "log_tick_interval", "updated_at",
	}
	got := make([]string, len(cols))
	for i, c := range cols {
		got[i] = c.Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("settings columns: got %v, want %v", got, want)
	}
}

type columnInfo struct {
	CID     int
	Name    string
	Type    string
	NotNull int
	Dflt    *string
	PK      int
}

func tableColumns(db *sql.DB, table string) ([]columnInfo, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []columnInfo
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.CID, &c.Name, &c.Type, &c.NotNull, &c.Dflt, &c.PK); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
