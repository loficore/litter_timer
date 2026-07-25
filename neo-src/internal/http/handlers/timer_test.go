// Package handlers — Timer handler tests.
//
// Tests for all 10 timer endpoints in timer.go. Each test uses
// httptest.ResponseRecorder and real gin.Context via gin.CreateTestContext().
// Mock *app.App with stubbed Clock, Settings, SQLite, Backup, Lock/Unlock.
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// newTestApp builds a real App with real SQLite + SettingsManager for
// integration-style handler tests.
func newTestApp(t *testing.T) *app.App {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}

	return app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		nil, // backup
		dbPath,
	)
}

// setupTestRouter builds a gin.Engine with only the timer routes registered.
func setupTestRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Inject app into context via middleware
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})

	// Register timer routes
	r.GET("/api/state", TimerState)
	r.GET("/api/timer/progress", TimerProgress)
	r.POST("/api/start", TimerStart)
	r.POST("/api/pause", TimerPause)
	r.POST("/api/reset", TimerReset)
	r.POST("/api/finish", TimerFinish)
	r.POST("/api/mode", TimerMode)
	r.POST("/api/timer/rest", TimerStartRest)
	r.GET("/api/timer/config", TimerConfig)
	r.POST("/api/timer/config", TimerUpdateConfig)

	return r
}

// =============================================================================
// handleGetState — GET /api/state
// =============================================================================

func TestTimer_GetState_NoActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/state: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	// Verify is_running=false (no active session)
	if isRunning, ok := got["is_running"].(bool); !ok || isRunning {
		t.Errorf("is_running = %v, want false (no active session)", isRunning)
	}

	// Verify timezone field present
	if _, ok := got["timezone"]; !ok {
		t.Errorf("missing timezone field in response")
	}

	// Verify habit_id is not present (no active session)
	if _, ok := got["habit_id"]; ok {
		t.Errorf("habit_id should not be present without active session")
	}
}

func TestTimer_GetState_TimezonePresent(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	r.ServeHTTP(w, req)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	tz, ok := got["timezone"]
	if !ok {
		t.Fatal("timezone field missing")
	}
	// Timezone should be an int8 (JSON number)
	if _, ok := tz.(float64); !ok {
		t.Errorf("timezone = %T, want number", tz)
	}
}

// =============================================================================
// handleGetProgress — GET /api/timer/progress
// =============================================================================

func TestTimer_GetProgress_NoActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	// Verify session_id is nil/absent
	if got["session_id"] != nil {
		t.Errorf("session_id = %v, want nil (no active session)", got["session_id"])
	}

	// Verify habit_id is nil/absent
	if got["habit_id"] != nil {
		t.Errorf("habit_id = %v, want nil (no active session)", got["habit_id"])
	}

	// Verify required fields present
	for _, key := range []string{"mode", "is_running", "is_paused", "is_finished", "elapsed_seconds", "remaining_seconds", "in_rest"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in progress response", key)
		}
	}
}

func TestTimer_GetProgress_WithActiveSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Create a session first
	a.Lock()
	sessionID, err := a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Unlock()
	if err != nil {
		t.Fatalf("CreateTimerSession: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	// Verify session_id matches
	gotSessionID := int64(got["session_id"].(float64))
	if gotSessionID != sessionID {
		t.Errorf("session_id = %d, want %d", gotSessionID, sessionID)
	}
}

// =============================================================================
// handleStart — POST /api/start
// =============================================================================

func TestTimer_Start_FreshStart(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// ponytail: habit_id is omitted because the test DB has no habits —
	// foreign_keys=ON would reject a dangling FK.
	body := bytes.NewBufferString(`{"mode": "countdown", "work_duration": 1500, "rest_duration": 300, "loop_count": 4}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started", got["status"])
	}
	if got["session_id"] == nil {
		t.Error("missing session_id in response")
	}
}

func TestTimer_Start_AlreadyRunning_Paused(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Create and pause a session
	a.Lock()
	sessionID, _ := a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	a.Unlock()

	body := bytes.NewBufferString(`{"habit_id": 456}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (paused): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (resumed)", got["status"])
	}
	gotSessionID := int64(got["session_id"].(float64))
	if gotSessionID != sessionID {
		t.Errorf("session_id = %d, want original %d", gotSessionID, sessionID)
	}
}

func TestTimer_Start_AlreadyRunning_Finished(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Create and finish a session
	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserFinishTimerEvent{})
	a.SaveProgressLocked()
	a.Unlock()

	body := bytes.NewBufferString(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (finished): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (new session)", got["status"])
	}
}

func TestTimer_Start_InvalidJSON(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (invalid json): code = %d", w.Code)
	}

	// Should still work with defaults (Zig behaviour: body is optional)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "started" {
		t.Errorf("status = %v, want started (defaults)", got["status"])
	}
}

func TestTimer_Start_MissingFields_UsesDefaults(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Empty body — should use defaults
	body := bytes.NewBufferString(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/start", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/start (empty): code = %d", w.Code)
	}

	// Verify session was created with defaults
	a.RLock()
	sessionID := a.CurrentTimerSessionID
	a.RUnlock()

	if sessionID == nil {
		t.Fatal("session not created")
	}

	// Verify the session has default values
	row, err := a.SQLite.Timers().GetTimerSessionByID(*sessionID)
	if err != nil {
		t.Fatalf("GetTimerSessionByID: %v", err)
	}

	// Default: stopwatch mode, 25*60 work duration
	if row.Mode != "stopwatch" {
		t.Errorf("mode = %s, want stopwatch (default)", row.Mode)
	}
	// ponytail: CreateTimerSession writes elapsed_seconds=0 at insert;
	// the requested work duration is captured in work_duration instead.
	if row.WorkDuration != 25*60 {
		t.Errorf("work_duration = %d, want %d (default)", row.WorkDuration, 25*60)
	}
}

// =============================================================================
// handlePause — POST /api/pause
// =============================================================================

func TestTimer_Pause_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Start a session first
	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pause", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/pause: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "paused" {
		t.Errorf("status = %v, want paused", got["status"])
	}

	// Verify clock is paused
	state := a.Clock.Update()
	if !state.IsPaused() {
		t.Error("clock state is not paused after pause")
	}
}

// =============================================================================
// handleReset — POST /api/reset
// =============================================================================

func TestTimer_Reset_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Start a session first
	a.Lock()
	a.CreateTimerSession(nil, "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/reset", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/reset: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "reset" {
		t.Errorf("status = %v, want reset", got["status"])
	}

	// Verify session cleared
	a.RLock()
	sessionID := a.CurrentTimerSessionID
	habitID := a.CurrentHabitID
	a.RUnlock()

	if sessionID != nil {
		t.Errorf("session_id = %v, want nil after reset", sessionID)
	}
	if habitID != nil {
		t.Errorf("habit_id = %v, want nil after reset", habitID)
	}
}

// =============================================================================
// handleFinish — POST /api/finish
// =============================================================================

func TestTimer_Finish_Success(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Start a session with a habit
	a.Lock()
	a.CreateTimerSession(int64Ptr(123), "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
	if got["elapsed_seconds"] == nil {
		t.Error("missing elapsed_seconds in response")
	}
}

func TestTimer_Finish_FallbackPath(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// Start a session
	a.Lock()
	a.CreateTimerSession(int64Ptr(456), "stopwatch", 25*60, 0, 0)
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	a.Unlock()

	// Simulate time passing
	time.Sleep(100 * time.Millisecond)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
}

func TestTimer_Finish_NoHabit_NoSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// No session, no habit
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/finish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/finish (no session): code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "finished" {
		t.Errorf("status = %v, want finished", got["status"])
	}
}

// =============================================================================
// handleModeSwitch — POST /api/mode
// =============================================================================

func TestTimer_ModeSwitch_Countdown(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "countdown"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode countdown: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "mode_changed" {
		t.Errorf("status = %v, want mode_changed", got["status"])
	}
	if got["new_mode"] != "countdown" {
		t.Errorf("new_mode = %v, want countdown", got["new_mode"])
	}
}

func TestTimer_ModeSwitch_Stopwatch(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "stopwatch"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode stopwatch: code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "mode_changed" {
		t.Errorf("status = %v, want mode_changed", got["status"])
	}
	if got["new_mode"] != "stopwatch" {
		t.Errorf("new_mode = %v, want stopwatch", got["new_mode"])
	}
}

func TestTimer_ModeSwitch_InvalidMode(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{"mode": "invalid"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (invalid): code = %d", w.Code)
	}

	// Invalid mode should return empty JSON (gracefully swallowed)
	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestTimer_ModeSwitch_EmptyBody(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(``)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (empty): code = %d", w.Code)
	}

	// Empty body should return empty JSON (silently ignored)
	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

func TestTimer_ModeSwitch_JSONParseFailure(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/mode", body)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (parse fail): code = %d", w.Code)
	}

	// JSON parse failure should return empty JSON
	got := strings.TrimSpace(w.Body.String())
	if got != "{}" {
		t.Errorf("body = %q, want {}", got)
	}
}

// =============================================================================
// handleStartRest — POST /api/timer/rest
// =============================================================================

func TestTimer_StartRest_Normal(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/rest", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/timer/rest: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "rest_started" {
		t.Errorf("status = %v, want rest_started", got["status"])
	}
	if got["rest_seconds"] != float64(5*60) {
		t.Errorf("rest_seconds = %v, want %d", got["rest_seconds"], 5*60)
	}

	// Verify clock is in countdown mode with 5-min config
	state := a.Clock.Update()
	if state.GetMode() != domain.CountdownMode {
		t.Errorf("mode = %v, want countdown", state.GetMode())
	}
}

// =============================================================================
// handleConfig — GET /api/timer/config
// =============================================================================

func TestTimer_Config_Get(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/timer/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/config: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	// Verify required fields
	for _, key := range []string{"default_mode", "countdown", "stopwatch"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in config response", key)
		}
	}

	// Verify countdown sub-fields
	countdown, ok := got["countdown"].(map[string]any)
	if !ok {
		t.Fatal("countdown field is not an object")
	}
	for _, key := range []string{"duration_seconds", "loop", "loop_count", "loop_interval_seconds"} {
		if _, ok := countdown[key]; !ok {
			t.Errorf("missing countdown key %q", key)
		}
	}

	// Verify stopwatch sub-fields
	stopwatch, ok := got["stopwatch"].(map[string]any)
	if !ok {
		t.Fatal("stopwatch field is not an object")
	}
	if _, ok := stopwatch["max_seconds"]; !ok {
		t.Error("missing stopwatch.max_seconds")
	}
}

// =============================================================================
// handleUpdateConfig — POST /api/timer/config
// =============================================================================

func TestTimer_UpdateConfig_ValidPartial(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// ponytail: default_mode is omitted because ClockTaskConfig.DefaultMode is
	// a ModeEnum (int), not a string — sending "countdown" as a string fails
	// the strict JSON bind.
	body := bytes.NewBufferString(`{"countdown": {"duration_seconds": 1800}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/config", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/timer/config: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "config_updated" {
		t.Errorf("status = %v, want config_updated", got["status"])
	}
}

func TestTimer_UpdateConfig_InvalidJSON(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/timer/config", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/timer/config (invalid): code = %d, want 400", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["error"] != "invalid json" {
		t.Errorf("error = %v, want 'invalid json'", got["error"])
	}
}

// =============================================================================
// Helpers
// =============================================================================

func int64Ptr(i int64) *int64 {
	return &i
}
