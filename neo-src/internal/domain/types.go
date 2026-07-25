// Package domain holds the core types and business-logic primitives of
// little-timer. This file is the Go port of the Zig module
// `src/core/interface.zig`.
//
// Mapping rules used here:
//
//   - Zig `enum`           → Go `type X int` with `iota` constants.
//   - Zig `union(enum)`    → Go sealed interface (marker method) with concrete
//     struct variants.
//   - Zig `struct { x = d }` → Go struct with field tags + a `New…` constructor
//     that applies the Zig defaults.
//   - Zig `[]const u8`     → Go `string` (immutable, no allocator needed).
//   - Zig `u64`/`i64`/`u32`→ Go `uint64`/`int64`/`uint32` (matched bit widths).
package domain

import "time"

var now = time.Now

// TodayString returns today's date in the user's timezone offset from UTC.
func TodayString(offsetHours int8) string {
	return formatDateAtOffset(now(), offsetHours)
}

func formatDateAtOffset(base time.Time, offsetHours int8) string {
	return base.UTC().Add(time.Duration(offsetHours) * time.Hour).Format("2006-01-02")
}

// -----------------------------------------------------------------------------
// Time-unit constants (seconds).
// -----------------------------------------------------------------------------

const (
	Second = 1
	Minute = 60
	Hour   = 3600
	Day    = 86400
	Year   = 31536000
)

// -----------------------------------------------------------------------------
// Defaults — mirror DEFAULT_* constants from interface.zig.
// -----------------------------------------------------------------------------

const (
	DefaultWorkDurationSeconds        = 25 * Minute // 1500 s (pomodoro work)
	DefaultRestDurationSeconds        = 5 * Minute  // 300 s
	DefaultMaxStopwatchSeconds        = 24 * Hour   // 86400 s
	DefaultMaxDurationSeconds         = Day         // 86400 s
	DefaultMaxYearSeconds             = 365 * Year  // ~1 year
	DefaultTickIntervalMs             = 1000
	DefaultAutoSaveIntervalMs         = 5000
	MinTickIntervalMs                 = 100
	MaxTickIntervalMs                 = 5000
	DefaultMaxLogFileSize      uint64 = 10 * 1024 * 1024

	DefaultColor       = "#6366f1"
	DefaultGoalSeconds = 1500
)

// -----------------------------------------------------------------------------
// ModeEnum — replaces `pub const ModeEnumT = enum { COUNTDOWN_MODE,
// STOPWATCH_MODE };` from interface.zig.
// -----------------------------------------------------------------------------

// ModeEnum is the high-level mode the clock is currently running in.
type ModeEnum int

const (
	CountdownMode ModeEnum = iota
	StopwatchMode
)

// String renders a stable label for logs/diagnostics.
func (m ModeEnum) String() string {
	switch m {
	case CountdownMode:
		return "countdown"
	case StopwatchMode:
		return "stopwatch"
	default:
		return "unknown"
	}
}

// DefaultMode is the persisted "preferred mode" stored in SettingsConfig.
// Mirrors `pub const DefaultMode = enum { countdown, stopwatch };` in
// interface.zig (string-valued in Zig because it serialises to JSON; Go
// serialises via `String()` if a JSON tag is added later).
type DefaultMode int

const (
	DefaultModeCountdown DefaultMode = iota
	DefaultModeStopwatch
)

func (d DefaultMode) String() string {
	switch d {
	case DefaultModeCountdown:
		return "countdown"
	case DefaultModeStopwatch:
		return "stopwatch"
	default:
		return "unknown"
	}
}

// ParseDefaultMode converts a string to DefaultMode, defaulting to countdown.
func ParseDefaultMode(s string) DefaultMode {
	if s == "stopwatch" {
		return DefaultModeStopwatch
	}
	return DefaultModeCountdown
}

// -----------------------------------------------------------------------------
// ClockTaskConfig — port of `pub const ClockTaskConfig = struct { … }`.
// -----------------------------------------------------------------------------

// CountdownConfig is the configuration block for the countdown (timer) mode.
type CountdownConfig struct {
	DurationSeconds     uint64 `json:"duration_seconds"`
	Loop                bool   `json:"loop"`
	LoopIntervalSeconds uint64 `json:"loop_interval_seconds"`
	LoopCount           uint32 `json:"loop_count"`
}

// NewDefaultCountdownConfig returns the pomodoro-style default: 25 min, no loop.
func NewDefaultCountdownConfig() CountdownConfig {
	return CountdownConfig{
		DurationSeconds:     DefaultWorkDurationSeconds,
		Loop:                false,
		LoopIntervalSeconds: 0,
		LoopCount:           0,
	}
}

// StopwatchConfig is the configuration block for the stopwatch mode.
type StopwatchConfig struct {
	MaxSeconds uint64 `json:"max_seconds"`
}

// NewDefaultStopwatchConfig returns the default 24-hour cap.
func NewDefaultStopwatchConfig() StopwatchConfig {
	return StopwatchConfig{
		MaxSeconds: DefaultMaxStopwatchSeconds,
	}
}

// ClockTaskConfig is the union of countdown + stopwatch config plus a default
// mode — stored alongside presets and settings.
type ClockTaskConfig struct {
	DefaultMode ModeEnum        `json:"default_mode"`
	Countdown   CountdownConfig `json:"countdown"`
	Stopwatch   StopwatchConfig `json:"stopwatch"`
}

// NewDefaultClockTaskConfig returns a config pre-populated with the Zig
// defaults: countdown mode, 25 min work, 24 h stopwatch cap.
func NewDefaultClockTaskConfig() ClockTaskConfig {
	return ClockTaskConfig{
		DefaultMode: CountdownMode,
		Countdown:   NewDefaultCountdownConfig(),
		Stopwatch:   NewDefaultStopwatchConfig(),
	}
}

// -----------------------------------------------------------------------------
// ClockEvent — sealed-interface discriminated union (was a Zig tagged union).
// -----------------------------------------------------------------------------

// ClockEvent is the sum type of every event the clock accepts. In Zig this was
// `pub const ClockEvent = union(enum) { tick: i64, … };`; in Go the marker
// method `isClockEvent()` seals the interface to the concrete variants below
// (and to no others). Adding a new variant means writing a new struct that
// implements the marker.
type ClockEvent interface {
	isClockEvent()
}

// TickEvent advances both countdown and stopwatch states by DeltaMs.
type TickEvent struct {
	DeltaMs int64
}

func (TickEvent) isClockEvent() {}

// UserStartTimerEvent starts (or resumes) the clock.
type UserStartTimerEvent struct{}

func (UserStartTimerEvent) isClockEvent() {}

// UserPauseTimerEvent pauses the clock.
type UserPauseTimerEvent struct{}

func (UserPauseTimerEvent) isClockEvent() {}

// UserResetTimerEvent restores the initial configuration.
type UserResetTimerEvent struct{}

func (UserResetTimerEvent) isClockEvent() {}

// UserFinishTimerEvent freezes the clock and marks it as finished (for stats).
type UserFinishTimerEvent struct{}

func (UserFinishTimerEvent) isClockEvent() {}

// UserChangeModeEvent switches modes with hard-coded defaults (see
// ClockManager.handleEvent for the reset semantics).
type UserChangeModeEvent struct {
	Mode ModeEnum
}

func (UserChangeModeEvent) isClockEvent() {}

// UserChangeConfigEvent replaces the current configuration wholesale.
type UserChangeConfigEvent struct {
	Config ClockTaskConfig
}

func (UserChangeConfigEvent) isClockEvent() {}

// -----------------------------------------------------------------------------
// TimerPreset — port of `pub const TimerPreset = struct { … };`.
// -----------------------------------------------------------------------------

// TimerPreset is a saved, named configuration the user can recall.
type TimerPreset struct {
	Name   string          `json:"name"`
	Mode   ModeEnum        `json:"mode"`
	Config ClockTaskConfig `json:"config"`
}

// -----------------------------------------------------------------------------
// SettingsConfig — port of `pub const SettingsConfig = struct { … };`.
// -----------------------------------------------------------------------------

// SettingsBasic captures the basic user preferences.
type SettingsBasic struct {
	Timezone    int8        `json:"timezone"` // hours east of UTC, default 8 (CN)
	Language    string      `json:"language"` // e.g. "ZH"
	DefaultMode DefaultMode `json:"default_mode"`
	ThemeMode   string      `json:"theme_mode"` // "dark" | "light" | ...
	Wallpaper   string      `json:"wallpaper"`  // global wallpaper path/URL
}

func NewDefaultSettingsBasic() SettingsBasic {
	return SettingsBasic{
		Timezone:    8,
		Language:    "ZH",
		DefaultMode: DefaultModeCountdown,
		ThemeMode:   "dark",
		Wallpaper:   "",
	}
}

// SettingsLogging captures the logging + perf tuning knobs.
type SettingsLogging struct {
	Level             string `json:"level"`
	EnableTimestamp   bool   `json:"enable_timestamp"`
	TickIntervalMs    int64  `json:"tick_interval_ms"`
	EnableFileLogging bool   `json:"enable_file_logging"`
	LogDir            string `json:"log_dir"`
	MaxFileSize       uint64 `json:"max_file_size"`
	MaxFileCount      uint8  `json:"max_file_count"`
}

func NewDefaultSettingsLogging() SettingsLogging {
	return SettingsLogging{
		Level:             "INFO",
		EnableTimestamp:   true,
		TickIntervalMs:    1000,
		EnableFileLogging: true,
		LogDir:            "",
		MaxFileSize:       DefaultMaxLogFileSize,
		MaxFileCount:      5,
	}
}

// SettingsAuth captures the bearer-token auth toggle.
type SettingsAuth struct {
	AuthEnabled bool   `json:"auth_enabled"`
	AuthToken   string `json:"auth_token"`
}

func NewDefaultSettingsAuth() SettingsAuth {
	return SettingsAuth{
		AuthEnabled: false,
		AuthToken:   "",
	}
}

// SettingsConfig is the top-level persisted application configuration.
type SettingsConfig struct {
	Basic         SettingsBasic   `json:"basic"`
	ClockDefaults ClockTaskConfig `json:"clock_defaults"`
	Logging       SettingsLogging `json:"logging"`
	Auth          SettingsAuth    `json:"auth"`
}

// NewDefaultSettingsConfig returns a fully-populated SettingsConfig that
// matches the Zig defaults. The clock_defaults block uses `New…` to apply
// nested defaults too.
func NewDefaultSettingsConfig() SettingsConfig {
	return SettingsConfig{
		Basic:         NewDefaultSettingsBasic(),
		ClockDefaults: NewDefaultClockTaskConfig(),
		Logging:       NewDefaultSettingsLogging(),
		Auth:          NewDefaultSettingsAuth(),
	}
}

// -----------------------------------------------------------------------------
// SettingsEvent — secondary union, ported for parity with interface.zig.
// The HTTP layer consumes JSON payloads; this just keeps the shape available.
// -----------------------------------------------------------------------------

// SettingsGetEvent asks the settings layer to serialise the current config
// into the supplied buffer slot (no-op in Go — http layer handles it).
type SettingsGetEvent struct {
	// Slot is a logical "where to write" hint — in Go the http layer reads
	// directly from the store, so this is reserved for future use.
	Slot string
}

func (SettingsGetEvent) isSettingsEvent() {}

// SettingsChangeEvent carries a JSON-encoded config to apply.
type SettingsChangeEvent struct {
	JSON string
}

func (SettingsChangeEvent) isSettingsEvent() {}

// SettingsEvent is the discriminated union over settings-layer events.
type SettingsEvent interface {
	isSettingsEvent()
}

// -----------------------------------------------------------------------------
// EventType — top-level dispatcher between clock and settings streams.
// -----------------------------------------------------------------------------

// EventType is the discriminated union used by the cross-cutting event bus.
type EventType interface {
	isEventType()
}

func (ClockEventWrapper) isEventType()    {}
func (SettingsEventWrapper) isEventType() {}

// ClockEventWrapper carries a ClockEvent through the bus.
type ClockEventWrapper struct {
	Event ClockEvent
}

// SettingsEventWrapper carries a SettingsEvent through the bus.
type SettingsEventWrapper struct {
	Event SettingsEvent
}

// -----------------------------------------------------------------------------
// Backup target / info / config — ported verbatim from interface.zig.
// -----------------------------------------------------------------------------

// BackupTargetType selects which destination a backup uses.
type BackupTargetType int

const (
	BackupTargetLocal BackupTargetType = iota
	BackupTargetWebDAV
	BackupTargetS3
)

func (b BackupTargetType) String() string {
	switch b {
	case BackupTargetLocal:
		return "local"
	case BackupTargetWebDAV:
		return "webdav"
	case BackupTargetS3:
		return "s3"
	default:
		return "unknown"
	}
}

// UnlockResult is returned by a credentials unlock attempt.
type UnlockResult struct {
	Success     bool  `json:"success"`
	LockedUntil int64 `json:"locked_until"`
}

// MasterPasswordStatus summarises the credentials subsystem state.
type MasterPasswordStatus struct {
	HasPassword bool  `json:"has_password"`
	Unlocked    bool  `json:"unlocked"`
	LockedUntil int64 `json:"locked_until"`
	UnlockTime  int64 `json:"unlock_time"`
}

// ApiAction is the cross-layer UI trigger (port of `pub const ApiAction =
// union(enum)`). Kept here so downstream packages can share the contract;
// modal params are stored as a free-form map for now.
type ApiAction struct {
	ShowModal *ApiShowModal `json:"show_modal,omitempty"`
}

// ApiShowModal is the modal-show variant of ApiAction.
type ApiShowModal struct {
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

// BackupConfig is the persisted backup configuration. All WebDAV / S3 fields
// are present (even when unused by the active target) so a single struct
// round-trips through JSON without dropping user input.
type BackupConfig struct {
	Enabled        bool             `json:"enabled"`
	AutoBackup     bool             `json:"auto_backup"`
	AutoBackupSecs uint64           `json:"auto_backup_interval"` // seconds
	TargetType     BackupTargetType `json:"target_type"`

	// Local-only.
	LocalPath string `json:"local_path"`

	// WebDAV-only.
	WebDAVURL        string `json:"webdav_url"`
	WebDAVUsername   string `json:"webdav_username"`
	WebDAVPassword   string `json:"webdav_password"` // encrypted at rest
	WebDAVPathPrefix string `json:"webdav_path_prefix"`

	// S3-only.
	S3Endpoint   string `json:"s3_endpoint"`
	S3Bucket     string `json:"s3_bucket"`
	S3Region     string `json:"s3_region"`
	S3AccessKey  string `json:"s3_access_key"`
	S3SecretKey  string `json:"s3_secret_key"` // encrypted at rest
	S3PathPrefix string `json:"s3_path_prefix"`

	// Credentials.
	HasMasterPassword        bool   `json:"has_master_password"`
	CredentialsUnlockTime    int64  `json:"credentials_unlock_time"`
	CredentialUnlockAttempts uint32 `json:"credential_unlock_attempts"`
	CredentialLockedUntil    int64  `json:"credential_locked_until"`
}

// NewDefaultBackupConfig returns the Zig defaults: disabled, local target,
// empty paths.
func NewDefaultBackupConfig() BackupConfig {
	return BackupConfig{
		Enabled:                  false,
		AutoBackup:               false,
		AutoBackupSecs:           Day,
		TargetType:               BackupTargetLocal,
		LocalPath:                "",
		WebDAVURL:                "",
		WebDAVUsername:           "",
		WebDAVPassword:           "",
		WebDAVPathPrefix:         "little_timer/",
		S3Endpoint:               "",
		S3Bucket:                 "",
		S3Region:                 "",
		S3AccessKey:              "",
		S3SecretKey:              "",
		S3PathPrefix:             "little_timer/",
		HasMasterPassword:        false,
		CredentialsUnlockTime:    0,
		CredentialUnlockAttempts: 0,
		CredentialLockedUntil:    0,
	}
}

// BackupInfo describes a single backup artifact (file listing, history, etc).
type BackupInfo struct {
	Name      string `json:"name"`
	Timestamp int64  `json:"timestamp"`
	SizeBytes uint64 `json:"size_bytes"`
}

// BoolToInt mirrors Zig's `@intFromBool`.  SQLite BOOLEAN stores 0/1.
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// -----------------------------------------------------------------------------
// Time helpers — keep wall-clock semantics identical to the Zig port.
// -----------------------------------------------------------------------------

// NowMs returns wall-clock milliseconds since the Unix epoch. The Zig
// reference uses `std.time.nanoTimestamp() / 1_000_000`; Go's `UnixMilli()`
// is the equivalent.
func NowMs() int64 { return time.Now().UnixMilli() }
