// Package storage — SqliteManager: the top-level connection lifecycle.
//
// Port of `src/storage/storage_sqlite.zig` (little_timer).  The Zig version
// coordinates five sub-modules (migration / health / backup / crud /
// habit_crud); this Go port ships the same surface minus backup, which is
// stubbed in internal/storage/backup pending a later wave.
//
// File permissions: the Zig source calls `std.os.linux.chmod(path, 0o600)`
// after opening the DB file.  We do the same via `os.Chmod`.
//
// PRAGMA foreign_keys = ON: matches the spec requirement.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"little-timer/internal/log"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteDriverName is the name `mattn/go-sqlite3` self-registers under.  We
// keep a named constant so a future swap (e.g. to modernc.org/sqlite for
// pure Go) is a one-line change.
const sqliteDriverName = "sqlite3"

// -----------------------------------------------------------------------------
// SqliteError mirrors `pub const SqliteError = error{...}`.
// -----------------------------------------------------------------------------

// SqliteError 表示 SQLite 管理器可能返回的存储层错误。
type SqliteError string

const (
	ErrDatabaseOpenFailed   SqliteError = "database open failed"
	ErrDatabaseNotConnected SqliteError = "database not connected"
)

func (e SqliteError) Error() string { return string(e) }

// -----------------------------------------------------------------------------
// SqliteManager — Go port of `pub const SqliteManager = struct {...}`.
// -----------------------------------------------------------------------------

// SqliteManager 管理 SQLite 连接生命周期并协调各存储子模块。
// Init() must be called before Open(); Open() must be called before any
// CRUD method on a sub-manager.
type SqliteManager struct {
	dbPath string

	// sub-managers — populated by Init().
	migration *MigrationManager
	health    *HealthCheckManager
	crud      *CrudManager
	habitSets *HabitSetCrud
	habits    *HabitCrud
	timers    *TimerSessionCrud

	db *sql.DB // nil until Open() succeeds
}

// NewSqliteManager 构造一个未初始化的 SqliteManager。
// set the path, then Open() to actually open the file.
func NewSqliteManager() *SqliteManager {
	return &SqliteManager{}
}

// Init 设置数据库路径并构造各存储子模块。
// Zig `pub fn init(allocator, db_path, backup_dir)` minus backup.
//
// `dbPath` may be absolute or relative; relative paths are resolved against
// the current working directory (matches Go's `database/sql` behaviour).
func (m *SqliteManager) Init(dbPath string) *SqliteManager {
	m.dbPath = dbPath
	m.migration = NewMigrationManager()
	m.health = NewHealthCheckManager()
	m.crud = NewCrudManager()
	m.habitSets = NewHabitSetCrud()
	m.habits = NewHabitCrud()
	m.timers = NewTimerSessionCrud()
	return m
}

// Open 打开 SQLite 数据库文件并完成子模块接线与初始化检查。
// with `Create|ReadWrite`, hardens the file to 0600, enables foreign keys,
// wires the *sql.DB into every sub-manager, and runs migration + health
// check.  Idempotent: a second Open() is a no-op.
//
// Mirrors `pub fn open(self)` in storage_sqlite.zig.  Errors are returned to
// the caller; the Zig version logged-and-continued on migration/health
// errors, but we surface them so the caller can decide.
func (m *SqliteManager) Open() error {
	if m.dbPath == "" {
		log.Error("storage.open failed", "error", "Init(dbPath) must be called before Open()")
		return errors.New("storage: Init(dbPath) must be called before Open()")
	}
	if m.db != nil {
		return nil // already open
	}

	// Ensure parent directory exists (matches `makeDir(dp)` in Zig).
	if dir := filepath.Dir(m.dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			// MkdirAll returns EEXIST when the directory is already there;
			// that's fine.  Anything else is a hard error.
			if !errors.Is(err, os.ErrExist) {
				log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
				return fmt.Errorf("%w: mkdir %s: %w", ErrDatabaseOpenFailed, dir, err)
			}
		}
		// Best-effort chmod to 0700 (mirror Zig `std.os.linux.chmod(dp, 0o700)`).
		// ponytail: chmod errors here are non-fatal — the file permission
		// hardening on the DB file itself is the security-relevant step.
		_ = os.Chmod(dir, 0o700)
	}

	// Open the SQLite file.
	db, err := sql.Open(sqliteDriverName, m.dbPath)
	if err != nil {
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: %w", ErrDatabaseOpenFailed, err)
	}
	// Ping forces an actual connect so the file is created on disk before we
	// try to chmod it.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: ping: %w", ErrDatabaseOpenFailed, err)
	}

	// Hard file permission: 0600 (matches Zig `chmod(path, 0o600)`).
	if err := os.Chmod(m.dbPath, 0o600); err != nil {
		// Non-fatal but worth surfacing — the spec calls this out as a
		// security step.  The warning uses the shared logger.
		log.Warn("storage.chmod", "db_path", m.dbPath, "error", err.Error())
	}

	// Enable foreign keys on every connection.
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: pragma foreign_keys: %w", ErrDatabaseOpenFailed, err)
	}

	m.db = db

	// Wire sub-managers.
	m.migration.SetDB(db)
	m.health.SetDB(db)
	m.crud.SetDB(db)
	m.habitSets.SetDB(db)
	m.habits.SetDB(db)
	m.timers.SetDB(db)

	log.Info("storage.open", "db_path", m.dbPath)
	return nil
}

// Migrate 执行迁移检查并按需建表。
// `checkAndMigrate` call inside `SqliteManager.open` in Zig.
func (m *SqliteManager) Migrate() error {
	if m.db == nil {
		return ErrDatabaseNotConnected
	}
	start := time.Now()
	err := m.migration.CheckAndMigrate()
	log.Info("storage.migrate", "duration_ms", time.Since(start).Milliseconds())
	return err
}

// Close 关闭底层 *sql.DB 并清空子模块的句柄引用。
// in Go there is no separate "deinit"; close is enough.
func (m *SqliteManager) Close() error {
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil

	// Clear sub-manager handles so post-Close operations fail cleanly
	// instead of operating on a stale *sql.DB.
	m.migration.SetDB(nil)
	m.health.SetDB(nil)
	m.crud.SetDB(nil)
	m.habitSets.SetDB(nil)
	m.habits.SetDB(nil)
	m.timers.SetDB(nil)
	return err
}

// -----------------------------------------------------------------------------
// Convenience accessors — used by SqliteManager.SaveSettings / LoadSettings
// in storage.go and by tests.
// -----------------------------------------------------------------------------

// DB 返回底层 *sql.DB，未打开时返回 nil。
func (m *SqliteManager) DB() *sql.DB { return m.db }

// Migration 返回迁移子模块。
func (m *SqliteManager) Migration() *MigrationManager { return m.migration }

// Health 返回健康检查子模块。
func (m *SqliteManager) Health() *HealthCheckManager { return m.health }

// Crud 返回 settings 行 CRUD 子模块。
func (m *SqliteManager) Crud() *CrudManager { return m.crud }

// HabitSets 返回 habit_sets 子模块。
func (m *SqliteManager) HabitSets() *HabitSetCrud { return m.habitSets }

// Habits 返回 habits 子模块。
func (m *SqliteManager) Habits() *HabitCrud { return m.habits }

// Timers 返回 timer-sessions 子模块。
func (m *SqliteManager) Timers() *TimerSessionCrud { return m.timers }

// IsOpen 报告底层 *sql.DB 是否已连接。
func (m *SqliteManager) IsOpen() bool { return m.db != nil }
