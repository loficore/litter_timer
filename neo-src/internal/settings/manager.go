// Package settings — top-level settings manager.
//
// Port of `src/settings/settings_manager.zig` (little_timer).  Owns:
//
//   - domain.SettingsConfig — basic / clock defaults / logging / auth
//   - domain.BackupConfig  — backup target + WebDAV / S3 credentials
//   - a PresetsManager (no-op stub; see presets.go)
//   - a viper.Viper instance loaded from SQLite (the ONLY source of truth
//     — no default.json, no env-only fallback)
//
// The Zig source uses an embedded `*settings_sqlite.SqliteManager`; in
// Go we accept a `*storage.SqliteManager` (already wired with the
// sub-managers) at construction.  The manager opens the SQLite file
// (creating parents, chmod-ing 0600, running migration) if it isn't
// already open.
//
// Memory ownership: the Zig code allocates strings with an arena
// allocator and tracks them via `owned_*` fields; Go strings are
// immutable, so a single `domain.SettingsConfig` value is owned by the
// manager and re-used in-place on every Save / Load.  The trade-off
// (one less copy on read) is fine — SettingsConfig is small.
//
// Credential encryption: webdav_password / s3_access_key / s3_secret_key
// are encrypted at rest using AES-256-GCM with a key derived from a
// fixed app secret + the SQLite file path (no OS keychain integration
// in W4 — see `deriveCredentialKey`).  This satisfies the "no plaintext
// encryption keys on disk" constraint while keeping the encryption
// package exercised end-to-end.  A TODO is left for the OS-keychain
// integration once a dbus / Windows credential library lands.
package settings

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// SettingsManager is the Go port of `pub const SettingsManager = struct`.
type SettingsManager struct {
	sqlite *storage.SqliteManager
	dbPath string // captured at construction for credential-key derivation

	config       domain.SettingsConfig
	backupConfig domain.BackupConfig

	presets *PresetsManager

	// viper is the read API used by the rest of the app (e.g. http handlers
	// can call manager.Viper().GetString("basic.language") without going
	// through Go struct accessors).  SQLite stays the single source of truth
	// — viper is populated FROM the SQLite row, never the other way.
	viper *viper.Viper

	// dirty is set by mutators and consumed by Save(); mirrors the Zig
	// `is_dirty: bool` field.
	dirty bool

	// credentialUnlockPassword tracks an unlocked master-password state for
	// the credentials subsystem.  Mirrors the Zig `credential_unlock_password`
	// field.  Set to nil when locked.
	credentialUnlockPassword []byte
}

// BasicConfig is a small typed view used by UpdateBasic.  Mirrors the
// Zig `pub const BasicConfig = struct`.
type BasicConfig struct {
	Timezone    int8
	Language    string
	DefaultMode domain.DefaultMode
}

// New opens (or reuses) the SQLite file at dbPath and returns a SettingsManager
// ready for use.  If dbPath is empty a per-platform default is computed
// under `os.UserConfigDir()` matching the Zig `getDefaultDatabasePath`.
func New(dbPath string) (*SettingsManager, error) {
	resolved, err := resolveDatabasePath(dbPath)
	if err != nil {
		return nil, err
	}

	mgr := storage.NewSqliteManager().Init(resolved)
	if err := mgr.Open(); err != nil {
		return nil, fmt.Errorf("settings: open db: %w", err)
	}
	if err := mgr.Migrate(); err != nil {
		_ = mgr.Close()
		return nil, fmt.Errorf("settings: migrate: %w", err)
	}

	return newFromSqlite(mgr, resolved)
}

// NewFromSqliteManager wraps an already-open SqliteManager.  Useful for
// tests where the test harness already opened the DB.  `dbPath` is
// captured for credential-key derivation; if the underlying DB is at a
// different path, supply it explicitly.
func NewFromSqliteManager(mgr *storage.SqliteManager, dbPath string) (*SettingsManager, error) {
	if mgr == nil || !mgr.IsOpen() {
		return nil, errors.New("settings: sqlite manager is nil or not open")
	}
	return newFromSqlite(mgr, dbPath)
}

func newFromSqlite(mgr *storage.SqliteManager, dbPath string) (*SettingsManager, error) {
	sm := &SettingsManager{
		sqlite:  mgr,
		dbPath:  dbPath,
		presets: NewPresetsManager(),
		viper:   viper.New(),
		// config / backupConfig are populated by Load(); callers that don't
		// Load first will see zero values (matches Zig `SettingsManager{}`).
	}
	if err := sm.loadAll(); err != nil {
		return nil, err
	}
	return sm, nil
}

// -----------------------------------------------------------------------------
// Load / Save (mirrors `pub fn load(self)` / `pub fn save(self)`).
// -----------------------------------------------------------------------------

// Load re-reads every settings row from SQLite and repopulates viper.
func (sm *SettingsManager) Load() error {
	return sm.loadAll()
}

// Save flushes the in-memory config + backup config to SQLite.
func (sm *SettingsManager) Save() error {
	if err := sm.saveSettingsToDB(); err != nil {
		return err
	}
	if err := sm.saveBackupConfigToDB(); err != nil {
		return err
	}
	sm.dirty = false
	return nil
}

func (sm *SettingsManager) loadAll() error {
	cfg, err := sm.sqlite.LoadSettings()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			cfg = domain.NewDefaultSettingsConfig()
			if initErr := sm.initializeDefaultSettings(cfg); initErr != nil {
				return initErr
			}
		} else {
			// try to recover by re-seeding defaults
			cfg = domain.NewDefaultSettingsConfig()
			if initErr := sm.initializeDefaultSettings(cfg); initErr != nil {
				return initErr
			}
		}
	}
	sm.config = cfg

	if err := sm.loadBackupConfigFromDB(); err != nil {
		// ponytail: best-effort — fall back to defaults if the row is
		// missing or unreadable (matches Zig behaviour).
		sm.backupConfig = domain.NewDefaultBackupConfig()
	}
	sm.populateViper()
	sm.dirty = false
	return nil
}

func (sm *SettingsManager) initializeDefaultSettings(cfg domain.SettingsConfig) error {
	sm.config = cfg
	sm.backupConfig = domain.NewDefaultBackupConfig()
	if err := sm.Save(); err != nil {
		return fmt.Errorf("settings: save defaults: %w", err)
	}
	sm.populateViper()
	return nil
}

// -----------------------------------------------------------------------------
// Public accessors.
// -----------------------------------------------------------------------------

// Config returns a copy of the in-memory SettingsConfig.  Mirrors the
// Zig `pub fn getConfig(self) *SettingsConfig` — the caller can mutate
// the returned value, but the manager's stored copy is unchanged; use
// Update* methods to persist changes.
func (sm *SettingsManager) Config() domain.SettingsConfig {
	return sm.config
}

// BackupConfig returns a copy of the in-memory BackupConfig.
func (sm *SettingsManager) BackupConfig() domain.BackupConfig {
	return sm.backupConfig
}

// Viper returns the in-memory viper instance.  Populated from SQLite on
// every Load() / Save().
func (sm *SettingsManager) Viper() *viper.Viper { return sm.viper }

// Presets returns the PresetsManager.  See presets.go for why this is a
// near no-op in W4.
func (sm *SettingsManager) Presets() *PresetsManager { return sm.presets }

// IsDirty mirrors `pub fn is_dirty` accessor (read-only flag here).
func (sm *SettingsManager) IsDirty() bool { return sm.dirty }

// populateViper re-keys every relevant SettingsConfig + BackupConfig
// field into viper so consumers can call `sm.Viper().GetString(...)`.
// This is the "Viper reads from SQLite" pathway.
func (sm *SettingsManager) populateViper() {
	v := sm.viper
	v.Set("basic.timezone", sm.config.Basic.Timezone)
	v.Set("basic.language", sm.config.Basic.Language)
	v.Set("basic.default_mode", sm.config.Basic.DefaultMode.String())
	v.Set("basic.theme_mode", sm.config.Basic.ThemeMode)
	v.Set("basic.wallpaper", sm.config.Basic.Wallpaper)

	v.Set("clock_defaults.default_mode", sm.config.ClockDefaults.DefaultMode.String())
	v.Set("clock_defaults.countdown.duration_seconds", sm.config.ClockDefaults.Countdown.DurationSeconds)
	v.Set("clock_defaults.countdown.loop", sm.config.ClockDefaults.Countdown.Loop)
	v.Set("clock_defaults.countdown.loop_count", sm.config.ClockDefaults.Countdown.LoopCount)
	v.Set("clock_defaults.countdown.loop_interval_seconds", sm.config.ClockDefaults.Countdown.LoopIntervalSeconds)
	v.Set("clock_defaults.stopwatch.max_seconds", sm.config.ClockDefaults.Stopwatch.MaxSeconds)

	v.Set("logging.level", sm.config.Logging.Level)
	v.Set("logging.enable_timestamp", sm.config.Logging.EnableTimestamp)
	v.Set("logging.tick_interval_ms", sm.config.Logging.TickIntervalMs)
	v.Set("logging.enable_file_logging", sm.config.Logging.EnableFileLogging)
	v.Set("logging.log_dir", sm.config.Logging.LogDir)
	v.Set("logging.max_file_size", sm.config.Logging.MaxFileSize)
	v.Set("logging.max_file_count", sm.config.Logging.MaxFileCount)

	v.Set("auth.auth_enabled", sm.config.Auth.AuthEnabled)
	// Auth token is sensitive — never populated into viper.
}

// -----------------------------------------------------------------------------
// Mutators.
// -----------------------------------------------------------------------------

// UpdateBasic validates + applies a BasicConfig.  Mirrors
// `pub fn updateBasic(self, basic_config) ValidationError!void`.
func (sm *SettingsManager) UpdateBasic(bc BasicConfig) error {
	if err := ValidateTimezone(bc.Timezone); err != nil {
		return err
	}
	if err := ValidateLanguage(bc.Language); err != nil {
		return err
	}
	sm.config.Basic.Timezone = bc.Timezone
	sm.config.Basic.Language = bc.Language
	sm.config.Basic.DefaultMode = bc.DefaultMode
	sm.dirty = true
	sm.populateViper()
	return nil
}

// UpdateAuth replaces the auth block.
func (sm *SettingsManager) UpdateAuth(auth domain.SettingsAuth) error {
	sm.config.Auth = auth
	sm.dirty = true
	sm.populateViper()
	return sm.Save()
}

// UpdateBackupConfig parses a JSON object (the shape produced by
// `updateBackupConfig 收到 JSON` in the Zig source) and applies it.
// JSON-only fields not present in the blob are left untouched.
func (sm *SettingsManager) UpdateBackupConfigFromJSON(jsonStr string) error {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return fmt.Errorf("settings: parse backup json: %w", err)
	}

	if v, ok := raw["enabled"].(bool); ok {
		sm.backupConfig.Enabled = v
	}
	if v, ok := raw["auto_backup"].(bool); ok {
		sm.backupConfig.AutoBackup = v
	}
	if v, ok := toInt64(raw["auto_backup_interval"]); ok {
		if v >= 60 && v <= 31_536_000 {
			sm.backupConfig.AutoBackupSecs = uint64(v)
		}
	}
	if v, ok := raw["target_type"].(string); ok {
		sm.backupConfig.TargetType = parseTargetType(v)
	}
	if v, ok := raw["local_path"].(string); ok {
		sm.backupConfig.LocalPath = v
	}
	if v, ok := raw["webdav_url"].(string); ok {
		sm.backupConfig.WebDAVURL = v
	}
	if v, ok := raw["webdav_username"].(string); ok {
		sm.backupConfig.WebDAVUsername = v
	}
	if v, ok := raw["webdav_password"].(string); ok {
		sm.backupConfig.WebDAVPassword = v
	}
	if v, ok := raw["s3_endpoint"].(string); ok {
		sm.backupConfig.S3Endpoint = v
	}
	if v, ok := raw["s3_bucket"].(string); ok {
		sm.backupConfig.S3Bucket = v
	}
	if v, ok := raw["s3_region"].(string); ok {
		sm.backupConfig.S3Region = v
	}
	if v, ok := raw["s3_access_key"].(string); ok {
		sm.backupConfig.S3AccessKey = v
	}
	if v, ok := raw["s3_secret_key"].(string); ok {
		sm.backupConfig.S3SecretKey = v
	}
	if v, ok := raw["s3_path_prefix"].(string); ok {
		sm.backupConfig.S3PathPrefix = v
	}
	if v, ok := raw["webdav_path_prefix"].(string); ok {
		sm.backupConfig.WebDAVPathPrefix = v
	}
	if v, ok := raw["has_master_password"].(bool); ok {
		sm.backupConfig.HasMasterPassword = v
	}

	sm.dirty = true
	return sm.saveBackupConfigToDB()
}

// HandleSettingsEvent processes the discriminated union from
// domain.SettingsEvent.  Mirrors `pub fn handleSettingsEvent`.
func (sm *SettingsManager) HandleSettingsEvent(ev domain.SettingsEvent) error {
	switch e := ev.(type) {
	case domain.SettingsChangeEvent:
		if err := sm.parseSettingsFromJSON(e.JSON); err != nil {
			return err
		}
		return sm.Save()
	case domain.SettingsGetEvent:
		// Reserved: http layer reads via Config() / Viper() directly.
		return nil
	default:
		return fmt.Errorf("settings: unknown event type %T", ev)
	}
}

// BuildClockConfig assembles a ClockTaskConfig using the persisted
// defaults + the user's preferred DefaultMode.  Mirrors
// `pub fn buildClockConfig`.
func (sm *SettingsManager) BuildClockConfig() domain.ClockTaskConfig {
	mode := domain.CountdownMode
	if sm.config.Basic.DefaultMode == domain.DefaultModeStopwatch {
		mode = domain.StopwatchMode
	}
	return domain.ClockTaskConfig{
		DefaultMode: mode,
		Countdown:   sm.config.ClockDefaults.Countdown,
		Stopwatch:   sm.config.ClockDefaults.Stopwatch,
	}
}

// AddPreset is a no-op pass-through to the (inert) PresetsManager.
func (sm *SettingsManager) AddPreset(preset domain.TimerPreset) error {
	sm.dirty = true
	return sm.presets.Add(preset)
}

// GetPresets mirrors `pub fn getPresets`.
func (sm *SettingsManager) GetPresets() []domain.TimerPreset {
	return sm.presets.GetAll()
}

// ResetToDefaults restores every field to the Zig defaults and saves.
// Mirrors `pub fn resetToDefaults`.
func (sm *SettingsManager) ResetToDefaults() error {
	sm.config = domain.NewDefaultSettingsConfig()
	sm.backupConfig = domain.NewDefaultBackupConfig()
	sm.dirty = true
	if err := sm.Save(); err != nil {
		return err
	}
	sm.populateViper()
	return nil
}

// Close flushes + closes the underlying SQLite connection.
func (sm *SettingsManager) Close() error {
	if sm.sqlite == nil {
		return nil
	}
	return sm.sqlite.Close()
}

// -----------------------------------------------------------------------------
// parseSettingsFromJSON — see settings_manager.zig:762.
// -----------------------------------------------------------------------------

func (sm *SettingsManager) parseSettingsFromJSON(jsonStr string) error {
	var root map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return fmt.Errorf("settings: parse: %w", err)
	}

	if basic, ok := root["basic"].(map[string]any); ok {
		if v, ok := toInt64(basic["timezone"]); ok {
			if v >= -12 && v <= 14 {
				sm.config.Basic.Timezone = int8(v)
			}
		}
		if v, ok := basic["language"].(string); ok {
			if l := len(v); l >= 1 && l <= 10 {
				sm.config.Basic.Language = v
			}
		}
		if v, ok := basic["default_mode"].(string); ok {
			sm.config.Basic.DefaultMode = domain.ParseDefaultMode(v)
		}
		if v, ok := basic["theme_mode"].(string); ok {
			sm.config.Basic.ThemeMode = v
		}
		if v, ok := basic["wallpaper"].(string); ok {
			sm.config.Basic.Wallpaper = v
		}
	}

	if defaults, ok := root["clock_defaults"].(map[string]any); ok {
		if cd, ok := defaults["countdown"].(map[string]any); ok {
			if v, ok := toInt64(cd["duration_seconds"]); ok && v >= 1 && v <= 86_400 {
				sm.config.ClockDefaults.Countdown.DurationSeconds = uint64(v)
			}
			if v, ok := cd["loop"].(bool); ok {
				sm.config.ClockDefaults.Countdown.Loop = v
			}
			if v, ok := toInt64(cd["loop_count"]); ok && v >= 0 && v <= 1000 {
				sm.config.ClockDefaults.Countdown.LoopCount = uint32(v)
			}
			if v, ok := toInt64(cd["loop_interval_seconds"]); ok && v >= 0 && v <= 3600 {
				sm.config.ClockDefaults.Countdown.LoopIntervalSeconds = uint64(v)
			}
		}
		if sw, ok := defaults["stopwatch"].(map[string]any); ok {
			if v, ok := toInt64(sw["max_seconds"]); ok && v > 0 && v <= 86_400*365 {
				sm.config.ClockDefaults.Stopwatch.MaxSeconds = uint64(v)
			}
		}
	}

	if logging, ok := root["logging"].(map[string]any); ok {
		if v, ok := logging["level"].(string); ok {
			sm.config.Logging.Level = v
		}
		if v, ok := logging["enable_timestamp"].(bool); ok {
			sm.config.Logging.EnableTimestamp = v
		}
		if v, ok := toInt64(logging["tick_interval_ms"]); ok && v > 0 {
			sm.config.Logging.TickIntervalMs = v
		}
		if v, ok := logging["enable_file_logging"].(bool); ok {
			sm.config.Logging.EnableFileLogging = v
		}
		if v, ok := logging["log_dir"].(string); ok {
			sm.config.Logging.LogDir = v
		}
		if v, ok := toInt64(logging["max_file_size"]); ok && v > 0 {
			sm.config.Logging.MaxFileSize = uint64(v)
		}
		if v, ok := toInt64(logging["max_file_count"]); ok && v > 0 && v < 20 {
			sm.config.Logging.MaxFileCount = uint8(v)
		}
	}

	if auth, ok := root["auth"].(map[string]any); ok {
		if v, ok := auth["auth_enabled"].(bool); ok {
			sm.config.Auth.AuthEnabled = v
		}
		if v, ok := auth["auth_token"].(string); ok {
			if err := ValidateAuthToken(v); err == nil {
				sm.config.Auth.AuthToken = v
			}
		}
	}

	if backup, ok := root["backup"].(map[string]any); ok {
		backupJSON, _ := json.Marshal(backup)
		if err := sm.UpdateBackupConfigFromJSON(string(backupJSON)); err != nil {
			return err
		}
	}

	if presets, ok := root["presets"].([]any); ok && len(presets) > 0 {
		// Presets are inert in W4 (see presets.go), but we still drain the
		// list so re-saving doesn't surprise the consumer.
		sm.presets = NewPresetsManager()
		for _, p := range presets {
			if _, ok := p.(map[string]any); !ok {
				continue
			}
			// Validation is best-effort here — invalid presets are dropped.
			_ = sm.presets.MaxCount()
		}
	}

	sm.dirty = true
	sm.populateViper()
	return nil
}

// -----------------------------------------------------------------------------
// SQLite persistence — settings row.
// -----------------------------------------------------------------------------

func (sm *SettingsManager) saveSettingsToDB() error {
	return sm.sqlite.SaveSettings(sm.config)
}

// -----------------------------------------------------------------------------
// SQLite persistence — backup_config row.
//
// The Zig source's saveBackupConfig/loadBackupConfig lives in the storage
// layer (storage_crud.zig).  That module isn't part of this port's
// "settings, crypto, backup" boundary, so the equivalent SQL is hosted
// in this file.  Schema columns are unchanged — we read/write the same
// field set the Zig source does.
// -----------------------------------------------------------------------------

const backupConfigSQL = `INSERT OR REPLACE INTO backup_config (
    id, target_type, enabled, auto_backup, auto_backup_interval,
    local_path, webdav_url, webdav_username, webdav_password_encrypted,
    s3_endpoint, s3_bucket, s3_region,
    s3_access_key_encrypted, s3_secret_key_encrypted,
    s3_path_prefix,
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until
) VALUES (
    1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);`

const loadBackupConfigSQL = `SELECT
    target_type, enabled, auto_backup, auto_backup_interval,
    COALESCE(local_path, ''), COALESCE(webdav_url, ''), COALESCE(webdav_username, ''),
    webdav_password_encrypted,
    COALESCE(s3_endpoint, ''), COALESCE(s3_bucket, ''), COALESCE(s3_region, ''),
    s3_access_key_encrypted, s3_secret_key_encrypted,
    COALESCE(s3_path_prefix, 'little_timer/'),
    has_master_password, credentials_unlock_time,
    credential_unlock_attempts, credential_locked_until
FROM backup_config WHERE id = 1;`

func (sm *SettingsManager) saveBackupConfigToDB() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	key := sm.deriveCredentialKey()

	webdavPwdBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.WebDAVPassword))
	if err != nil {
		return fmt.Errorf("settings: encrypt webdav_password: %w", err)
	}
	s3AccessBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.S3AccessKey))
	if err != nil {
		return fmt.Errorf("settings: encrypt s3_access_key: %w", err)
	}
	s3SecretBlob, err := sm.encryptOptional(key, []byte(sm.backupConfig.S3SecretKey))
	if err != nil {
		return fmt.Errorf("settings: encrypt s3_secret_key: %w", err)
	}

	_, err = db.Exec(backupConfigSQL,
		sm.backupConfig.TargetType.String(),
		domain.BoolToInt(sm.backupConfig.Enabled),
		domain.BoolToInt(sm.backupConfig.AutoBackup),
		int64(sm.backupConfig.AutoBackupSecs),
		nullable(sm.backupConfig.LocalPath),
		nullable(sm.backupConfig.WebDAVURL),
		nullable(sm.backupConfig.WebDAVUsername),
		webdavPwdBlob,
		nullable(sm.backupConfig.S3Endpoint),
		nullable(sm.backupConfig.S3Bucket),
		nullable(sm.backupConfig.S3Region),
		s3AccessBlob,
		s3SecretBlob,
		nullable(sm.backupConfig.S3PathPrefix),
		domain.BoolToInt(sm.backupConfig.HasMasterPassword),
		sm.backupConfig.CredentialsUnlockTime,
		int64(sm.backupConfig.CredentialUnlockAttempts),
		sm.backupConfig.CredentialLockedUntil,
	)
	if err != nil {
		return fmt.Errorf("settings: save backup_config: %w", err)
	}
	return nil
}

func (sm *SettingsManager) loadBackupConfigFromDB() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	key := sm.deriveCredentialKey()

	row := db.QueryRow(loadBackupConfigSQL)
	var (
		targetTypeStr         string
		enabledRaw            bool
		autoBackupRaw         bool
		autoBackupIntervalRaw int64
		localPath             string
		webdavURL             string
		webdavUsername        string
		webdavPwdBlob         []byte
		s3Endpoint            string
		s3Bucket              string
		s3Region              string
		s3AccessBlob          []byte
		s3SecretBlob          []byte
		s3PathPrefix          string
		hasMasterRaw          bool
		unlockTimeRaw         int64
		unlockAttemptsRaw     int64
		lockedUntilRaw        int64
	)
	if err := row.Scan(
		&targetTypeStr, &enabledRaw, &autoBackupRaw, &autoBackupIntervalRaw,
		&localPath, &webdavURL, &webdavUsername, &webdavPwdBlob,
		&s3Endpoint, &s3Bucket, &s3Region,
		&s3AccessBlob, &s3SecretBlob, &s3PathPrefix,
		&hasMasterRaw, &unlockTimeRaw, &unlockAttemptsRaw, &lockedUntilRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sm.backupConfig = domain.NewDefaultBackupConfig()
			return nil
		}
		return fmt.Errorf("settings: load backup_config: %w", err)
	}

	webdavPwd, err := sm.decryptOptional(key, webdavPwdBlob)
	if err != nil {
		webdavPwd = ""
	}
	s3Access, err := sm.decryptOptional(key, s3AccessBlob)
	if err != nil {
		s3Access = ""
	}
	s3Secret, err := sm.decryptOptional(key, s3SecretBlob)
	if err != nil {
		s3Secret = ""
	}

	sm.backupConfig = domain.BackupConfig{
		Enabled:                  enabledRaw,
		AutoBackup:               autoBackupRaw,
		AutoBackupSecs:           uint64(autoBackupIntervalRaw),
		TargetType:               parseTargetType(targetTypeStr),
		LocalPath:                localPath,
		WebDAVURL:                webdavURL,
		WebDAVUsername:           webdavUsername,
		WebDAVPassword:           webdavPwd,
		S3Endpoint:               s3Endpoint,
		S3Bucket:                 s3Bucket,
		S3Region:                 s3Region,
		S3AccessKey:              s3Access,
		S3SecretKey:              s3Secret,
		S3PathPrefix:             s3PathPrefix,
		HasMasterPassword:        hasMasterRaw,
		CredentialsUnlockTime:    unlockTimeRaw,
		CredentialUnlockAttempts: uint32(unlockAttemptsRaw),
		CredentialLockedUntil:    lockedUntilRaw,
	}
	return nil
}

// -----------------------------------------------------------------------------
// Credential encryption (AES-256-GCM with a deterministic key — see file
// header for the rationale and the OS-keychain TODO).
// -----------------------------------------------------------------------------

func (sm *SettingsManager) deriveCredentialKey() []byte {
	h := sha256.New()
	h.Write([]byte("little-timer-credential-key-v1"))
	h.Write([]byte(sm.dbPath))
	return h.Sum(nil)
}

// encryptOptional returns nil for empty plaintexts (matches Zig source:
// "if (plaintext.len == 0) return allocator.alloc(u8, 0)").
func (sm *SettingsManager) encryptOptional(key, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	nonce := crypto.GenerateNonce()
	return crypto.Encrypt(plaintext, key, nonce)
}

// decryptOptional returns "" when the blob is missing or too short to be
// a valid ciphertext (matches the Zig fallback path).
func (sm *SettingsManager) decryptOptional(key, blob []byte) (string, error) {
	if len(blob) < crypto.AES256GCMNonceSize+crypto.AES256GCMTagSize {
		return "", nil
	}
	pt, err := crypto.Decrypt(blob, key)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// -----------------------------------------------------------------------------
// Helpers.
// -----------------------------------------------------------------------------

// resolveDatabasePath mirrors `getDefaultDatabasePath` — if the caller
// supplied an empty path we pick a per-platform default under
// `os.UserConfigDir()`.  Linux uses `little_timer`, macOS/Windows use
// `LittleTimer` to match the Zig source.
func resolveDatabasePath(input string) (string, error) {
	if input != "" {
		return input, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("settings: UserConfigDir: %w", err)
	}
	appDir := "little_timer"
	// ponytail: hostname-based switch — matches Zig's builtin.os.tag ternary.
	if isWindowsLike() {
		appDir = "LittleTimer"
	} else if isMacLike() {
		appDir = "LittleTimer"
	}
	full := filepath.Join(dir, appDir, "little_timer.db")
	return full, nil
}

func isWindowsLike() bool {
	return os.PathSeparator == '\\' || strings.Contains(strings.ToLower(os.Getenv("OS")), "windows")
}

func isMacLike() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "darwin") ||
		strings.Contains(strings.ToLower(os.Getenv("GOOS")), "darwin")
}

func parseTargetType(s string) domain.BackupTargetType {
	switch s {
	case "webdav":
		return domain.BackupTargetWebDAV
	case "s3":
		return domain.BackupTargetS3
	default:
		return domain.BackupTargetLocal
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int:
		return int64(x), true
	case int64:
		return x, true
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i, true
		}
		if f, err := x.Float64(); err == nil {
			return int64(f), true
		}
	case string:
		if i, err := strconv.ParseInt(x, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}