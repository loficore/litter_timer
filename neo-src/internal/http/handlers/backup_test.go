package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

func newBackupTestApp(t *testing.T) (*app.App, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	backupDir := tmpDir + "/backups"

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

	cfg := domain.NewDefaultBackupConfig()
	cfg.Enabled = true
	cfg.TargetType = domain.BackupTargetLocal
	cfg.LocalPath = backupDir
	if err := sm.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg)); err != nil {
		t.Fatalf("update backup config: %v", err)
	}

	bm, err := backup.NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("backup manager: %v", err)
	}

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		bm,
		dbPath,
	)
	return a, backupDir
}

func backupConfigToJSON(cfg domain.BackupConfig) string {
	b, _ := json.Marshal(cfg)
	return string(b)
}

func TestHandleBackupConfigGet(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/backup/config", nil)
	c.Set("app", a)

	BackupConfigGet(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if got["enabled"] != true {
		t.Errorf("enabled = %v, want true", got["enabled"])
	}
	if got["target_type"] != "local" {
		t.Errorf("target_type = %v, want local", got["target_type"])
	}
}

func TestHandleBackupConfigUpdate(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"enabled":true,"target_type":"local","local_path":"/tmp/backups"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/config", body)
	c.Set("app", a)

	BackupConfigUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupConfigUpdate_CloudWithoutMasterPassword(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"enabled":true,"target_type":"webdav","webdav_url":"http://example.com"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/config", body)
	c.Set("app", a)

	BackupConfigUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["error"] != "master_password_required" {
		t.Errorf("error = %v, want master_password_required", got["error"])
	}
}

func TestHandleBackupConfigUpdate_S3(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)

	// Given: S3 updates require an unlocked master password.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"password":"testpass123"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/master-password", body)
	c.Set("app", a)
	MasterPasswordSet(c)
	if w.Code != http.StatusOK {
		t.Fatalf("master-password code = %d, body = %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	body = strings.NewReader(`{"enabled":true,"target_type":"s3","s3_endpoint":"https://s3.example.com","s3_bucket":"my-bucket","s3_region":"us-east-1","s3_access_key":"AKIAEXAMPLE","s3_secret_key":"secret","s3_path_prefix":"backups"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/config", body)
	c.Set("app", a)

	BackupConfigUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}

	// When: the persisted config is read back and reflects the S3 settings.
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/backup/config", nil)
	c.Set("app", a)
	BackupConfigGet(c)

	if w.Code != http.StatusOK {
		t.Fatalf("get code = %d, body = %s", w.Code, w.Body.String())
	}
	var gotCfg map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &gotCfg); err != nil {
		t.Fatalf("config response not JSON: %v", err)
	}
	if gotCfg["target_type"] != "s3" {
		t.Errorf("target_type = %v, want s3", gotCfg["target_type"])
	}
	if gotCfg["s3_bucket"] != "my-bucket" {
		t.Errorf("s3_bucket = %v, want my-bucket", gotCfg["s3_bucket"])
	}
}

func TestHandleBackupConfigUpdate_S3WithoutMasterPassword(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"enabled":true,"target_type":"s3","s3_endpoint":"https://s3.example.com","s3_bucket":"my-bucket","s3_region":"us-east-1","s3_access_key":"AKIAEXAMPLE","s3_secret_key":"secret","s3_path_prefix":"backups"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/config", body)
	c.Set("app", a)

	BackupConfigUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["error"] != "master_password_required" {
		t.Errorf("error = %v, want master_password_required", got["error"])
	}
}

func TestHandleBackupCreate(t *testing.T) {
	a, backupDir := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)

	BackupCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if got["backup_path"] == nil {
		t.Errorf("missing backup_path in response")
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) == 0 {
		t.Errorf("no backup files created in %s", backupDir)
	}
}

func TestHandleBackupCreate_NoBackupManager(t *testing.T) {
	a, _ := newBackupTestApp(t)
	a.Backup = nil
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)

	BackupCreate(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", w.Code)
	}
}

func TestHandleBackupRestore(t *testing.T) {
	a, _ := newBackupTestApp(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)
	BackupCreate(c)

	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	backupName, _ := created["backup_path"].(string)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	body := strings.NewReader(`{"name":"` + backupName + `"}`)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/backup/restore", body)
	c2.Set("app", a)

	BackupRestore(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupRestore_MissingName(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/restore", body)
	c.Set("app", a)

	BackupRestore(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleBackupRestoreByName(t *testing.T) {
	a, _ := newBackupTestApp(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)
	BackupCreate(c)

	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	backupName, _ := created["backup_path"].(string)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/backup/restore/"+backupName, nil)
	c2.Set("app", a)

	BackupRestoreByName(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupRestoreByName_InvalidName(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/restore/../../../etc/passwd", nil)
	c.Set("app", a)

	BackupRestoreByName(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleBackupList(t *testing.T) {
	a, _ := newBackupTestApp(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)
	BackupCreate(c)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/backup/list", nil)
	c2.Set("app", a)

	BackupList(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupList_NoBackupManager(t *testing.T) {
	a, _ := newBackupTestApp(t)
	a.Backup = nil
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/backup/list", nil)
	c.Set("app", a)

	BackupList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	backups, _ := got["backups"].([]any)
	if len(backups) != 0 {
		t.Errorf("expected empty list, got %d items", len(backups))
	}
}

func TestHandleBackupInfo(t *testing.T) {
	a, _ := newBackupTestApp(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)
	BackupCreate(c)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/backup/info", nil)
	c2.Set("app", a)

	BackupInfo(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupDeleteByName(t *testing.T) {
	a, _ := newBackupTestApp(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)
	BackupCreate(c)

	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	backupName, _ := created["backup_path"].(string)

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/api/backup/delete/"+backupName, nil)
	c2.Set("app", a)

	BackupDeleteByName(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupVerify(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/verify", nil)
	c.Set("app", a)

	BackupVerify(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupUnlock(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"password":"testpass"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/unlock", body)
	c.Set("app", a)

	BackupUnlock(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleBackupUnlock_MissingPassword(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/unlock", body)
	c.Set("app", a)

	BackupUnlock(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleBackupLock(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/lock", nil)
	c.Set("app", a)

	BackupLock(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleMasterPasswordGet(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/backup/master-password", nil)
	c.Set("app", a)

	MasterPasswordGet(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if _, ok := got["has_password"]; !ok {
		t.Errorf("missing has_password in response")
	}
	if _, ok := got["unlocked"]; !ok {
		t.Errorf("missing unlocked in response")
	}
}

func TestHandleMasterPasswordSet(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"password":"testpass123"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/master-password", body)
	c.Set("app", a)

	MasterPasswordSet(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleMasterPasswordSet_TooShort(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"password":"abc"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/master-password", body)
	c.Set("app", a)

	MasterPasswordSet(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleAuthStatus(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	c.Set("app", a)

	AuthStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if _, ok := got["auth_enabled"]; !ok {
		t.Errorf("missing auth_enabled in response")
	}
	if _, ok := got["has_token"]; !ok {
		t.Errorf("missing has_token in response")
	}
}

func TestHandleAuthEnable(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/enable", nil)
	c.Set("app", a)

	AuthEnable(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if got["token"] == nil {
		t.Errorf("missing token in response")
	}
}

func TestHandleAuthDisable(t *testing.T) {
	a, _ := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/disable", nil)
	c.Set("app", a)

	AuthDisable(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestValidBackupName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"backup_2024_01_01", true},
		{"backup-2024.db", true},
		{"/absolute", false},
		{"back\x00up.db", false},
		{"back\x1fab", false},
		{"\\absolute", false},
	}

	for _, tt := range tests {
		if got := validBackupName(tt.name); got != tt.want {
			t.Errorf("validBackupName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMask(t *testing.T) {
	if mask("") != "" {
		t.Errorf("mask(\"\") = %q, want \"\"", mask(""))
	}
	if mask("secret") != "******" {
		t.Errorf("mask(\"secret\") = %q, want \"******\"", mask("secret"))
	}
}

func TestMasterPasswordError(t *testing.T) {
	got := masterPasswordError("test_code", "test message", "setup")

	if got["success"] != false {
		t.Errorf("success = %v, want false", got["success"])
	}
	if got["error"] != "test_code" {
		t.Errorf("error = %v, want test_code", got["error"])
	}
	if got["message"] != "test message" {
		t.Errorf("message = %v, want 'test message'", got["message"])
	}
	action, _ := got["action"].(gin.H)
	if action == nil {
		t.Fatal("action should be a gin.H")
	}
	if action["target"] != "master_password" {
		t.Errorf("action.target = %v, want master_password", action["target"])
	}
}

func TestHandleBackupCreate_VerifiesSHA256(t *testing.T) {
	a, backupDir := newBackupTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)

	BackupCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Fatalf("success = %v, want true: %+v", got["success"], got)
	}

	manifestPath := filepath.Join(backupDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Backups []struct {
			Sha256 string `json:"sha256"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if len(manifest.Backups) == 0 {
		t.Fatal("manifest has no backup entries")
	}
	if manifest.Backups[0].Sha256 == "" {
		t.Error("manifest sha256 field is empty")
	}
}

func TestHandleBackupCreate_FailOnSHA256Mismatch(t *testing.T) {
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

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}

	cfg := domain.NewDefaultBackupConfig()
	cfg.Enabled = true
	cfg.TargetType = domain.BackupTargetLocal
	cfg.LocalPath = backupDir
	if err := sm.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg)); err != nil {
		t.Fatalf("update backup config: %v", err)
	}

	fake := backup.NewFakeLocalAdapter(backupDir)
	fake.SetRestoreData([]byte("different-bytes-than-vacuum-output"))

	bm := backup.NewManagerWithAdapter(sqlite, dbPath, backupDir, fake)
	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		bm,
		dbPath,
	)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/backup/create", nil)
	c.Set("app", a)

	BackupCreate(c)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != false {
		t.Errorf("success = %v, want false", got["success"])
	}
	errStr, _ := got["error"].(string)
	if !strings.Contains(errStr, "sha256") {
		t.Errorf("error should mention sha256, got: %q", errStr)
	}

	if n := fake.DeletedNames(); len(n) != 1 {
		t.Errorf("Delete calls = %d, want 1 (best-effort cleanup of corrupted backup)", len(n))
	} else if !strings.HasPrefix(n[0], "presets_backup_") {
		t.Errorf("deleted name = %q, want presets_backup_* prefix", n[0])
	}
}
