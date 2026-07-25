// Package handlers — Timer endpoints.
//
// File `timer.go` is the Go port of the timer-related handlers in
// std_server.zig.  Each handler pulls the *App from the Gin context
// (set by the auth middleware) and mirrors the Zig response shapes
// byte-for-byte.
//
// Endpoints (paths match Zig exactly):
//
//	GET  /api/state             → handleGetState
//	GET  /api/timer/progress    → handleGetProgress
//	POST /api/start             → handleStart
//	POST /api/pause             → handlePause
//	POST /api/reset             → handleReset
//	POST /api/finish            → handleFinish
//	POST /api/mode              → handleModeSwitch
//	POST /api/timer/rest        → handleStartRest
//	GET  /api/timer/config      → handleGetConfig
//	POST /api/timer/config      → handleUpdateConfig
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
)

// ponytail: package-level constants for hard-coded timer durations
const (
	DefaultWorkDuration = 25 * 60 // seconds
	DefaultRestDuration = 5 * 60  // seconds
)

// appFromCtx pulls the *App placed by the auth middleware.  Panics if
// the middleware was not installed — that's a programmer error, not a
// runtime condition to handle.
func appFromCtx(c *gin.Context) *app.App {
	return c.MustGet("app").(*app.App)
}

// buildStateResponse mirrors `buildStateJson` in std_server.zig.
// Returns the JSON object as a map[string]any so the field order is
// stable across Go versions (Gin's JSON encoder sorts keys).
func buildStateResponse(state *domain.ClockState, modeKey string, timezone int8, habitID *int64) gin.H {
	out := gin.H{
		"time":           state.GetTimeInfo(),
		"elapsed":        state.GetElapsedSeconds(),
		"mode":           modeKey,
		"is_running":     !state.IsPaused(),
		"is_finished":    state.IsFinished(),
		"in_rest":        state.InRest(),
		"loop_remaining": state.GetLoopRemaining(),
		"loop_total":     state.GetLoopTotal(),
		"rest_remaining": state.GetRestRemainingTime(),
		"timezone":       timezone,
	}
	if habitID != nil {
		out["habit_id"] = *habitID
	}
	return out
}

func modeKey(m domain.ModeEnum) string {
	if m == domain.CountdownMode {
		return "countdown"
	}
	return "stopwatch"
}

// -----------------------------------------------------------------------------
// GET /api/state
// -----------------------------------------------------------------------------

// handleGetState mirrors `handleGetState` — returns the current clock
// state as JSON.
func TimerState(c *gin.Context) {
	a := appFromCtx(c)
	state := a.Clock.Update()
	tz := a.Settings.Config().Basic.Timezone

	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()

	c.JSON(http.StatusOK, buildStateResponse(state, modeKey(state.GetMode()), tz, habitID))
}

// -----------------------------------------------------------------------------
// GET /api/timer/progress
// -----------------------------------------------------------------------------

// handleGetProgress mirrors `handleGetProgress` — returns the live
// progress + mode + paused/finished flags.  Lazily loads progress if
// no current session is active.
func TimerProgress(c *gin.Context) {
	a := appFromCtx(c)

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
	c.JSON(http.StatusOK, gin.H{
		"session_id":        sessionID,
		"habit_id":          habitID,
		"mode":              modeKey(state.GetMode()),
		"is_running":        !state.IsPaused(),
		"is_paused":         state.IsPaused(),
		"is_finished":       state.IsFinished(),
		"elapsed_seconds":   state.GetElapsedSeconds(),
		"remaining_seconds": state.GetRemainingSeconds(),
		"in_rest":           state.InRest(),
	})
}

// -----------------------------------------------------------------------------
// POST /api/start
// -----------------------------------------------------------------------------

// startRequest is the JSON body of `POST /api/start`.  All fields are
// optional — defaults mirror the Zig source.
type startRequest struct {
	HabitID      *int64 `json:"habit_id,omitempty"`
	Mode         string `json:"mode,omitempty"`
	WorkDuration int64  `json:"work_duration,omitempty"`
	RestDuration int64  `json:"rest_duration,omitempty"`
	LoopCount    int64  `json:"loop_count,omitempty"`
}

// handleStart mirrors `handleStart`.  Body: {habit_id?, mode?, work_duration?, rest_duration?, loop_count?}.
func TimerStart(c *gin.Context) {
	a := appFromCtx(c)

	var req startRequest
	_ = c.ShouldBindJSON(&req) // body is optional — defaults below.

	mode := "stopwatch"
	if req.Mode == "countdown" {
		mode = "countdown"
	}
	work := req.WorkDuration
	if work == 0 {
		work = DefaultWorkDuration
	}
	rest := req.RestDuration
	loop := req.LoopCount

	a.Lock()
	defer a.Unlock()

	// Already-running branch — Zig keeps the same session and reports
	// the current habit id.
	if a.CurrentTimerSessionID != nil {
		// Look up the live row.
		row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
		if err == nil {
			if row.IsRunning && !row.IsFinished && !row.IsPaused {
				a.CurrentHabitID = req.HabitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				c.JSON(http.StatusOK, gin.H{
					"status":     "already_running",
					"habit_id":   a.CurrentHabitID,
					"session_id": *a.CurrentTimerSessionID,
				})
				return
			}
			// Paused branch — resume.
			state := a.Clock.Update()
			if state.IsPaused() && (!state.IsFinished() || row.IsPaused) {
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
				a.CurrentHabitID = req.HabitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				c.JSON(http.StatusOK, gin.H{
					"status":     "started",
					"habit_id":   a.CurrentHabitID,
					"session_id": row.ID,
				})
				return
			}
		}
		// Stale session — clean up.
		a.ResetTimerSession()
	}

	sessionID, err := a.CreateTimerSession(req.HabitID, mode, work, rest, loop)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create timer session"})
		return
	}
	a.CurrentHabitID = req.HabitID
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	c.JSON(http.StatusOK, gin.H{
		"status":     "started",
		"habit_id":   req.HabitID,
		"session_id": sessionID,
	})
}

// -----------------------------------------------------------------------------
// POST /api/pause
// -----------------------------------------------------------------------------

// handlePause mirrors `handlePause`.
func TimerPause(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// -----------------------------------------------------------------------------
// POST /api/reset
// -----------------------------------------------------------------------------

// handleReset mirrors `handleReset`.
func TimerReset(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.ResetTimerSession()
	a.CurrentHabitID = nil
	a.Clock.HandleEvent(domain.UserResetTimerEvent{})
	c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

// -----------------------------------------------------------------------------
// POST /api/finish
// -----------------------------------------------------------------------------

// handleFinish mirrors `handleFinish`.  On success creates a daily
// session row tied to the current habit so habit stats stay in sync.
func TimerFinish(c *gin.Context) {
	a := appFromCtx(c)
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
			if _, err := a.SQLite.Timers().CreateSession(*habitID, elapsedSeconds, 1, domain.TodayString(a.Settings.Config().Basic.Timezone)); err != nil {
				log.Printf("failed to create fallback session: %v", err)
			}
		}
		a.ResetTimerSession()
		c.JSON(http.StatusOK, gin.H{
			"status":          "finished",
			"elapsed_seconds": elapsedSeconds,
		})
		return
	}

	if habitID != nil && elapsed > 0 {
		if _, err := a.SQLite.Timers().CreateSession(*habitID, elapsed, 1, domain.TodayString(a.Settings.Config().Basic.Timezone)); err != nil {
			log.Printf("failed to create session at end: %v", err)
		}
	}
	a.ResetTimerSession()

	c.JSON(http.StatusOK, gin.H{
		"status":          "finished",
		"elapsed_seconds": elapsed,
		"session_id":      sessionID,
	})
}

// -----------------------------------------------------------------------------
// POST /api/mode
// -----------------------------------------------------------------------------

// handleModeSwitch mirrors `handleModeChange`.  Body is a JSON object
// `{"mode":"countdown"|"stopwatch"}`.
func TimerMode(c *gin.Context) {
	a := appFromCtx(c)

	var body struct {
		Mode string `json:"mode"`
	}
	// Body is the raw mode string in Zig (no JSON object).  We accept
	// either form: a JSON object with "mode", or a bare string.  The
	// browser client always sends JSON, so this is the common path.
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		_ = json.Unmarshal(raw, &body)
		trimmed = strings.TrimSpace(body.Mode)
	}

	var newMode domain.ModeEnum
	switch trimmed {
	case "countdown":
		newMode = domain.CountdownMode
	case "stopwatch":
		newMode = domain.StopwatchMode
	default:
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	a.Clock.HandleEvent(domain.UserChangeModeEvent{Mode: newMode})
	c.JSON(http.StatusOK, gin.H{
		"status":   "mode_changed",
		"new_mode": trimmed,
	})
}

// -----------------------------------------------------------------------------
// POST /api/timer/rest
// -----------------------------------------------------------------------------

// handleStartRest mirrors `handleStartRest` — switches the clock into
// a 5-minute countdown and starts it.
func TimerStartRest(c *gin.Context) {
	a := appFromCtx(c)
	const restSeconds uint64 = DefaultRestDuration

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
	c.JSON(http.StatusOK, gin.H{
		"status":       "rest_started",
		"rest_seconds": restSeconds,
	})
}

// -----------------------------------------------------------------------------
// GET /api/timer/config  /  POST /api/timer/config
// -----------------------------------------------------------------------------

// handleConfig returns the currently-active ClockTaskConfig as JSON.
// Mirrors the Zig GET branch — there's no dedicated handler in std_server.zig,
// but the route is referenced in task-list comments, so we wire it through
// to the active state.
func TimerConfig(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.BuildClockConfig()
	c.JSON(http.StatusOK, gin.H{
		"default_mode": cfg.DefaultMode.String(),
		"countdown": gin.H{
			"duration_seconds":      cfg.Countdown.DurationSeconds,
			"loop":                  cfg.Countdown.Loop,
			"loop_count":            cfg.Countdown.LoopCount,
			"loop_interval_seconds": cfg.Countdown.LoopIntervalSeconds,
		},
		"stopwatch": gin.H{
			"max_seconds": cfg.Stopwatch.MaxSeconds,
		},
	})
}

// handleUpdateConfig applies a partial config update.  Body mirrors the
// ClockTaskConfig shape with the same JSON field names as `GET`.
func TimerUpdateConfig(c *gin.Context) {
	a := appFromCtx(c)
	var req domain.ClockTaskConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	// Reuse the same UserChangeConfigEvent path the Zig source uses.
	a.Clock.HandleEvent(domain.UserChangeConfigEvent{Config: req})
	c.JSON(http.StatusOK, gin.H{"status": "config_updated"})
}

// -----------------------------------------------------------------------------
// Internals.
// -----------------------------------------------------------------------------
