package app

import (
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

func TestHabitServiceListSessionsPreservesExplicitDates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := sm.UpdateBasic(settings.BasicConfig{Timezone: 8, Language: "EN", DefaultMode: domain.DefaultModeStopwatch}); err != nil {
		t.Fatalf("update timezone: %v", err)
	}
	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)
	setID, err := sqlite.HabitSets().Create("Test", "", "#000000")
	if err != nil {
		t.Fatalf("create set: %v", err)
	}
	habitID, err := sqlite.Habits().Create(setID, "Focus", 60, "#000000")
	if err != nil {
		t.Fatalf("create habit: %v", err)
	}
	if _, err := sqlite.Timers().CreateSession(habitID, 60, 1, domain.TodayString(8)); err != nil {
		t.Fatalf("create today session: %v", err)
	}
	if _, err := sqlite.Timers().CreateSession(habitID, 120, 1, "2024-01-16"); err != nil {
		t.Fatalf("create explicit session: %v", err)
	}

	service := NewHabitService(a)
	defaultRows, err := service.ListSessions("", "", "")
	if err != nil {
		t.Fatalf("list default sessions: %v", err)
	}
	if len(defaultRows.([]storage.SessionRow)) != 1 || defaultRows.([]storage.SessionRow)[0].DurationSeconds != 60 {
		t.Fatalf("default date did not use persisted timezone: %+v", defaultRows)
	}
	explicitRows, err := service.ListSessions("2024-01-16", "", "")
	if err != nil {
		t.Fatalf("list explicit sessions: %v", err)
	}
	if len(explicitRows.([]storage.SessionRow)) != 1 || explicitRows.([]storage.SessionRow)[0].DurationSeconds != 120 {
		t.Fatalf("explicit date did not select caller date: %+v", explicitRows)
	}
	rangeRows, err := service.ListSessions("", "2024-01-16", "2024-01-17")
	if err != nil {
		t.Fatalf("list explicit date range: %v", err)
	}
	if len(rangeRows.([]storage.SessionRow)) != 1 || rangeRows.([]storage.SessionRow)[0].DurationSeconds != 120 {
		t.Fatalf("explicit date range did not select caller range: %+v", rangeRows)
	}
}
