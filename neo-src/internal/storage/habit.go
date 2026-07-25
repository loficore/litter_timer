// Package storage — habit / session / timer-session CRUD.
//
// Port of `src/storage/habit_crud.zig` (little_timer).  The Zig file exposes
// a single HabitCrudManager that owns every habit-related operation; this Go
// port splits them into three separate types so each table has a focused API
// surface and tests can target them in isolation:
//
//   - HabitSetCrud      — operates on `habit_sets`
//   - HabitCrud         — operates on `habits`
//   - TimerSessionCrud  — operates on `sessions` + `timer_sessions`
//
// Memory ownership: Zig uses an allocator for returned strings; Go strings
// are immutable, so there's no equivalent of freeHabitSets / freeHabits —
// callers just let GC reclaim them.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"little-timer/internal/domain"
)

// HabitError mirrors `pub const HabitError = error{...}` in habit_crud.zig.
type HabitError string

const (
	ErrHabitInsertFailed HabitError = "habit insert failed"
	ErrHabitUpdateFailed HabitError = "habit update failed"
	ErrHabitDeleteFailed HabitError = "habit delete failed"
	ErrHabitQueryFailed  HabitError = "habit query failed"
	ErrHabitNotFound     HabitError = "habit not found"
)

func (e HabitError) Error() string { return string(e) }

// -----------------------------------------------------------------------------
// Row types — Go ports of HabitSetRow / HabitRow / SessionRow / TimerSessionRow.
// -----------------------------------------------------------------------------

// HabitSetRow mirrors `pub const HabitSetRow = struct {...}`.
type HabitSetRow struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Wallpaper   string `json:"wallpaper"`
}

// HabitRow mirrors `pub const HabitRow = struct {...}`.  Wallpaper is
// dereferenced via COALESCE in queries so an empty string is returned for
// NULL.
type HabitRow struct {
	ID          int64     `json:"id"`
	SetID       int64     `json:"set_id"`
	Name        string    `json:"name"`
	GoalSeconds int64     `json:"goal_seconds"`
	Color       string    `json:"color"`
	Wallpaper   string    `json:"wallpaper"`
	CreatedAt   time.Time `json:"created_at"`
}

// SessionRow mirrors `pub const SessionRow = struct {...}`.
type SessionRow struct {
	ID              int64  `json:"id"`
	HabitID         int64  `json:"habit_id"`
	DurationSeconds int64  `json:"duration_seconds"`
	Count           int64  `json:"count"`
	StartedAt       string `json:"started_at"`
	Date            string `json:"date"`
}

// TimerSessionRow mirrors `pub const TimerSessionRow = struct {...}`.  The
// nullable columns map to `*int64` in Go (sql.NullInt64 is fine but pointer
// scans stay closer to the Zig `?i64` shape).
type TimerSessionRow struct {
	ID                 int64  `json:"id"`
	HabitID            *int64 `json:"habit_id"`
	Mode               string `json:"mode"`
	StartedAt          int64  `json:"started_at"`
	UpdatedAt          int64  `json:"updated_at"`
	IsRunning          bool   `json:"is_running"`
	IsFinished         bool   `json:"is_finished"`
	IsPaused           bool   `json:"is_paused"`
	ElapsedSeconds     int64  `json:"elapsed_seconds"`
	PausedTotalSeconds int64  `json:"paused_total_seconds"`
	PauseStartedAt     *int64 `json:"pause_started_at"`
	LastSyncedAt       *int64 `json:"last_synced_at"`
	RemainingSeconds   *int64 `json:"remaining_seconds"`
	WorkDuration       int64  `json:"work_duration"`
	RestDuration       int64  `json:"rest_duration"`
	LoopCount          int64  `json:"loop_count"`
	CurrentRound       int64  `json:"current_round"`
	InRest             bool   `json:"in_rest"`
}

// -----------------------------------------------------------------------------
// HabitSetCrud — `habit_sets` table.
// -----------------------------------------------------------------------------

// HabitSetCrud is the Go split of HabitCrudManager's habit-set methods.
type HabitSetCrud struct {
	db *sql.DB
}

// NewHabitSetCrud returns an empty HabitSetCrud.  Mirrors
// `HabitCrudManager.init(allocator, null)` (one manager to rule them all).
func NewHabitSetCrud() *HabitSetCrud { return &HabitSetCrud{} }

// SetDB attaches the *sql.DB.  Mirrors `habit_manager.db = self.db`.
func (h *HabitSetCrud) SetDB(db *sql.DB) { h.db = db }

// Create inserts a new habit_set and returns its rowid.  Mirrors the Zig
// `pub fn createHabitSet(name, description, color)`.
func (h *HabitSetCrud) Create(name, description, color string) (int64, error) {
	if h.db == nil {
		return 0, ErrHabitQueryFailed
	}
	res, err := h.db.Exec(
		`INSERT INTO habit_sets (name, description, color) VALUES (?, ?, ?);`,
		name, description, color,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// List returns every habit_set ordered by created_at DESC.  Mirrors the
// Zig `pub fn getAllHabitSets`.
func (h *HabitSetCrud) List(limit, offset int) ([]HabitSetRow, error) {
	if h.db == nil {
		return nil, ErrHabitQueryFailed
	}
	rows, err := h.db.Query(
		`SELECT id, name, description, color, COALESCE(wallpaper, '') FROM habit_sets ORDER BY created_at DESC LIMIT ? OFFSET ?;`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	out := []HabitSetRow{}
	for rows.Next() {
		var r HabitSetRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Color, &r.Wallpaper); err != nil {
			return nil, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Update overwrites the editable columns for a habit_set.
func (h *HabitSetCrud) Update(id int64, name, description, color, wallpaper string) error {
	if h.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := h.db.Exec(
		`UPDATE habit_sets SET name = ?, description = ?, color = ?, wallpaper = ? WHERE id = ?;`,
		name, description, color, wallpaper, id,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitUpdateFailed, err)
	}
	return nil
}

// Delete removes a habit_set (cascading to habits + sessions per FK).
func (h *HabitSetCrud) Delete(id int64) error {
	if h.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := h.db.Exec(`DELETE FROM habit_sets WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitDeleteFailed, err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// HabitCrud — `habits` table.
// -----------------------------------------------------------------------------

// HabitCrud is the Go split of HabitCrudManager's habit methods.
type HabitCrud struct {
	db *sql.DB
}

// NewHabitCrud returns an empty HabitCrud.
func NewHabitCrud() *HabitCrud { return &HabitCrud{} }

// SetDB attaches the *sql.DB.
func (h *HabitCrud) SetDB(db *sql.DB) { h.db = db }

// Create inserts a new habit and returns its rowid.
func (h *HabitCrud) Create(setID int64, name string, goalSeconds int64, color string) (int64, error) {
	if h.db == nil {
		return 0, ErrHabitQueryFailed
	}
	res, err := h.db.Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color) VALUES (?, ?, ?, ?);`,
		setID, name, goalSeconds, color,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// NameExistsInSet reports whether a habit name is already used in a set.
// excludeID is omitted by passing 0 because habit IDs start at 1.
func (h *HabitCrud) NameExistsInSet(setID int64, name string, excludeID *int64) (bool, error) {
	if h.db == nil {
		return false, ErrHabitQueryFailed
	}

	excludedID := int64(0)
	if excludeID != nil {
		excludedID = *excludeID
	}

	var count int64
	err := h.db.QueryRow(
		`SELECT COUNT(*) FROM habits WHERE set_id = ? AND name = ? AND id != ?;`,
		setID, name, excludedID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	return count > 0, nil
}

// List returns every habit ordered by created_at DESC.  Mirrors Zig
// `getAllHabits`.
func (h *HabitCrud) List(limit, offset int) ([]HabitRow, error) {
	if h.db == nil {
		return nil, ErrHabitQueryFailed
	}
	rows, err := h.db.Query(
		`SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits ORDER BY created_at DESC LIMIT ? OFFSET ?;`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	out := []HabitRow{}
	for rows.Next() {
		var r HabitRow
		if err := rows.Scan(&r.ID, &r.SetID, &r.Name, &r.GoalSeconds, &r.Color, &r.Wallpaper); err != nil {
			return nil, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListBySet returns habits scoped to a single set.  Mirrors `getHabitsBySet`.
func (h *HabitCrud) ListBySet(setID int64, limit, offset int) ([]HabitRow, error) {
	if h.db == nil {
		return nil, ErrHabitQueryFailed
	}
	rows, err := h.db.Query(
		`SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits WHERE set_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?;`,
		setID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	out := []HabitRow{}
	for rows.Next() {
		var r HabitRow
		if err := rows.Scan(&r.ID, &r.SetID, &r.Name, &r.GoalSeconds, &r.Color, &r.Wallpaper); err != nil {
			return nil, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetByID returns a single habit, or ErrHabitNotFound when no row matches.
func (h *HabitCrud) GetByID(id int64) (HabitRow, error) {
	if h.db == nil {
		return HabitRow{}, ErrHabitQueryFailed
	}
	var r HabitRow
	err := h.db.QueryRow(
		`SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits WHERE id = ?;`,
		id,
	).Scan(&r.ID, &r.SetID, &r.Name, &r.GoalSeconds, &r.Color, &r.Wallpaper)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HabitRow{}, ErrHabitNotFound
		}
		return HabitRow{}, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	return r, nil
}

// Update overwrites the editable columns for a habit.
func (h *HabitCrud) Update(id int64, name string, goalSeconds int64, color, wallpaper string) error {
	if h.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := h.db.Exec(
		`UPDATE habits SET name = ?, goal_seconds = ?, color = ?, wallpaper = ? WHERE id = ?;`,
		name, goalSeconds, color, wallpaper, id,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitUpdateFailed, err)
	}
	return nil
}

// Delete removes a habit (cascading to its sessions).
func (h *HabitCrud) Delete(id int64) error {
	if h.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := h.db.Exec(`DELETE FROM habits WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitDeleteFailed, err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// TimerSessionCrud — `sessions` + `timer_sessions` tables.
// -----------------------------------------------------------------------------

// TimerSessionCrud owns both the focus-session table (`sessions`) and the
// live timer-state table (`timer_sessions`).
type TimerSessionCrud struct {
	db *sql.DB
}

// NewTimerSessionCrud returns an empty TimerSessionCrud.
func NewTimerSessionCrud() *TimerSessionCrud { return &TimerSessionCrud{} }

// SetDB attaches the *sql.DB.
func (t *TimerSessionCrud) SetDB(db *sql.DB) { t.db = db }

// CreateSession inserts a focus-session row and returns its rowid.  Mirrors
// the Zig `pub fn createSession`.
func (t *TimerSessionCrud) CreateSession(habitID, durationSeconds, count int64, date string) (int64, error) {
	if t.db == nil {
		return 0, ErrHabitQueryFailed
	}
	res, err := t.db.Exec(
		`INSERT INTO sessions (habit_id, duration_seconds, count, date) VALUES (?, ?, ?, ?);`,
		habitID, durationSeconds, count, date,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// ListSessionsByDate returns sessions for a single date, ordered by
// started_at DESC.  Mirrors `getSessionsByDate`.
func (t *TimerSessionCrud) ListSessionsByDate(date string, limit, offset int) ([]SessionRow, error) {
	if t.db == nil {
		return nil, ErrHabitQueryFailed
	}
	rows, err := t.db.Query(
		`SELECT id, habit_id, duration_seconds, count, started_at, date FROM sessions WHERE date = ? ORDER BY started_at DESC LIMIT ? OFFSET ?;`,
		date, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	out := []SessionRow{}
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.HabitID, &r.DurationSeconds, &r.Count, &r.StartedAt, &r.Date); err != nil {
			return nil, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListSessionsByDateRange returns sessions in a half-open date window.
// Mirrors `getSessionsByDateRange`.
func (t *TimerSessionCrud) ListSessionsByDateRange(start, end string, limit, offset int) ([]SessionRow, error) {
	if t.db == nil {
		return nil, ErrHabitQueryFailed
	}
	rows, err := t.db.Query(
		`SELECT id, habit_id, duration_seconds, count, started_at, date FROM sessions WHERE date >= ? AND date <= ? ORDER BY date DESC LIMIT ? OFFSET ?;`,
		start, end, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	out := []SessionRow{}
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.HabitID, &r.DurationSeconds, &r.Count, &r.StartedAt, &r.Date); err != nil {
			return nil, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TodaySecondsForHabit sums duration_seconds for one habit on one date.
// Mirrors `getHabitTodaySeconds`.
func (t *TimerSessionCrud) TodaySecondsForHabit(habitID int64, date string) (int64, error) {
	if t.db == nil {
		return 0, ErrHabitQueryFailed
	}
	var total sql.NullInt64
	if err := t.db.QueryRow(
		`SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE habit_id = ? AND date = ?;`,
		habitID, date,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

// CreateTimerSession inserts a new timer_sessions row and returns its rowid.
//
// Mirrors the Zig `pub fn createTimerSession(habit_id, mode, work_duration,
// rest_duration, loop_count)`.  The Zig code uses `std.time.timestamp()` to
// stamp started_at / updated_at; Go uses time.Now().Unix().
func (t *TimerSessionCrud) CreateTimerSession(habitID *int64, mode string, workDuration, restDuration, loopCount int64) (int64, error) {
	if t.db == nil {
		return 0, ErrHabitQueryFailed
	}
	now := time.Now().Unix()

	const insertSQL = `INSERT INTO timer_sessions
		(habit_id, mode, started_at, updated_at, is_running, is_finished, is_paused, elapsed_seconds, paused_total_seconds, pause_started_at, last_synced_at, remaining_seconds, work_duration, rest_duration, loop_count, current_round, in_rest)
		VALUES (?, ?, ?, ?, 1, 0, 0, 0, 0, NULL, ?, ?, ?, ?, ?, 0, 0);`

	res, err := t.db.Exec(insertSQL,
		habitID, mode, now, now, now,
		workDuration, workDuration, restDuration, loopCount,
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrHabitInsertFailed, err)
	}
	return res.LastInsertId()
}

// UpdateTimerSession patches the live state of a timer_sessions row.
// Mirrors the Zig `pub fn updateTimerSession`.
func (t *TimerSessionCrud) UpdateTimerSession(
	sessionID int64,
	elapsedSeconds int64,
	remainingSeconds *int64,
	pausedTotalSeconds int64,
	pauseStartedAt *int64,
	lastSyncedAt *int64,
	isRunning, isPaused, isFinished bool,
	currentRound int64,
	inRest bool,
) error {
	if t.db == nil {
		return ErrHabitQueryFailed
	}
	now := time.Now().Unix()

	const updateSQL = `UPDATE timer_sessions
		SET updated_at = ?, elapsed_seconds = ?, remaining_seconds = ?, paused_total_seconds = ?, pause_started_at = ?, last_synced_at = ?, is_running = ?, is_paused = ?, is_finished = ?, current_round = ?, in_rest = ?
		WHERE id = ?;`

	_, err := t.db.Exec(updateSQL,
		now, elapsedSeconds, remainingSeconds, pausedTotalSeconds,
		pauseStartedAt, lastSyncedAt,
		domain.BoolToInt(isRunning), domain.BoolToInt(isPaused), domain.BoolToInt(isFinished),
		currentRound, domain.BoolToInt(inRest),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitUpdateFailed, err)
	}
	return nil
}

// GetActiveTimerSession returns the most-recently-updated non-finished
// timer_session, or ErrHabitNotFound when none exists.  Mirrors
// `getActiveTimerSession`.
func (t *TimerSessionCrud) GetActiveTimerSession() (TimerSessionRow, error) {
	return t.getTimerSession(`
		WHERE is_finished = 0
		ORDER BY updated_at DESC
		LIMIT 1;`, nil)
}

// GetTimerSessionByID returns a single row by id.  Mirrors
// `getTimerSessionById`.
func (t *TimerSessionCrud) GetTimerSessionByID(sessionID int64) (TimerSessionRow, error) {
	return t.getTimerSession(`WHERE id = ? LIMIT 1;`, &sessionID)
}

// getTimerSession is the shared SELECT + scan used by GetActive/GetByID.
func (t *TimerSessionCrud) getTimerSession(whereClause string, arg any) (TimerSessionRow, error) {
	if t.db == nil {
		return TimerSessionRow{}, ErrHabitQueryFailed
	}
	const selectCols = `id, habit_id, mode, started_at, updated_at, is_running, is_finished, is_paused,
		elapsed_seconds, paused_total_seconds, pause_started_at, last_synced_at, remaining_seconds, work_duration, rest_duration, loop_count, current_round, in_rest`
	q := `SELECT ` + selectCols + ` FROM timer_sessions ` + whereClause

	var (
		row     TimerSessionRow
		habit   sql.NullInt64
		isRun   int64
		isFin   int64
		isPause int64
		inRest  int64
	)
	var rows *sql.Rows
	var err error
	if arg == nil {
		rows, err = t.db.Query(q)
	} else {
		rows, err = t.db.Query(q, arg)
	}
	if err != nil {
		return TimerSessionRow{}, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return TimerSessionRow{}, fmt.Errorf("%w: %w", ErrHabitQueryFailed, err)
		}
		return TimerSessionRow{}, ErrHabitNotFound
	}

	err = rows.Scan(
		&row.ID, &habit, &row.Mode, &row.StartedAt, &row.UpdatedAt,
		&isRun, &isFin, &isPause,
		&row.ElapsedSeconds, &row.PausedTotalSeconds,
		&row.PauseStartedAt, &row.LastSyncedAt, &row.RemainingSeconds,
		&row.WorkDuration, &row.RestDuration, &row.LoopCount,
		&row.CurrentRound, &inRest,
	)
	if err != nil {
		return TimerSessionRow{}, fmt.Errorf("%w: scan: %w", ErrHabitQueryFailed, err)
	}
	if habit.Valid {
		v := habit.Int64
		row.HabitID = &v
	}
	row.IsRunning = isRun != 0
	row.IsFinished = isFin != 0
	row.IsPaused = isPause != 0
	row.InRest = inRest != 0
	return row, nil
}

// DeleteTimerSession removes a timer_session row.
func (t *TimerSessionCrud) DeleteTimerSession(sessionID int64) error {
	if t.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := t.db.Exec(`DELETE FROM timer_sessions WHERE id = ?;`, sessionID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitDeleteFailed, err)
	}
	return nil
}

// DeleteSession removes a session row.
func (t *TimerSessionCrud) DeleteSession(sessionID int64) error {
	if t.db == nil {
		return ErrHabitQueryFailed
	}
	res, err := t.db.Exec(`DELETE FROM sessions WHERE id = ?;`, sessionID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitDeleteFailed, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrHabitNotFound
	}
	return nil
}

// FinishTimerSession marks a timer_session as finished and stopped.
func (t *TimerSessionCrud) FinishTimerSession(sessionID int64) error {
	if t.db == nil {
		return ErrHabitQueryFailed
	}
	_, err := t.db.Exec(
		`UPDATE timer_sessions
		 SET updated_at = ?, is_running = 0, is_finished = 1, is_paused = 0
		 WHERE id = ?;`,
		time.Now().Unix(), sessionID,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHabitUpdateFailed, err)
	}
	return nil
}

// GetHabitStats returns aggregated stats for a habit over the current week.
func (t *TimerSessionCrud) GetHabitStats(habitID int64) (map[string]interface{}, error) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, time.UTC)
	weekStartStr := weekStart.Format("2006-01-02")

	// Query sessions for this habit since week start
	rows, err := t.db.Query(`
		SELECT date, SUM(duration_seconds)
		FROM sessions
		WHERE habit_id = ? AND date >= ?
		GROUP BY date
		ORDER BY date ASC`,
		habitID, weekStartStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Calculate totals and build weekly breakdown
	totalSeconds := int64(0)
	totalSessions := 0
	weeklyBreakdown := make(map[string]int64)
	daysOfWeek := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

	for rows.Next() {
		var date string
		var seconds int64
		if err := rows.Scan(&date, &seconds); err != nil {
			return nil, err
		}
		totalSeconds += seconds
		totalSessions++
		// Parse date to get day of week
		if t, err := time.Parse("2006-01-02", date); err == nil {
			dow := int(t.Weekday())
			if dow == 0 {
				dow = 7
			}
			weeklyBreakdown[daysOfWeek[dow-1]] = seconds
		}
	}

	return map[string]interface{}{
		"habit_id":            habitID,
		"total_seconds_week":  totalSeconds,
		"total_sessions_week": totalSessions,
		"weekly_breakdown":    weeklyBreakdown,
	}, nil
}
