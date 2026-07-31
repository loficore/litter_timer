// Package app — Wails v3 service bindings.
//
// This file hosts the four services that the Wails v3 Android
// frontend calls directly. Wails v3 binds exported methods via
// reflection — no annotations are required — so the method names
// and signatures here must match the WailsBindings generated in
// `cmd/server/assets/bindings/little-timer/internal/app/wailsbindings.ts`
// (which is driven by `assets/src/utils/wailsApiClient.ts`).
//
// Each service is a thin wrapper around the existing *App helpers
// plus a small amount of HTTP-handler-equivalent logic for the
// endpoints that don't have direct *App methods yet. No new
// business logic lives here.
//
// ponytail: no //wails: annotations, no factories, no interfaces —
// every service holds *App and forwards. The Wails reflection layer
// only needs the exported methods to exist on a public type.
package app

import (
	"context"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// modeKey mirrors `modeKey` in handlers/timer.go — renders the active
// clock mode as the stable string the JS client expects.
func modeKey(m domain.ModeEnum) string {
	if m == domain.CountdownMode {
		return "countdown"
	}
	return "stopwatch"
}

// -----------------------------------------------------------------------------
// TimerService — mirrors the timer endpoints of std_server.zig.
// -----------------------------------------------------------------------------

// TimerService exposes the timer state + control methods to the
// Wails v3 frontend. All exported methods are reflection-bound.
type TimerService struct {
	app *App
}

// NewTimerService builds a TimerService bound to the supplied App.
func NewTimerService(app *App) *TimerService { return &TimerService{app: app} }

// GetState mirrors handleGetState — returns the live clock state.
func (s *TimerService) GetState() (any, error) {
	a := s.app
	state := a.Clock.Update()
	tz := a.Settings.Config().Basic.Timezone

	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()

	return map[string]any{
		"time":           state.GetTimeInfo(),
		"elapsed":        state.GetElapsedSeconds(),
		"mode":           modeKey(state.GetMode()),
		"is_running":     !state.IsPaused(),
		"is_finished":    state.IsFinished(),
		"in_rest":        state.InRest(),
		"loop_remaining": state.GetLoopRemaining(),
		"loop_total":     state.GetLoopTotal(),
		"rest_remaining": state.GetRestRemainingTime(),
		"timezone":       tz,
		"habit_id":       habitID,
	}, nil
}

// StartTimer mirrors handleStart. All parameters are optional — the
// handler-equivalent defaults apply (stopwatch, 25 min work).
func (s *TimerService) StartTimer(habitID *int64, mode string, workDuration int64, restDuration int64, loopCount int64) (any, error) {
	a := s.app

	m := "stopwatch"
	if mode == "countdown" {
		m = "countdown"
	}
	work := workDuration
	if work == 0 {
		work = 25 * 60
	}

	a.Lock()
	defer a.Unlock()

	// Already-running branch — Zig keeps the same session and reports
	// the current habit id.
	if a.CurrentTimerSessionID != nil {
		row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
		if err == nil {
			if row.IsRunning && !row.IsFinished && !row.IsPaused {
				a.CurrentHabitID = habitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				return map[string]any{
					"status":     "already_running",
					"habit_id":   a.CurrentHabitID,
					"session_id": *a.CurrentTimerSessionID,
				}, nil
			}
			// Paused branch — resume.
			cs := a.Clock.Update()
			if (cs.IsPaused() && !cs.IsFinished()) || row.IsPaused {
				pausedTotal := row.PausedTotalSeconds
				now := time.Now().Unix()
				if row.PauseStartedAt != nil && now > *row.PauseStartedAt {
					pausedTotal += now - *row.PauseStartedAt
				}
				a.Clock.HandleEvent(domain.UserStartTimerEvent{})
				_ = a.SQLite.Timers().UpdateTimerSession(
					row.ID, row.ElapsedSeconds, row.RemainingSeconds,
					pausedTotal, nil, &now,
					true, false, false,
					row.CurrentRound, row.InRest,
				)
				a.CurrentHabitID = habitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				return map[string]any{
					"status":     "started",
					"habit_id":   a.CurrentHabitID,
					"session_id": row.ID,
				}, nil
			}
		}
		// Stale session — clean up.
		a.ResetTimerSession()
	}

	sessionID, err := a.CreateTimerSession(habitID, m, work, restDuration, loopCount)
	if err != nil {
		return nil, err
	}
	a.CurrentHabitID = habitID
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	return map[string]any{
		"status":     "started",
		"habit_id":   habitID,
		"session_id": sessionID,
	}, nil
}

// FinishTimer mirrors handleFinish — freezes the clock and emits a
// daily session row tied to the current habit.
func (s *TimerService) FinishTimer() (any, error) {
	a := s.app
	a.Lock()
	defer a.Unlock()

	habitID := a.CurrentHabitID
	sessionID := a.CurrentTimerSessionID

	elapsed, err := a.FinishTimerSession()
	if err != nil {
		// Fallback path — same shape as Zig: emit user_finish_timer,
		// compute elapsed from the clock state, and persist a daily
		// session if there was an active habit.
		a.Clock.HandleEvent(domain.UserFinishTimerEvent{})
		state := a.Clock.Update()
		elapsedSeconds := state.GetElapsedSeconds()
		if habitID != nil && elapsedSeconds > 0 {
			_, _ = a.SQLite.Timers().CreateSession(*habitID, elapsedSeconds, 1, domain.TodayString(a.Settings.Config().Basic.Timezone))
		}
		a.ResetTimerSession()
		return map[string]any{
			"status":          "finished",
			"elapsed_seconds": elapsedSeconds,
		}, nil
	}

	if habitID != nil && elapsed > 0 {
		_, _ = a.SQLite.Timers().CreateSession(*habitID, elapsed, 1, domain.TodayString(a.Settings.Config().Basic.Timezone))
	}
	a.ResetTimerSession()

	return map[string]any{
		"status":          "finished",
		"elapsed_seconds": elapsed,
		"session_id":      sessionID,
	}, nil
}

// GetProgress mirrors handleGetProgress — returns the live progress
// + mode + paused/finished flags.  Lazily loads progress if no
// current session is active.
func (s *TimerService) GetProgress() (any, error) {
	a := s.app

	a.RLock()
	hasSession := a.CurrentTimerSessionID != nil
	a.RUnlock()
	if !hasSession {
		a.LoadTimerProgress()
	}

	a.RLock()
	sessionID := a.CurrentTimerSessionID
	habitID := a.CurrentHabitID
	a.RUnlock()

	state := a.Clock.Update()
	return map[string]any{
		"session_id":        sessionID,
		"habit_id":          habitID,
		"mode":              modeKey(state.GetMode()),
		"is_running":        !state.IsPaused(),
		"is_paused":         state.IsPaused(),
		"is_finished":       state.IsFinished(),
		"elapsed_seconds":   state.GetElapsedSeconds(),
		"remaining_seconds": state.GetRemainingSeconds(),
		"in_rest":           state.InRest(),
	}, nil
}

// StartRest mirrors handleStartRest — switches the clock into a
// 5-minute countdown and starts it.
func (s *TimerService) StartRest() (any, error) {
	a := s.app
	const restSeconds uint64 = 5 * 60

	a.Clock.HandleEvent(domain.UserChangeConfigEvent{
		Config: domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown: domain.CountdownConfig{
				DurationSeconds:     restSeconds,
				Loop:                false,
				LoopCount:           0,
				LoopIntervalSeconds: 0,
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: 24 * 3600,
			},
		},
	})
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	return map[string]any{
		"status":       "rest_started",
		"rest_seconds": restSeconds,
	}, nil
}

// PauseTimer mirrors handlePause.
func (s *TimerService) PauseTimer() (any, error) {
	a := s.app
	a.Lock()
	defer a.Unlock()
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	return map[string]any{"status": "paused"}, nil
}

// ResetTimer mirrors handleReset.
func (s *TimerService) ResetTimer() (any, error) {
	a := s.app
	a.Lock()
	defer a.Unlock()
	a.ResetTimerSession()
	a.CurrentHabitID = nil
	a.Clock.HandleEvent(domain.UserResetTimerEvent{})
	return map[string]any{"status": "reset"}, nil
}

// -----------------------------------------------------------------------------
// HabitService — mirrors the habit / habit-set / session endpoints.
// -----------------------------------------------------------------------------

// HabitService exposes habit CRUD + session queries to the Wails v3
// frontend.
type HabitService struct {
	app *App
}

// NewHabitService builds a HabitService bound to the supplied App.
func NewHabitService(app *App) *HabitService { return &HabitService{app: app} }

// ListHabitSets mirrors handleHabitSetList.
func (s *HabitService) ListHabitSets() (any, error) {
	rows, err := s.app.SQLite.HabitSets().List(100, 0)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateHabitSet mirrors handleHabitSetCreate.
func (s *HabitService) CreateHabitSet(name string, description string, color string) (any, error) {
	if name == "" {
		return nil, errEmptyName
	}
	if color == "" {
		color = domain.DefaultColor
	}
	id, err := s.app.SQLite.HabitSets().Create(name, description, color)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
		"color":       color,
	}, nil
}

// UpdateHabitSet mirrors handleHabitSetUpdate.
func (s *HabitService) UpdateHabitSet(id int64, name string, description string, color string, wallpaper string) (any, error) {
	if name == "" {
		return nil, errEmptyName
	}
	if color == "" {
		color = domain.DefaultColor
	}
	if err := s.app.SQLite.HabitSets().Update(id, name, description, color, wallpaper); err != nil {
		return nil, err
	}
	return map[string]any{
		"id":          id,
		"name":        name,
		"description": description,
		"color":       color,
		"wallpaper":   wallpaper,
	}, nil
}

// DeleteHabitSet mirrors handleHabitSetDelete.
func (s *HabitService) DeleteHabitSet(id int64) (any, error) {
	if err := s.app.SQLite.HabitSets().Delete(id); err != nil {
		return nil, err
	}
	return map[string]any{"success": true}, nil
}

// ListHabits mirrors handleHabitList.  When setID is nil, returns
// every habit; otherwise scopes to the requested set.
func (s *HabitService) ListHabits(setID *int64) (any, error) {
	var (
		rows []storage.HabitRow
		err  error
	)
	const limit, offset = 100, 0
	if setID != nil {
		rows, err = s.app.SQLite.Habits().ListBySet(*setID, limit, offset)
	} else {
		rows, err = s.app.SQLite.Habits().List(limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateHabit mirrors handleHabitCreate.
func (s *HabitService) CreateHabit(setID int64, name string, goalSeconds int64, color string) (any, error) {
	if name == "" {
		return nil, errEmptyName
	}
	if goalSeconds == 0 {
		goalSeconds = domain.DefaultGoalSeconds
	}
	if color == "" {
		color = domain.DefaultColor
	}
	id, err := s.app.SQLite.Habits().Create(setID, name, goalSeconds, color)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":           id,
		"set_id":       setID,
		"name":         name,
		"goal_seconds": goalSeconds,
		"color":        color,
	}, nil
}

// UpdateHabit mirrors handleHabitUpdate.
func (s *HabitService) UpdateHabit(id int64, name string, goalSeconds int64, color string, wallpaper string) (any, error) {
	if name == "" {
		return nil, errEmptyName
	}
	if goalSeconds == 0 {
		goalSeconds = domain.DefaultGoalSeconds
	}
	if color == "" {
		color = domain.DefaultColor
	}
	if err := s.app.SQLite.Habits().Update(id, name, goalSeconds, color, wallpaper); err != nil {
		return nil, err
	}
	return map[string]any{
		"id":           id,
		"name":         name,
		"goal_seconds": goalSeconds,
		"color":        color,
		"wallpaper":    wallpaper,
	}, nil
}

// DeleteHabit mirrors handleHabitDelete.
func (s *HabitService) DeleteHabit(id int64) (any, error) {
	if err := s.app.SQLite.Habits().Delete(id); err != nil {
		return nil, err
	}
	return map[string]any{"success": true}, nil
}

// CreateSession mirrors handleSessionCreate.  Empty `date` defaults
// to today.
func (s *HabitService) CreateSession(habitID int64, durationSeconds int64, count int64, date string) (any, error) {
	if date == "" {
		date = domain.TodayString(s.app.Settings.Config().Basic.Timezone)
	}
	if count == 0 {
		count = 1
	}
	id, err := s.app.SQLite.Timers().CreateSession(habitID, durationSeconds, count, date)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":               id,
		"habit_id":         habitID,
		"duration_seconds": durationSeconds,
		"date":             date,
	}, nil
}

// ListSessions mirrors handleSessionList.  Three query shapes are
// supported (matching the Zig source): a single date, a date range,
// or no filter (= today).
func (s *HabitService) ListSessions(date string, startDate string, endDate string) (any, error) {
	var (
		rows []storage.SessionRow
		err  error
	)
	const limit, offset = 100, 0
	switch {
	case startDate != "" && endDate != "":
		rows, err = s.app.SQLite.Timers().ListSessionsByDateRange(startDate, endDate, limit, offset)
	case date != "":
		rows, err = s.app.SQLite.Timers().ListSessionsByDate(date, limit, offset)
	default:
		rows, err = s.app.SQLite.Timers().ListSessionsByDate(domain.TodayString(s.app.Settings.Config().Basic.Timezone), limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetHabitDetail mirrors handleHabitDetail — single habit with
// today's accumulated seconds + progress percent.
func (s *HabitService) GetHabitDetail(id int64, date string) (any, error) {
	a := s.app
	if date == "" {
		date = domain.TodayString(a.Settings.Config().Basic.Timezone)
	}
	row, err := a.SQLite.Habits().GetByID(id)
	if err != nil {
		return nil, err
	}
	todaySeconds, _ := a.SQLite.Timers().TodaySecondsForHabit(id, date)
	var progressPercent int64
	if row.GoalSeconds > 0 {
		progressPercent = (todaySeconds * 100) / row.GoalSeconds
	}
	return map[string]any{
		"id":               row.ID,
		"name":             row.Name,
		"goal_seconds":     row.GoalSeconds,
		"color":            row.Color,
		"today_seconds":    todaySeconds,
		"progress_percent": progressPercent,
	}, nil
}

// -----------------------------------------------------------------------------
// SettingsService — mirrors the /api/settings endpoints.
// -----------------------------------------------------------------------------

// SettingsService exposes the settings config to the Wails v3
// frontend.
type SettingsService struct {
	app *App
}

// NewSettingsService builds a SettingsService bound to the supplied App.
func NewSettingsService(app *App) *SettingsService { return &SettingsService{app: app} }

// GetSettings mirrors handleSettingsGet.
func (s *SettingsService) GetSettings() (any, error) {
	return s.app.Settings.Config(), nil
}

// UpdateSettings mirrors handleSettingsUpdate — applies a partial
// SettingsConfig via SettingsManager.HandleSettingsEvent.
func (s *SettingsService) UpdateSettings(json string) (any, error) {
	if err := s.app.Settings.HandleSettingsEvent(domain.SettingsChangeEvent{JSON: json}); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"status": "settings_updated"}, nil
}

// -----------------------------------------------------------------------------
// BackupService — mirrors the /api/backup/* + master-password endpoints.
// -----------------------------------------------------------------------------

// BackupService exposes the backup config + master-password
// lifecycle to the Wails v3 frontend.
type BackupService struct {
	app *App
}

// NewBackupService builds a BackupService bound to the supplied App.
func NewBackupService(app *App) *BackupService { return &BackupService{app: app} }

// GetBackupConfig mirrors handleBackupConfigGet — returns the
// persisted BackupConfig with secrets masked.
func (s *BackupService) GetBackupConfig() (any, error) {
	cfg := s.app.Settings.BackupConfig()
	return map[string]any{
		"enabled":              cfg.Enabled,
		"auto_backup":          cfg.AutoBackup,
		"auto_backup_interval": cfg.AutoBackupSecs,
		"target_type":          cfg.TargetType.String(),
		"local_path":           cfg.LocalPath,
		"webdav_url":           cfg.WebDAVURL,
		"webdav_username":      cfg.WebDAVUsername,
		"webdav_password":      maskSecret(cfg.WebDAVPassword),
		"s3_endpoint":          cfg.S3Endpoint,
		"s3_bucket":            cfg.S3Bucket,
		"s3_region":            cfg.S3Region,
		"s3_access_key":        maskSecret(cfg.S3AccessKey),
		"s3_secret_key":        maskSecret(cfg.S3SecretKey),
		"s3_path_prefix":       cfg.S3PathPrefix,
	}, nil
}

// UpdateBackupConfig mirrors handleBackupConfigUpdate — applies a
// JSON BackupConfig update via SettingsManager.UpdateBackupConfigFromJSON.
func (s *BackupService) UpdateBackupConfig(json string) (any, error) {
	if err := s.app.Settings.UpdateBackupConfigFromJSON(json); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	// Config is persisted; rebuild the manager from it.  A rebuild
	// failure must not fail the call — the config is saved, the
	// manager will be rebuilt on the next config change or boot.
	if err := s.app.RebuildBackup(context.Background()); err != nil {
		log.Error("UpdateBackupConfig: rebuild failed", "error", err.Error())
	}
	return map[string]any{"success": true}, nil
}

// CreateBackup mirrors handleBackupCreate.
func (s *BackupService) CreateBackup() (any, error) {
	a := s.app
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	cfg := a.Settings.BackupConfig()
	if !cfg.Enabled {
		return map[string]any{"success": false, "error": "backup not enabled"}, nil
	}
	name, err := bm.CreateBackup()
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"success": true, "backup_path": name}, nil
}

// RestoreBackup mirrors handleBackupRestore.
func (s *BackupService) RestoreBackup(name string) (any, error) {
	a := s.app
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if err := bm.RestoreFromBackup(name); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"success": true}, nil
}

// DeleteBackup mirrors handleBackupDeleteByName — DELETE on the
// `/api/backup/:name` route (the simpler form, not the `/delete/`
// prefix).
func (s *BackupService) DeleteBackup(name string) (any, error) {
	a := s.app
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if err := bm.DeleteBackup(name); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"success": true}, nil
}

// VerifyBackup mirrors handleBackupVerify — exercises the
// configured adapter's connection.
func (s *BackupService) VerifyBackup() (any, error) {
	a := s.app
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if !a.Settings.BackupConfig().Enabled {
		return map[string]any{"success": false, "error": "backup not enabled"}, nil
	}
	if err := bm.TestConnection(); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"success": true}, nil
}

// ListBackups mirrors handleBackupList.
func (s *BackupService) ListBackups() (any, error) {
	a := s.app
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": true, "backups": []any{}}, nil
	}
	items, err := bm.ListBackups()
	if err != nil {
		return map[string]any{"success": true, "backups": []any{}}, nil
	}
	return map[string]any{"success": true, "backups": items}, nil
}

// GetMasterPasswordStatus mirrors handleMasterPasswordGet.
func (s *BackupService) GetMasterPasswordStatus() (any, error) {
	return s.app.GetMasterPasswordStatus(), nil
}

// SetMasterPassword mirrors handleMasterPasswordSet — enforces the
// 4-character minimum that matches the Zig validator.
func (s *BackupService) SetMasterPassword(password string) (any, error) {
	if password == "" {
		return map[string]any{"success": false, "error": "missing password"}, nil
	}
	if len(password) < 4 {
		return map[string]any{"success": false, "error": "password too short (minimum 4 characters)"}, nil
	}
	if err := s.app.SetMasterPassword(password); err != nil {
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	return map[string]any{"success": true}, nil
}

// UnlockCredentials mirrors handleBackupUnlock.
func (s *BackupService) UnlockCredentials(password string) (any, error) {
	res := s.app.UnlockCredentials(password)
	return map[string]any{
		"success":      res.Success,
		"locked_until": res.LockedUntil,
	}, nil
}

// LockCredentials mirrors handleBackupLock.
func (s *BackupService) LockCredentials() (any, error) {
	s.app.LockCredentials()
	return map[string]any{"success": true}, nil
}

// -----------------------------------------------------------------------------
// Shared helpers — duplicated from handlers/ to keep this file self-contained.
// -----------------------------------------------------------------------------

// maskSecret returns "******" for non-empty secrets, "" otherwise.
// Mirrors `mask` in handlers/backup.go.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

// errEmptyName is returned by CreateHabitSet / CreateHabit / Update*
// when the caller omits the name field.  Mirrors the same validator
// in handlers/habits.go.
var errEmptyName = &wailsError{code: "missing_name", message: "missing name"}

// wailsError is the tiny error type used by the Wails service layer.
// Kept distinct from httpError (in app.go) so the two layers stay
// decoupled.
type wailsError struct {
	code, message string
}

func (e *wailsError) Error() string { return e.message }
