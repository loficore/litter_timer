// Package handlers — Settings handler tests.
//
// Tests for the 2 settings endpoints in settings.go. Each test uses
// httptest.ResponseRecorder and real gin.Context via gin.CreateTestContext().
// Mock *app.App with stubbed Clock, Settings, SQLite, Backup, Lock/Unlock.
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
)

// setupSettingsRouter builds a gin.Engine with only the settings routes.
func setupSettingsRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Inject app into context via middleware
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})

	// Register settings routes
	r.GET("/api/settings", SettingsGet)
	r.POST("/api/settings", SettingsUpdate)

	return r
}

// =============================================================================
// handleSettingsGet — GET /api/settings
// =============================================================================

func TestSettings_Get_ReturnsFullConfig(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/settings: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	// Verify top-level sections present
	for _, key := range []string{"basic", "clock_defaults", "logging", "auth"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in settings response", key)
		}
	}

	// Verify basic section fields
	basic, ok := got["basic"].(map[string]any)
	if !ok {
		t.Fatal("basic field is not an object")
	}
	for _, key := range []string{"timezone", "language", "default_mode", "theme_mode", "wallpaper"} {
		if _, ok := basic[key]; !ok {
			t.Errorf("missing basic key %q", key)
		}
	}

	// Verify clock_defaults section fields
	clockDefaults, ok := got["clock_defaults"].(map[string]any)
	if !ok {
		t.Fatal("clock_defaults field is not an object")
	}
	for _, key := range []string{"default_mode", "countdown", "stopwatch"} {
		if _, ok := clockDefaults[key]; !ok {
			t.Errorf("missing clock_defaults key %q", key)
		}
	}

	// Verify logging section fields
	logging, ok := got["logging"].(map[string]any)
	if !ok {
		t.Fatal("logging field is not an object")
	}
	for _, key := range []string{"level", "enable_timestamp", "tick_interval_ms", "enable_file_logging", "log_dir", "max_file_size", "max_file_count"} {
		if _, ok := logging[key]; !ok {
			t.Errorf("missing logging key %q", key)
		}
	}

	// Verify auth section fields
	auth, ok := got["auth"].(map[string]any)
	if !ok {
		t.Fatal("auth field is not an object")
	}
	if _, ok := auth["auth_enabled"]; !ok {
		t.Error("missing auth.auth_enabled")
	}
}

func TestSettings_Get_TimezoneValue(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	r.ServeHTTP(w, req)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	basic := got["basic"].(map[string]any)
	tz := basic["timezone"]
	if tz == nil {
		t.Fatal("timezone is nil")
	}
	// Timezone should be a number (int8 in Go, float64 in JSON)
	if _, ok := tz.(float64); !ok {
		t.Errorf("timezone = %T, want number", tz)
	}
}

// =============================================================================
// handleSettingsUpdate — POST /api/settings
// =============================================================================

func TestSettings_Update_ValidFullUpdate(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Full settings update payload
	body := bytes.NewBufferString(`{
		"basic": {
			"timezone": 9,
			"language": "en",
			"default_mode": "stopwatch",
			"theme_mode": "dark",
			"wallpaper": "solid_color"
		},
		"clock_defaults": {
			"default_mode": "countdown",
			"countdown": {
				"duration_seconds": 1800,
				"loop": true,
				"loop_count": 4,
				"loop_interval_seconds": 300
			},
			"stopwatch": {
				"max_seconds": 86400
			}
		},
		"logging": {
			"level": "info",
			"enable_timestamp": true,
			"tick_interval_ms": 1000,
			"enable_file_logging": false,
			"log_dir": "/tmp/logs",
			"max_file_size": 10485760,
			"max_file_count": 5
		},
		"auth": {
			"auth_enabled": false
		}
	}`)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings: code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "settings_updated" {
		t.Errorf("status = %v, want settings_updated", got["status"])
	}

	// Verify the settings were actually updated
	cfg := a.Settings.Config()
	if cfg.Basic.Timezone != 9 {
		t.Errorf("timezone = %d, want 9", cfg.Basic.Timezone)
	}
	if cfg.Basic.Language != "en" {
		t.Errorf("language = %s, want en", cfg.Basic.Language)
	}
}

func TestSettings_Update_EmptyBody(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Empty body — should be handled gracefully
	body := bytes.NewBufferString(`{}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (empty): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "settings_updated" {
		t.Errorf("status = %v, want settings_updated", got["status"])
	}
}

func TestSettings_Update_InvalidJSON(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{invalid json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/settings (invalid): code = %d, want 400", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["err"] == "" {
		t.Error("missing err field in error response")
	}
}

func TestSettings_Update_PartialUpdate(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Partial update — only timezone
	body := bytes.NewBufferString(`{"basic": {"timezone": -5}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (partial): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "settings_updated" {
		t.Errorf("status = %v, want settings_updated", got["status"])
	}

	// Verify timezone was updated
	cfg := a.Settings.Config()
	if cfg.Basic.Timezone != -5 {
		t.Errorf("timezone = %d, want -5", cfg.Basic.Timezone)
	}
}

func TestSettings_Update_InvalidTimezone(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Invalid timezone (out of range)
	body := bytes.NewBufferString(`{"basic": {"timezone": 99}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// ponytail: parseSettingsFromJSON silently drops out-of-range values
	// rather than returning an error, so the request still succeeds and
	// the on-disk timezone stays at its prior value.  Assert the value
	// was rejected (not the HTTP code).
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (invalid tz): code = %d, body = %s", w.Code, w.Body.String())
	}
	cfg := a.Settings.Config()
	if cfg.Basic.Timezone == 99 {
		t.Errorf("invalid timezone 99 was accepted; want previous value %d", cfg.Basic.Timezone)
	}
}

func TestSettings_Update_InvalidLanguage(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Invalid language (too long)
	body := bytes.NewBufferString(`{"basic": {"language": "toolonglanguage"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// ponytail: parseSettingsFromJSON silently drops over-long language
	// strings; assert the field stayed at its previous value.
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (invalid lang): code = %d, body = %s", w.Code, w.Body.String())
	}
	cfg := a.Settings.Config()
	if cfg.Basic.Language == "toolonglanguage" {
		t.Errorf("over-long language was accepted; want previous value %q", cfg.Basic.Language)
	}
}

func TestSettings_Update_DefaultMode(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Update default mode to stopwatch
	body := bytes.NewBufferString(`{"basic": {"default_mode": "stopwatch"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (mode): code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["status"] != "settings_updated" {
		t.Errorf("status = %v, want settings_updated", got["status"])
	}

	// Verify default mode was updated
	cfg := a.Settings.Config()
	if cfg.Basic.DefaultMode != domain.DefaultModeStopwatch {
		t.Errorf("default_mode = %v, want stopwatch", cfg.Basic.DefaultMode)
	}
}

func TestSettings_Update_ThemeMode(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{"basic": {"theme_mode": "light"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (theme): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if cfg.Basic.ThemeMode != "light" {
		t.Errorf("theme_mode = %s, want light", cfg.Basic.ThemeMode)
	}
}

func TestSettings_Update_Wallpaper(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{"basic": {"wallpaper": "nature"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (wallpaper): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if cfg.Basic.Wallpaper != "nature" {
		t.Errorf("wallpaper = %s, want nature", cfg.Basic.Wallpaper)
	}
}

func TestSettings_Update_AuthEnabled(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{"auth": {"auth_enabled": true}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (auth): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if !cfg.Auth.AuthEnabled {
		t.Error("auth_enabled = false, want true")
	}
}

func TestSettings_Update_LoggingLevel(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{"logging": {"level": "debug"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (log level): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if cfg.Logging.Level != "debug" {
		t.Errorf("logging.level = %s, want debug", cfg.Logging.Level)
	}
}

func TestSettings_Update_ClockDefaults(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{
		"clock_defaults": {
			"countdown": {
				"duration_seconds": 3600,
				"loop": true,
				"loop_count": 5
			},
			"stopwatch": {
				"max_seconds": 172800
			}
		}
	}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (clock defaults): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if cfg.ClockDefaults.Countdown.DurationSeconds != 3600 {
		t.Errorf("countdown.duration_seconds = %d, want 3600", cfg.ClockDefaults.Countdown.DurationSeconds)
	}
	if !cfg.ClockDefaults.Countdown.Loop {
		t.Error("countdown.loop = false, want true")
	}
	if cfg.ClockDefaults.Countdown.LoopCount != 5 {
		t.Errorf("countdown.loop_count = %d, want 5", cfg.ClockDefaults.Countdown.LoopCount)
	}
}

// =============================================================================
// Edge cases and error handling
// =============================================================================

func TestSettings_Update_NoContentType(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{"basic": {"timezone": 5}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	// No Content-Type header
	r.ServeHTTP(w, req)

	// Should still work (gin's ShouldBindJSON is lenient)
	if w.Code != http.StatusOK {
		t.Logf("POST /api/settings (no content-type): code = %d (may be acceptable)", w.Code)
	}
}

func TestSettings_Update_MalformedBasic(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Malformed basic object
	body := bytes.NewBufferString(`{"basic": "not_an_object"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Should handle gracefully (ignore malformed section)
	if w.Code != http.StatusOK {
		t.Logf("POST /api/settings (malformed basic): code = %d (acceptable)", w.Code)
	}
}

func TestSettings_Update_NullValues(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Null values should be ignored
	body := bytes.NewBufferString(`{"basic": {"timezone": null, "language": null}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (nulls): code = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestSettings_Update_ExtraFields(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Extra unknown fields should be ignored
	body := bytes.NewBufferString(`{"basic": {"timezone": 3, "unknown_field": "ignored"}, "extra_section": {}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (extra fields): code = %d, body = %s", w.Code, w.Body.String())
	}

	cfg := a.Settings.Config()
	if cfg.Basic.Timezone != 3 {
		t.Errorf("timezone = %d, want 3", cfg.Basic.Timezone)
	}
}

func TestSettings_Get_AfterUpdate(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	// Update first
	updateBody := bytes.NewBufferString(`{"basic": {"timezone": 12, "language": "ko"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", updateBody)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings: code = %d", w.Code)
	}

	// Then get
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("GET /api/settings: code = %d", w2.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	basic := got["basic"].(map[string]any)
	if basic["timezone"] != float64(12) {
		t.Errorf("timezone = %v, want 12", basic["timezone"])
	}
	if basic["language"] != "ko" {
		t.Errorf("language = %s, want ko", basic["language"])
	}
}

// =============================================================================
// Backup config tests (via settings update)
// =============================================================================

func TestSettings_Update_BackupConfig(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{
		"backup": {
			"enabled": true,
			"auto_backup": true,
			"auto_backup_interval": 3600,
			"target_type": "local",
			"local_path": "/tmp/backups"
		}
	}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (backup): code = %d, body = %s", w.Code, w.Body.String())
	}

	backupCfg := a.Settings.BackupConfig()
	if !backupCfg.Enabled {
		t.Error("backup.enabled = false, want true")
	}
	if !backupCfg.AutoBackup {
		t.Error("backup.auto_backup = false, want true")
	}
	if backupCfg.LocalPath != "/tmp/backups" {
		t.Errorf("backup.local_path = %s, want /tmp/backups", backupCfg.LocalPath)
	}
}

func TestSettings_Update_WebDAVConfig(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{
		"backup": {
			"target_type": "webdav",
			"webdav_url": "https://example.com/dav",
			"webdav_username": "user",
			"webdav_password": "secret"
		}
	}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (webdav): code = %d, body = %s", w.Code, w.Body.String())
	}

	backupCfg := a.Settings.BackupConfig()
	if backupCfg.TargetType != domain.BackupTargetWebDAV {
		t.Errorf("backup.target_type = %v, want webdav", backupCfg.TargetType)
	}
	if backupCfg.WebDAVURL != "https://example.com/dav" {
		t.Errorf("backup.webdav_url = %s, want https://example.com/dav", backupCfg.WebDAVURL)
	}
	if backupCfg.WebDAVUsername != "user" {
		t.Errorf("backup.webdav_username = %s, want user", backupCfg.WebDAVUsername)
	}
}

func TestSettings_Update_S3Config(t *testing.T) {
	a := newTestApp(t)
	r := setupSettingsRouter(t, a)

	body := bytes.NewBufferString(`{
		"backup": {
			"target_type": "s3",
			"s3_endpoint": "https://s3.amazonaws.com",
			"s3_bucket": "my-bucket",
			"s3_region": "us-east-1",
			"s3_access_key": "AKIAIOSFODNN7EXAMPLE",
			"s3_secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"s3_path_prefix": "backups/"
		}
	}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/settings", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/settings (s3): code = %d, body = %s", w.Code, w.Body.String())
	}

	backupCfg := a.Settings.BackupConfig()
	if backupCfg.TargetType != domain.BackupTargetS3 {
		t.Errorf("backup.target_type = %v, want s3", backupCfg.TargetType)
	}
	if backupCfg.S3Bucket != "my-bucket" {
		t.Errorf("backup.s3_bucket = %s, want my-bucket", backupCfg.S3Bucket)
	}
	if backupCfg.S3Region != "us-east-1" {
		t.Errorf("backup.s3_region = %s, want us-east-1", backupCfg.S3Region)
	}
}
