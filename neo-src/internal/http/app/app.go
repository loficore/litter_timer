// Package app hosts the App struct — the Go analogue of the Zig
// `MainApplication` that the std_server.zig handlers thread through
// global state.  Each handler receives `*App` via the Gin context's
// `MustGet("app")` slot, so handlers can reach the clock, settings,
// SQLite manager, backup manager, and master-password state without
// importing a global package-level variable.
//
// Splitting `App` into its own sub-package breaks the import cycle
// that would otherwise form (router → handlers → http).  The router
// and the handlers both import this package.
//
// Memory ownership: the Zig source uses an arena allocator and mutates
// `MainApplication` under `std.Thread.Mutex`.  In Go we use a single
// `sync.RWMutex` to mirror the lock; the rest of the state is owned by
// the underlying components (ClockManager, SettingsManager, etc.).
package app

import (
	"context"
	"path/filepath"

	"sync"
	"time"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// App 汇集 HTTP 处理器所需的应用依赖与运行时状态。
//
// One App per server process.  Constructed by the caller (currently the
// server bootstrap in `cmd/server/main.go`) and passed to `NewRouter`.
//
// `Backup` is optional — when nil, backup endpoints return 503-ish
// responses (handlers treat nil as "backup not configured").  This keeps
// the http layer independent of whether a backup manager was wired in.
type App struct {
	mu sync.RWMutex

	// Clock + settings + DB — mirrors the Zig MainApplication fields
	// of the same name.
	// Clock 管理当前计时状态。
	Clock *domain.ClockManager
	// Settings 管理应用设置。
	Settings *settings.SettingsManager
	// SQLite 管理 SQLite 连接与存储子模块。
	SQLite *storage.SqliteManager
	// Backup 管理备份操作；未配置时可为 nil。
	Backup *backup.BackupManager

	// DBPath is captured at App construction so backup handlers can
	// hand it to the adapter if needed.  Matches the Zig
	// `app.settings_manager.sqlite_db.?.*.db_path` chain.
	// DBPath 是数据库文件路径，供备份处理器使用。
	DBPath string

	// CurrentHabitID / CurrentTimerSessionID are the in-memory mirrors
	// of `app.current_habit_id` / `app.current_timer_session_id`.  The
	// Zig source resets these on `resetTimerSession`; the Go port does
	// the same.
	// CurrentHabitID 是当前计时关联的习惯 ID。
	CurrentHabitID *int64
	// CurrentTimerSessionID 是当前计时会话 ID。
	CurrentTimerSessionID *int64

	// Secrets is the in-process master-password store.  Mirrors the
	// Zig `SoftwareSecretImpl` / `SecretStorage`.  Lazily created by
	// the helper methods so callers can omit it during construction.
	secrets *crypto.SecretStorage
}

// NewApp 使用给定的依赖与默认内存状态构造一个 App。
// state.  `dbPath` is captured so handlers that need the on-disk path
// (currently only the backup handlers that build ad-hoc adapters) can
// read it without re-deriving it from the SQLite manager.
func NewApp(
	clk *domain.ClockManager,
	sm *settings.SettingsManager,
	sqlite *storage.SqliteManager,
	bm *backup.BackupManager,
	dbPath string,
) *App {
	return &App{
		Clock:    clk,
		Settings: sm,
		SQLite:   sqlite,
		Backup:   bm,
		DBPath:   dbPath,
	}
}

// -----------------------------------------------------------------------------
// Convenience mutex accessors.  The Zig source takes the lock on every
// mutating endpoint (start/pause/reset/finish); in Go we expose Lock/Unlock
// rather than hiding them behind helper methods so handlers stay explicit.
// -----------------------------------------------------------------------------

// Lock 获取 App 的写锁。
func (a *App) Lock() { a.mu.Lock() }

// Unlock 释放 App 的写锁。
func (a *App) Unlock() { a.mu.Unlock() }

// RLock 获取 App 的读锁，供 SSE 等只读消费者使用。
func (a *App) RLock() { a.mu.RLock() }

// RUnlock 释放 App 的读锁。
func (a *App) RUnlock() { a.mu.RUnlock() }

// -----------------------------------------------------------------------------
// Backup manager accessors.  The manager is rebuilt whenever the
// persisted BackupConfig changes (target switch, creds rotation) —
// there is no SetTarget mutator on BackupManager, mirroring the Zig
// source.  Handlers and Wails services read the current manager through
// BackupManager() (RLock snapshot) and call RebuildBackup (write lock)
// when the config has changed.
// -----------------------------------------------------------------------------

// BackupManager returns the current BackupManager snapshot.  May be nil
// when backup is not configured (handlers treat nil as "backup not
// configured").
func (a *App) BackupManager() *backup.BackupManager {
	a.RLock()
	defer a.RUnlock()
	return a.Backup
}

// RebuildBackup reconstructs the BackupManager from the persisted
// BackupConfig and swaps it in under the App write lock.  When
// construction fails for a cloud target (webdav/s3), backup is
// disabled for this process: the error is logged with the target
// type, a.Backup is set to nil, and the error is returned — there is
// NO silent fallback to local, because the UI/settings advertise a
// cloud target and local backups would diverge from the configured
// behavior.  For local targets only, a construction error is logged
// and falls back to a local adapter rooted in the default backup dir;
// only when that fallback also fails is a.Backup set to nil (backup
// disabled for this process).
func (a *App) RebuildBackup(ctx context.Context) error {
	backupDir := defaultBackupDir(a.DBPath)
	cfg := a.Settings.BackupConfig()
	mgr, err := backup.NewFromConfig(ctx, a.SQLite, a.DBPath, backupDir, cfg)
	if err != nil {
		if cfg.TargetType == domain.BackupTargetWebDAV || cfg.TargetType == domain.BackupTargetS3 {
			log.Error("RebuildBackup: NewFromConfig failed for cloud target, disabling backup", "target_type", cfg.TargetType.String(), "error", err.Error())
			a.Lock()
			a.Backup = nil
			a.Unlock()
			return err
		}
		log.Error("RebuildBackup: NewFromConfig failed, falling back to local", "error", err.Error())
		mgr, err = backup.NewLocal(a.SQLite, a.DBPath, backupDir)
		if err != nil {
			log.Error("RebuildBackup: NewLocal fallback failed, disabling backup", "error", err.Error())
			a.Lock()
			a.Backup = nil
			a.Unlock()
			return err
		}
	}
	a.Lock()
	a.Backup = mgr
	a.Unlock()
	return nil
}

// defaultBackupDir returns a sibling-of-DB backup directory.  Ported
// from `cmd/server/main.go`'s original defaultBackupDir (removed in
// T3 once App owned the rule).
// Bare-filename DB paths (Dir == "" or ".") resolve to `./backups`.
func defaultBackupDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return "backups"
	}
	return filepath.Join(dir, "backups")
}

// -----------------------------------------------------------------------------
// Timer session helpers — mirrors `createTimerSession`, `finishTimerSession`,
// `resetTimerSession`, `saveTimerProgress`, `loadTimerProgress`.
//
// All four assume the caller holds the App mutex (Lock or RLock) —
// they only mutate the in-memory pointers and the database, never the
// lock itself.  This matches the Zig source, where the helpers run
// under the caller's mutex with no re-acquisition.
// -----------------------------------------------------------------------------

// CreateTimerSession 插入一条 timer_sessions 行并更新内存中的当前会话指针。调用方必须持有 a.mu 写锁。
// in-memory pointers.  Caller MUST hold a.mu (write).  Mirrors
// `app.createTimerSession`.
func (a *App) CreateTimerSession(habitID *int64, mode string, work, rest, loop int64) (int64, error) {
	id, err := a.SQLite.Timers().CreateTimerSession(habitID, mode, work, rest, loop)
	if err != nil {
		return 0, err
	}
	a.CurrentTimerSessionID = &id
	a.CurrentHabitID = habitID
	return id, nil
}

// FinishTimerSession 将当前 timer_session 标记为结束并返回已用秒数。调用方必须持有 a.mu 写锁。
// returns the elapsed seconds.  Caller MUST hold a.mu (write).  Mirrors
// `app.finishTimerSession`.
func (a *App) FinishTimerSession() (int64, error) {
	sessionID := a.CurrentTimerSessionID
	if sessionID == nil {
		return 0, nil
	}
	if err := a.SQLite.Timers().FinishTimerSession(*sessionID); err != nil {
		return 0, err
	}
	state := a.Clock.Update()
	return state.GetElapsedSeconds(), nil
}

// ResetTimerSession 清除内存中的当前会话指针并删除对应的 timer_session 行。调用方必须持有 a.mu 写锁。
// current timer_session row.  Caller MUST hold a.mu (write).  Mirrors
// `app.resetTimerSession`.
func (a *App) ResetTimerSession() {
	sessionID := a.CurrentTimerSessionID
	a.CurrentTimerSessionID = nil
	a.CurrentHabitID = nil
	if sessionID != nil {
		_ = a.SQLite.Timers().DeleteTimerSession(*sessionID)
	}
}

// LoadTimerProgress 重新读取最近未结束的 timer_session 到内存指针。调用方必须持有 a.mu 写锁。
// into the in-memory pointers.  Caller MUST hold a.mu (write).  Mirrors
// `app.loadTimerProgress`.
func (a *App) LoadTimerProgress() {
	row, err := a.SQLite.Timers().GetActiveTimerSession()
	if err != nil {
		return
	}
	id := row.ID
	a.CurrentTimerSessionID = &id
	if row.HabitID != nil {
		hid := *row.HabitID
		a.CurrentHabitID = &hid
	}
}

// SaveProgressLocked 将当前时钟状态持久化到活动的 timer_session 行。调用方必须持有 a.mu 写锁。
// timer_session row.  Caller MUST hold a.mu (write).  Mirrors
// `app.saveTimerProgress`.
func (a *App) SaveProgressLocked() {
	if a.CurrentTimerSessionID == nil {
		return
	}
	state := a.Clock.Update()
	now := time.Now().Unix()
	row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
	if err != nil {
		return
	}
	pausedTotal := row.PausedTotalSeconds
	pauseStarted := row.PauseStartedAt
	isPaused := state.IsPaused()
	isRunning := !isPaused
	if isPaused && pauseStarted == nil {
		pauseStarted = &now
	} else if !isPaused && pauseStarted != nil {
		if now > *pauseStarted {
			pausedTotal += now - *pauseStarted
		}
		pauseStarted = nil
	}
	remaining := state.GetRemainingSeconds()
	_ = a.SQLite.Timers().UpdateTimerSession(
		row.ID, state.GetElapsedSeconds(), &remaining,
		pausedTotal, pauseStarted, &now,
		isRunning, isPaused, state.IsFinished(),
		row.CurrentRound, state.InRest(),
	)
}

// -----------------------------------------------------------------------------
// Master-password helpers — the Zig source plumbs these through
// `app.settings_manager.hasMasterPassword()` etc.  The Go port keeps the
// state on the SettingsManager's BackupConfig (which already carries the
// lockout fields) plus a SecretStorage for the unlock password itself.
//
// All helpers tolerate a nil Secrets (treat as "no master password set").
// -----------------------------------------------------------------------------

func (a *App) ensureSecrets() *crypto.SecretStorage {
	if a.secrets == nil {
		a.secrets = crypto.New(filepath.Join(filepath.Dir(a.DBPath), "secret.db"))
	}
	return a.secrets
}

// HasMasterPassword 返回是否已设置主密码（基于磁盘上的凭据或 BackupConfig 标志）。
// Returns true when a master-password blob exists on disk OR the
// BackupConfig row already says so.
func (a *App) HasMasterPassword() bool {
	cfg := a.Settings.BackupConfig()
	if cfg.HasMasterPassword {
		return true
	}
	return a.ensureSecrets().HasMasterPassword()
}

// IsUnlocked 返回凭据是否已解锁且锁定期已过。
// when the secrets store holds an unlocked master password AND the
// lockout window has elapsed.
func (a *App) IsUnlocked() bool {
	cfg := a.Settings.BackupConfig()
	if !a.ensureSecrets().IsLocked() && cfg.CredentialLockedUntil <= time.Now().Unix() {
		return true
	}
	return false
}

// UnlockCredentials 使用给定密码解锁凭据并返回结构化结果。
// Returns an UnlockResult JSON-friendly struct.  Always returns
// `Success: true` when no master password is set (matches Zig).
func (a *App) UnlockCredentials(password string) domain.UnlockResult {
	cfg := a.Settings.BackupConfig()
	if !a.HasMasterPassword() {
		log.Info("UnlockCredentials: no master password")
		// No master password: always succeed.
		cfg.CredentialLockedUntil = 0
		cfg.CredentialsUnlockTime = time.Now().Unix()
		_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
		return domain.UnlockResult{Success: true, LockedUntil: 0}
	}
	err := a.ensureSecrets().Unlock([]byte(password))
	if err != nil {
		log.Error("UnlockCredentials: failed", "error", err.Error())
		return domain.UnlockResult{Success: false, LockedUntil: a.ensureSecrets().LockoutUntil()}
	}
	log.Info("UnlockCredentials: success")
	cfg.CredentialLockedUntil = 0
	cfg.CredentialsUnlockTime = time.Now().Unix()
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	return domain.UnlockResult{Success: true, LockedUntil: 0}
}

// SetMasterPassword 设置主密码并同步更新 BackupConfig 中的标志。
// Persists the password via SecretStorage AND updates the BackupConfig
// flag so the on-disk row matches.
func (a *App) SetMasterPassword(password string) error {
	if len(password) < 4 {
		return errPasswordTooShort
	}
	if err := a.ensureSecrets().SetMasterPassword([]byte(password)); err != nil {
		log.Error("SetMasterPassword: failed", "error", err.Error())
		return err
	}
	log.Info("SetMasterPassword: success")
	cfg := a.Settings.BackupConfig()
	cfg.HasMasterPassword = true
	cfg.CredentialLockedUntil = 0
	return a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
}

// GetMasterPasswordStatus 返回主密码相关的状态信息。
// `app.settings_manager.getMasterPasswordStatus`.
func (a *App) GetMasterPasswordStatus() domain.MasterPasswordStatus {
	cfg := a.Settings.BackupConfig()
	return domain.MasterPasswordStatus{
		HasPassword: a.HasMasterPassword(),
		Unlocked:    a.IsUnlocked(),
		LockedUntil: cfg.CredentialLockedUntil,
		UnlockTime:  cfg.CredentialsUnlockTime,
	}
}

// LockCredentials 立即锁定凭据：将锁定期设为当前时间并清空内存中的密钥缓存。
// lockout to "now+1s" and clears the in-memory secrets cache.
func (a *App) LockCredentials() {
	log.Info("LockCredentials: success")
	cfg := a.Settings.BackupConfig()
	cfg.CredentialLockedUntil = time.Now().Unix() + 1
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	a.ensureSecrets().Lock()
}

// -----------------------------------------------------------------------------
// Auth helpers.
// -----------------------------------------------------------------------------

// GenerateToken 生成 32 字节随机令牌并以 base64 字符串形式返回。
// Mirrors Zig `crypto.generateToken` (which returned a 64-char hex
// string; base64 of 32 bytes is 44 chars — close enough for the auth
// header format and far cheaper than hex encoding).
func GenerateToken() string {
	return base64Raw(crypto.GenerateKey())
}

// -----------------------------------------------------------------------------
// Errors.
// -----------------------------------------------------------------------------

// errPasswordTooShort 在 SetMasterPassword 接收到不足 4 个字符的密码时返回。
// supplied password is shorter than 4 characters.  Mirrors the Zig
// `if (password_str.len < 4)` branch in handleSetMasterPassword.
var errPasswordTooShort = &httpError{code: "password_too_short", message: "password too short (minimum 4 characters)"}

// httpError 是供内部使用的轻量错误类型，便于在 JSON 响应中生成稳定的错误字符串。
// `error.Error()` strings in JSON responses.
type httpError struct {
	code, message string
}

func (e *httpError) Error() string { return e.message }
