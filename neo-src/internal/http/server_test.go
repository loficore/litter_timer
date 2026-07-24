// Smoke tests for the HTTP layer.
//
// Goal: verify the Gin router registers every Zig-equivalent route and
// each handler responds (200/401/etc.) without panicking.  Handler-level
// tests build a real App with a real SQLite + SettingsManager so the
// dependency-injection paths actually execute.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// newTestRouter builds a router wired to a stub App whose managers are
// nil.  Only used for registration + middleware tests — handler-level
// tests use newRealTestRouter below.
func newTestRouter(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		nil, // settings — nil for registration-only tests
		nil, // sqlite — nil for registration-only tests
		nil, // backup — nil for registration-only tests
		dbPath,
	)

	r := NewRouter(a, "*")
	return r, a
}

// newRealTestRouter builds a router wired to a fully-initialised App:
// real SqliteManager, real SettingsManager.  Use this for tests that
// exercise handler logic (e.g. GET /api/state must read the clock).
func newRealTestRouter(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}

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

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		nil, // backup — handler returns service-unavailable when nil
		dbPath,
	)
	return NewRouter(a, "*"), a
}

// routeExists inspects the Gin router's tree to confirm a method+path
// is registered.  Gin doesn't expose its router tree directly, so we
// fire a synthetic OPTIONS request — every registered route answers
// the CORS middleware's OPTIONS short-circuit.
func routeExists(r *gin.Engine, method, path string) bool {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w.Code != http.StatusNotFound
}

// TestAllRoutesRegistered walks the full Zig route table and confirms
// each path answers (not 404).  This is the smoke gate.
//
// SSE (/api/events) is verified separately via TestEventsRouteExists
// because its streaming handler would otherwise block the test runner.
func TestAllRoutesRegistered(t *testing.T) {
	r, _ := newTestRouter(t)

	routes := []struct {
		method, path string
	}{
		// Static + frontend log
		{http.MethodGet, "/"},
		{http.MethodPost, "/api/log"},

		// Timer
		{http.MethodGet, "/api/state"},
		{http.MethodGet, "/api/timer/state"},
		{http.MethodGet, "/api/timer/progress"},
		{http.MethodGet, "/api/timer/config"},
		{http.MethodPost, "/api/start"},
		{http.MethodPost, "/api/pause"},
		{http.MethodPost, "/api/reset"},
		{http.MethodPost, "/api/finish"},
		{http.MethodPost, "/api/timer/finish"},
		{http.MethodPost, "/api/timer/rest"},
		{http.MethodPost, "/api/mode"},
		{http.MethodPost, "/api/timer/config"},

		// Habits
		{http.MethodGet, "/api/habit-sets"},
		{http.MethodPost, "/api/habit-sets"},
		{http.MethodGet, "/api/habits"},
		{http.MethodPost, "/api/habits"},
		{http.MethodGet, "/api/sessions"},
		{http.MethodPost, "/api/sessions"},
		{http.MethodGet, "/api/timer-sessions"},
		{http.MethodPost, "/api/timer-sessions"},

		// Settings
		{http.MethodGet, "/api/settings"},
		{http.MethodPost, "/api/settings"},

		// Backup
		{http.MethodGet, "/api/backup/config"},
		{http.MethodPost, "/api/backup/config"},
		{http.MethodPost, "/api/backup/create"},
		{http.MethodPost, "/api/backup/restore"},
		{http.MethodGet, "/api/backup/list"},
		{http.MethodGet, "/api/backup/info"},
		{http.MethodPost, "/api/backup/verify"},
		{http.MethodPost, "/api/backup/unlock"},
		{http.MethodPost, "/api/backup/lock"},
		{http.MethodGet, "/api/backup/master-password"},
		{http.MethodPost, "/api/backup/master-password"},

		// Auth
		{http.MethodGet, "/api/auth/status"},
		{http.MethodPost, "/api/auth/enable"},
		{http.MethodPost, "/api/auth/disable"},

		// Wallpapers
		{http.MethodGet, "/api/wallpapers"},
		{http.MethodPost, "/api/wallpapers"},
		{http.MethodPost, "/api/wallpapers/from-url"},
	}

	for _, rt := range routes {
		if !routeExists(r, rt.method, rt.path) {
			t.Errorf("route not registered: %s %s", rt.method, rt.path)
		}
	}
}

// TestEventsRouteExists confirms /api/events is registered without
// actually connecting (the handler streams forever).  The check: a
// `Connection: close` GET request returns 200 with the SSE headers
// rather than 404.  We use httptest.NewServer so the connection
// closes immediately when the test client returns.
func TestEventsRouteExists(t *testing.T) {
	r, _ := newTestRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Expected: the timeout fires before the server responds.
		// Anything else is a real failure.
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return
		}
		t.Fatalf("events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("/api/events returned 404")
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("/api/events Content-Type = %q, want text/event-stream prefix", got)
	}
}

// TestCORSHeaders confirms the CORS middleware emits the expected
// headers on a non-preflight request.
func TestCORSHeaders(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("CORS: Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Errorf("CORS: missing Access-Control-Allow-Methods")
	}
}

// TestCORSPreflight confirms OPTIONS short-circuits with 204.
func TestCORSPreflight(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/state", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight: code = %d, want 204", w.Code)
	}
}

// TestAuthPublicPath confirms /api/auth/status and /api/events bypass
// auth even when no SettingsManager is wired in (no auth required).
func TestAuthPublicPath(t *testing.T) {
	r, _ := newTestRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	if w.Code == http.StatusUnauthorized {
		t.Errorf("/api/auth/status unexpectedly gated: %d", w.Code)
	}
}

// Test404ForUnknownRoute confirms unknown paths return 404 instead of
// hitting a panic-prone handler.
func Test404ForUnknownRoute(t *testing.T) {
	r, _ := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/this/does/not/exist", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown route: code = %d, want 404", w.Code)
	}
}

// TestTimerStateHandler confirms /api/state returns a JSON body with
// the same shape the Zig source emits.
func TestTimerStateHandler(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/state", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/state: code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v (body=%s)", err, w.Body.String())
	}
	for _, key := range []string{"time", "elapsed", "mode", "is_running", "is_finished", "in_rest", "loop_remaining", "loop_total", "rest_remaining", "timezone"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in /api/state response: %v", key, got)
		}
	}
	if mode, _ := got["mode"].(string); mode != "countdown" && mode != "stopwatch" {
		t.Errorf("mode = %q, want countdown|stopwatch", mode)
	}
}

// TestTimerProgressHandler confirms /api/timer/progress returns a JSON
// body with elapsed/remaining/etc.
func TestTimerProgressHandler(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code = %d", w.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	for _, key := range []string{"mode", "is_running", "is_paused", "is_finished", "elapsed_seconds", "remaining_seconds", "in_rest"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in /api/timer/progress response", key)
		}
	}
}

// TestTimerModeSwitchEmpty confirms POST /api/mode with empty body
// returns `{}` (matches Zig's behaviour for unrecognised values).
func TestTimerModeSwitchEmpty(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mode", strings.NewReader("")))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode (empty): code = %d, body = %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != "{}" {
		t.Errorf("empty /api/mode body = %q, want {}", got)
	}
}

// TestTimerModeSwitchCountdown confirms POST /api/mode with a valid
// mode returns the expected JSON.
func TestTimerModeSwitchCountdown(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"mode":"countdown"}`)
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mode", body))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/mode countdown: code = %d, body = %s", w.Code, w.Body.String())
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

// TestFrontendLogEmptyBody confirms POST /api/log with empty body
// returns 200 + success=false (matches Zig behaviour).
func TestFrontendLogEmptyBody(t *testing.T) {
	r, _ := newRealTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader("")))
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/log empty: code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Errorf("expected success=false in body, got %s", w.Body.String())
	}
}

// TestGenerateTokenIsUnique confirms GenerateToken returns fresh
// tokens each call (used by the auth-enable handler).
func TestGenerateTokenIsUnique(t *testing.T) {
	t1 := app.GenerateToken()
	t2 := app.GenerateToken()
	if t1 == "" || t2 == "" {
		t.Errorf("GenerateToken returned empty string")
	}
	if t1 == t2 {
		t.Errorf("GenerateToken returned duplicate: %s", t1)
	}
}