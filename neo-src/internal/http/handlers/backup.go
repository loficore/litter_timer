// Package handlers — Backup + master-password + auth endpoints.
//
// File `backup.go` ports the backup, master-password, and auth
// handlers from std_server.zig.  Routes (paths match Zig exactly):
//
//	GET  /api/backup/config
//	POST /api/backup/config
//	POST /api/backup/create
//	POST /api/backup/restore
//	POST /api/backup/restore/:name
//	GET  /api/backup/list
//	GET  /api/backup/info
//	DELETE /api/backup/delete/:name
//	DELETE /api/backup/:id
//	POST /api/backup/verify
//	POST /api/backup/unlock
//	POST /api/backup/lock
//	GET  /api/backup/master-password
//	POST /api/backup/master-password
//
//	GET  /api/auth/status
//	POST /api/auth/enable
//	POST /api/auth/disable
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
)

// -----------------------------------------------------------------------------
// /api/backup/config
// -----------------------------------------------------------------------------

// handleBackupConfigGet mirrors `handleGetBackupConfig`.  Returns the
// persisted BackupConfig with secrets masked (`"******"`).
func BackupConfigGet(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.BackupConfig()

	c.JSON(http.StatusOK, gin.H{
		"enabled":              cfg.Enabled,
		"auto_backup":          cfg.AutoBackup,
		"auto_backup_interval": cfg.AutoBackupSecs,
		"target_type":          cfg.TargetType.String(),
		"local_path":           cfg.LocalPath,
		"webdav_url":           cfg.WebDAVURL,
		"webdav_username":      cfg.WebDAVUsername,
		"webdav_password":      mask(cfg.WebDAVPassword),
		"webdav_path_prefix":   cfg.WebDAVPathPrefix,
		"s3_endpoint":          cfg.S3Endpoint,
		"s3_bucket":            cfg.S3Bucket,
		"s3_region":            cfg.S3Region,
		"s3_access_key":        mask(cfg.S3AccessKey),
		"s3_secret_key":        mask(cfg.S3SecretKey),
		"s3_path_prefix":       cfg.S3PathPrefix,
	})
}

// handleBackupConfigUpdate mirrors `handleUpdateBackupConfig`.  When
// switching to a cloud target the handler enforces the master-password
// + unlock checks before persisting the change.
func BackupConfigUpdate(c *gin.Context) {
	a := appFromCtx(c)
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}

	// Credential gate — only enforced when switching to a cloud target.
	var probe struct {
		TargetType string `json:"target_type"`
	}
	_ = jsonUnmarshal(raw, &probe)
	isCloud := probe.TargetType == "webdav" || probe.TargetType == "s3"
	if isCloud {
		current := a.Settings.BackupConfig()
		hasCreds := false
		switch probe.TargetType {
		case "webdav":
			hasCreds = current.WebDAVPassword != ""
		case "s3":
			hasCreds = current.S3AccessKey != "" && current.S3SecretKey != ""
		}
		if !hasCreds {
			if !a.HasMasterPassword() {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "master_password_required",
					"message": "请先设置主密码才能使用云端备份",
					"action": gin.H{
						"type":   "show_modal",
						"target": "master_password",
						"params": gin.H{"mode": "setup"},
					},
				})
				return
			}
			if !a.IsUnlocked() {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "master_password_not_unlocked",
					"message": "凭证已过期，请重新解锁",
					"action": gin.H{
						"type":   "show_modal",
						"target": "master_password",
						"params": gin.H{"mode": "unlock"},
					},
				})
				return
			}
		}
	}

	if err := a.Settings.UpdateBackupConfigFromJSON(string(raw)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/backup/create
// -----------------------------------------------------------------------------

// handleBackupCreate mirrors `handleBackupCreate`.  Delegates to the
// BackupManager when one is wired into the App; otherwise responds
// with a 503-ish error.
func BackupCreate(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	if !a.Settings.BackupConfig().Enabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "backup not enabled"})
		return
	}
	cfg := a.Settings.BackupConfig()
	if cfg.TargetType == domain.BackupTargetWebDAV || cfg.TargetType == domain.BackupTargetS3 {
		credsOK := true
		switch cfg.TargetType {
		case domain.BackupTargetWebDAV:
			credsOK = cfg.WebDAVPassword != ""
		case domain.BackupTargetS3:
			credsOK = cfg.S3AccessKey != "" && cfg.S3SecretKey != ""
		}
		if !credsOK {
			actionMode := "setup"
			if a.HasMasterPassword() {
				actionMode = "unlock"
			}
			c.JSON(http.StatusOK, masterPasswordError("credentials_not_available", "凭证不可用，请先设置主密码", actionMode))
			return
		}
		if !a.IsUnlocked() {
			c.JSON(http.StatusOK, masterPasswordError("master_password_not_unlocked", "凭证已过期，请重新解锁", "unlock"))
			return
		}
	}

	name, err := a.Backup.CreateBackup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backup_path": name})
}

// -----------------------------------------------------------------------------
// /api/backup/restore
// -----------------------------------------------------------------------------

// handleBackupRestore mirrors `handleBackupRestore`.  Body: {name}.
func BackupRestore(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing name"})
		return
	}
	cfg := a.Settings.BackupConfig()
	if cfg.TargetType == domain.BackupTargetWebDAV || cfg.TargetType == domain.BackupTargetS3 {
		credsOK := true
		switch cfg.TargetType {
		case domain.BackupTargetWebDAV:
			credsOK = cfg.WebDAVPassword != ""
		case domain.BackupTargetS3:
			credsOK = cfg.S3AccessKey != "" && cfg.S3SecretKey != ""
		}
		if !credsOK {
			actionMode := "setup"
			if a.HasMasterPassword() {
				actionMode = "unlock"
			}
			c.JSON(http.StatusOK, masterPasswordError("credentials_not_available", "凭证不可用，请先设置主密码", actionMode))
			return
		}
		if !a.IsUnlocked() {
			c.JSON(http.StatusOK, masterPasswordError("master_password_not_unlocked", "凭证已过期，请重新解锁", "unlock"))
			return
		}
	}
	if err := a.Backup.RestoreFromBackup(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleBackupRestoreByName mirrors `handleBackupRestoreByName` —
// `POST /api/backup/restore/:name`.
func BackupRestoreByName(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/restore/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Backup.RestoreFromBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/backup/list
// -----------------------------------------------------------------------------

// handleBackupList mirrors `handleBackupList`.
func BackupList(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "backups": []any{}})
		return
	}
	items, err := a.Backup.ListBackups()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "backups": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "backups": items})
}

// -----------------------------------------------------------------------------
// /api/backup/info
// -----------------------------------------------------------------------------

// handleBackupInfo mirrors `handleBackupInfo`.
func BackupInfo(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"info": gin.H{
				"total_backups":    0,
				"total_size_bytes": 0,
				"oldest_backup":    nil,
				"newest_backup":    nil,
			},
		})
		return
	}
	summary, err := a.Backup.Summary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"info":    summary,
	})
}

// -----------------------------------------------------------------------------
// DELETE /api/backup/delete/:name  /  DELETE /api/backup/:id
// -----------------------------------------------------------------------------

// handleBackupDeleteByName mirrors `handleBackupDeleteByName`.
func BackupDeleteByName(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/delete/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Backup.DeleteBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleBackupDelete mirrors `handleBackupDelete` — generic
// `DELETE /api/backup/:id` form (where :id is interpreted as the
// backup name when there's no `/delete/` segment).
func BackupDelete(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	name := strings.TrimPrefix(c.Request.URL.Path, "/api/backup/")
	if !validBackupName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid backup name"})
		return
	}
	if err := a.Backup.DeleteBackup(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/backup/verify
// -----------------------------------------------------------------------------

// handleBackupVerify mirrors `handleBackupVerify`.  Calls TestConnection
// on the configured adapter (or, for the local adapter, simply
// verifies the target dir is reachable).
func BackupVerify(c *gin.Context) {
	a := appFromCtx(c)
	if a.Backup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "backup not configured"})
		return
	}
	if !a.Settings.BackupConfig().Enabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "backup not enabled"})
		return
	}
	if err := a.Backup.TestConnection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/backup/unlock  /  /api/backup/lock
// -----------------------------------------------------------------------------

// handleBackupUnlock mirrors `handleBackupUnlock`.  Body: {password}.
func BackupUnlock(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing password"})
		return
	}
	res := a.UnlockCredentials(req.Password)
	c.JSON(http.StatusOK, gin.H{
		"success":      res.Success,
		"locked_until": res.LockedUntil,
	})
}

// handleBackupLock mirrors `handleBackupLock`.
func BackupLock(c *gin.Context) {
	a := appFromCtx(c)
	a.LockCredentials()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/backup/master-password
// -----------------------------------------------------------------------------

// handleMasterPasswordGet mirrors `handleGetMasterPasswordStatus`.
func MasterPasswordGet(c *gin.Context) {
	a := appFromCtx(c)
	c.JSON(http.StatusOK, a.GetMasterPasswordStatus())
}

// handleMasterPasswordSet mirrors `handleSetMasterPassword`.  Body:
// {password}.  Minimum 4 characters (matches the Zig validator).
func MasterPasswordSet(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing password"})
		return
	}
	if len(req.Password) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password too short (minimum 4 characters)"})
		return
	}
	if err := a.SetMasterPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/auth/*
// -----------------------------------------------------------------------------

// handleAuthStatus mirrors `handleAuthStatus`.  Public route — the auth
// middleware lets it through without a token.
func AuthStatus(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.Config().Auth
	c.JSON(http.StatusOK, gin.H{
		"auth_enabled": cfg.AuthEnabled,
		"has_token":    cfg.AuthToken != "",
	})
}

// handleAuthEnable mirrors `handleAuthEnable`.  Generates a fresh token,
// persists it via SettingsManager.UpdateAuth, and returns it in the
// response so the client can save it.
func AuthEnable(c *gin.Context) {
	a := appFromCtx(c)
	token := app.GenerateToken()
	newAuth := a.Settings.Config().Auth
	newAuth.AuthEnabled = true
	newAuth.AuthToken = token
	if err := a.Settings.UpdateAuth(newAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "token": token})
}

// handleAuthDisable mirrors `handleAuthDisable`.
func AuthDisable(c *gin.Context) {
	a := appFromCtx(c)
	newAuth := a.Settings.Config().Auth
	newAuth.AuthEnabled = false
	if err := a.Settings.UpdateAuth(newAuth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// Internals.
// -----------------------------------------------------------------------------

// mask returns "******" for non-empty secrets, "" otherwise.  Matches
// the Zig source's password-masking branch.
func mask(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

// masterPasswordError builds the standard "needs master password" JSON
// response used by `handleBackupCreate`, `handleBackupRestore`, etc.
// Mirrors `createMasterPasswordError` in std_server.zig.
func masterPasswordError(code, message, actionMode string) gin.H {
	action := gin.H{
		"type":   "show_modal",
		"target": "master_password",
	}
	if actionMode != "" {
		action["params"] = gin.H{"mode": actionMode}
	}
	return gin.H{
		"success": false,
		"error":   code,
		"message": message,
		"action":  action,
	}
}
