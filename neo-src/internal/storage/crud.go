// Package storage — settings row CRUD.
//
// Port of `src/storage/storage_crud.zig` (little_timer), specifically the
// saveSettings / loadSettings pair.  Backup-config encryption lives in
// internal/storage/backup in this port — credential encryption needs the
// secret-storage helper that hasn't been ported yet, so we leave the
// encrypted-column handling for a later wave and keep this file focused on
// the settings round-trip.
package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"little-timer/internal/domain"
)

// CrudError 表示设置项 CRUD 操作可能返回的存储层错误。
type CrudError string

const (
	ErrSettingsNotFound   CrudError = "settings not found"
	ErrSettingsSaveFailed CrudError = "settings save failed"
	ErrQueryFailed        CrudError = "query failed"
	ErrCrudNoDatabase     CrudError = "database open failed"
)

func (e CrudError) Error() string { return string(e) }

// SettingsRow 表示 settings 表中的一行底层数据。
// higher-level domain.SettingsConfig is what callers actually pass in.
type SettingsRow struct {
	// ID 是设置行的固定主键。
	ID int64
	// Timezone 是用户时区偏移。
	Timezone int8
	// Language 是界面语言代码。
	Language string
	// DefaultMode 是默认计时模式。
	DefaultMode string
	// ThemeMode 是界面主题模式。
	ThemeMode string
	// Wallpaper 是壁纸标识或路径。
	Wallpaper string
	// DurationSeconds 是默认倒计时秒数。
	DurationSeconds int64
	// CountdownLoop 表示是否启用循环倒计时。
	CountdownLoop bool
	// CountdownLoopCount 是循环总次数，0 表示无限循环。
	CountdownLoopCount int64
	// CountdownLoopInterval 是循环间隔秒数。
	CountdownLoopInterval int64
	// StopwatchMaxSeconds 是正计时最大秒数。
	StopwatchMaxSeconds int64
	// LogLevel 是日志级别。
	LogLevel string
	// LogEnableTimestamp 表示日志是否包含时间戳。
	LogEnableTimestamp bool
	// LogTickInterval 是计时日志输出间隔。
	LogTickInterval int64
}

// CrudManager 持有 settings 表读写的 *sql.DB 句柄。
type CrudManager struct {
	db *sql.DB
}

// NewCrudManager 构造一个空的 CrudManager，调用方需在之后通过 SetDB 注入数据库句柄。
// `CrudManager.init(allocator, null)`.
func NewCrudManager() *CrudManager {
	return &CrudManager{}
}

// SetDB 为 CrudManager 注入 *sql.DB 句柄。
// Zig SqliteManager.open.
func (c *CrudManager) SetDB(db *sql.DB) { c.db = db }

// saveSettingsSQL is the UPSERT statement, byte-for-byte from
// storage_crud.zig:saveSettings.  Note the trailing space before `wallpaper`
// — preserved from the Zig source.
const saveSettingsSQL = `INSERT OR REPLACE INTO settings (id, timezone, language, default_mode, theme_mode, wallpaper, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

// SaveSettings 将 SettingsConfig 持久化到 settings 表的单行记录中。
//
// Mirrors `pub fn saveSettings(self, config)`.  Translation notes:
//
//   - Zig DefaultMode enum → Go int constant + String() lookup.
//   - Zig `bool` is stored as INTEGER 0/1 by SQLite; the column is BOOLEAN
//     NOT NULL DEFAULT 0/1 in the schema.  We write 0/1 explicitly.
func (c *CrudManager) SaveSettings(config domain.SettingsConfig) error {
	if c.db == nil {
		return ErrCrudNoDatabase
	}

	defaultModeStr := config.Basic.DefaultMode.String()
	themeMode := config.Basic.ThemeMode
	if themeMode == "" {
		themeMode = "dark" // matches schema DEFAULT 'dark' and Zig defaults.
	}
	logLevel := config.Logging.Level
	if logLevel == "" {
		logLevel = "INFO"
	}
	lang := config.Basic.Language
	if lang == "" {
		lang = "ZH"
	}

	_, err := c.db.Exec(saveSettingsSQL,
		config.Basic.Timezone,
		lang,
		defaultModeStr,
		themeMode,
		config.Basic.Wallpaper,
		int64(config.ClockDefaults.Countdown.DurationSeconds),
		boolToInt(config.ClockDefaults.Countdown.Loop),
		int64(config.ClockDefaults.Countdown.LoopCount),
		int64(config.ClockDefaults.Countdown.LoopIntervalSeconds),
		int64(config.ClockDefaults.Stopwatch.MaxSeconds),
		logLevel,
		boolToInt(config.Logging.EnableTimestamp),
		config.Logging.TickIntervalMs,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSettingsSaveFailed, err)
	}
	return nil
}

// LoadSettings 读取 settings 行并返回填充好的 SettingsConfig。
//
// Mirrors `pub fn loadSettings(self, allocator)`.  When no row exists, the
// Zig source returns `SettingsConfig{}` (zero-valued); we return
// NewDefaultSettingsConfig() instead — same effect for any consumer that
// only reads fields, and safer for callers that forget to apply defaults.
const settingsSelectSQL = `SELECT timezone, language, default_mode, theme_mode, COALESCE(wallpaper, ''), duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;`

const settingsSelectWithIDSQL = `SELECT id, timezone, language, default_mode, theme_mode, COALESCE(wallpaper, ''), duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;`

func (c *CrudManager) LoadSettings() (domain.SettingsConfig, error) {
	if c.db == nil {
		return domain.SettingsConfig{}, ErrCrudNoDatabase
	}

	var (
		timezone              int64
		language              string
		defaultModeStr        string
		themeMode             string
		wallpaper             string
		durationSeconds       int64
		countdownLoop         bool
		countdownLoopCount    int64
		countdownLoopInterval int64
		stopwatchMaxSeconds   int64
		logLevel              string
		logEnableTimestamp    bool
		logTickInterval       int64
	)
	err := c.db.QueryRow(settingsSelectSQL).Scan(
		&timezone, &language, &defaultModeStr, &themeMode, &wallpaper,
		&durationSeconds, &countdownLoop, &countdownLoopCount,
		&countdownLoopInterval, &stopwatchMaxSeconds, &logLevel,
		&logEnableTimestamp, &logTickInterval,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewDefaultSettingsConfig(), nil
		}
		return domain.SettingsConfig{}, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	return domain.SettingsConfig{
		Basic: domain.SettingsBasic{
			Timezone:    int8(timezone),
			Language:    language,
			DefaultMode: parseDefaultMode(defaultModeStr),
			ThemeMode:   themeMode,
			Wallpaper:   wallpaper,
		},
		ClockDefaults: domain.ClockTaskConfig{
			Countdown: domain.CountdownConfig{
				DurationSeconds:     uint64(durationSeconds),
				Loop:                countdownLoop,
				LoopCount:           uint32(countdownLoopCount),
				LoopIntervalSeconds: uint64(countdownLoopInterval),
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: uint64(stopwatchMaxSeconds),
			},
		},
		Logging: domain.SettingsLogging{
			Level:           logLevel,
			EnableTimestamp: logEnableTimestamp,
			TickIntervalMs:  logTickInterval,
		},
	}, nil
}

// LoadSettingsRow 返回 settings 行的原始结构体视图。
func (c *CrudManager) LoadSettingsRow() (SettingsRow, error) {
	if c.db == nil {
		return SettingsRow{}, ErrCrudNoDatabase
	}

	var row SettingsRow
	err := c.db.QueryRow(settingsSelectWithIDSQL).Scan(
		&row.ID, &row.Timezone, &row.Language, &row.DefaultMode,
		&row.ThemeMode, &row.Wallpaper, &row.DurationSeconds,
		&row.CountdownLoop, &row.CountdownLoopCount,
		&row.CountdownLoopInterval, &row.StopwatchMaxSeconds,
		&row.LogLevel, &row.LogEnableTimestamp, &row.LogTickInterval,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SettingsRow{}, ErrSettingsNotFound
		}
		return SettingsRow{}, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	return row, nil
}

// boolToInt mirrors Zig's `@intFromBool`.  SQLite BOOLEAN stores 0/1.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseDefaultMode converts the persisted TEXT back to the DefaultMode enum.
// Mirrors the Zig if/else that checked for "countdown" and assumed
// "stopwatch" otherwise.
func parseDefaultMode(s string) domain.DefaultMode {
	if s == "countdown" {
		return domain.DefaultModeCountdown
	}
	// Zig treats everything-not-"countdown" as stopwatch; preserve that.
	// The schema's CHECK also allows "world_clock" but no consumer uses it.
	return domain.DefaultModeStopwatch
}
