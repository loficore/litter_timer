// Package settings — input validation.
//
// Port of `src/settings/settings_validator.zig` (little_timer).  Every
// rule from the Zig source is preserved verbatim — same names, same
// range bounds, same error semantics.  The functions return a
// `ValidationError` (a typed sentinel) rather than panicking so the
// caller can choose how to surface the failure.
//
// Mapping notes:
//
//   - Zig `ValidationError` error set → Go typed-sentinel `ValidationError`
//     with `Error()` for use with `errors.Is`/`errors.As`.
//   - `safeI8FromJson`/`safeU32FromJson`/`safeU64FromJson`/`safeI64FromJson`
//     become methods on the `Validator` zero-value type so callers can
//     `validator.SafeI8FromJson(...)` without importing the package twice.
package settings

import (
	"fmt"

	"little-timer/internal/domain"
)

// ValidationError 对应 Zig 源码中的 `pub const ValidationError = error{...}`，
// 是一个类型化哨兵错误，可通过 errors.Is / errors.As 进行匹配。
type ValidationError string

const (
	ErrInvalidTimezone     ValidationError = "invalid timezone"
	ErrInvalidLanguage     ValidationError = "invalid language"
	ErrInvalidDuration     ValidationError = "invalid duration"
	ErrInvalidLoopCount    ValidationError = "invalid loop count"
	ErrInvalidLoopInterval ValidationError = "invalid loop interval"
	ErrInvalidMaxSeconds   ValidationError = "invalid max seconds"
	ErrInvalidTickInterval ValidationError = "invalid tick interval"
	ErrInvalidPresetName    ValidationError = "invalid preset name"
	ErrPresetLimitExceeded  ValidationError = "preset limit exceeded"
	ErrInvalidToken         ValidationError = "invalid auth token"
)

func (e ValidationError) Error() string { return string(e) }

// Time / interval bounds — mirror constants from interface.zig.
const (
	minTimezone = -12
	maxTimezone = 14

	minLanguageLen = 1
	maxLanguageLen = 10

	minDurationSec = uint64(1)
	maxDurationSec = domain.Day // 86400

	maxLoopCount    = uint32(1000)
	maxLoopInterval = uint64(3600)

	minMaxSeconds = uint64(1)
	maxMaxSeconds = domain.Year * 365 // DEFAULT_MAX_YEAR_SECONDS

	minTickIntervalMs = domain.MinTickIntervalMs // 100
	maxTickIntervalMs = domain.MaxTickIntervalMs // 5000

	maxPresetNameLen = 64
	maxPresetCount   = 999

	minAuthTokenLen = 32
	maxAuthTokenLen = 256
)

// Validator 是零值结构体，集中存放各类校验规则。仅用于 API 命名空间——
// 调用方使用 `Validator.ValidateTimezone(tz)` 而非包级自由函数。
type Validator struct{}

// NewValidator 返回一个 Validator 实例。状态全部由类型承载，值本身无状态，
// 保留构造函数便于未来扩展（如可插拔规则）时保持 API 兼容。
func NewValidator() Validator { return Validator{} }

// -----------------------------------------------------------------------------
// Range checks.
// -----------------------------------------------------------------------------

// ValidateTimezone 接受区间 [-12, 14] 的时区偏移。对应 Zig 中的
// `pub fn validateTimezone`。
func (Validator) ValidateTimezone(tz int8) error {
	if tz < minTimezone || tz > maxTimezone {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidTimezone, tz, minTimezone, maxTimezone)
	}
	return nil
}

// ValidateLanguage 接受长度为 1..10 的语言代码。对应 Zig 中的
// `pub fn validateLanguage`。
func (Validator) ValidateLanguage(lang string) error {
	if len := len(lang); len < minLanguageLen || len > maxLanguageLen {
		return fmt.Errorf("%w: length %d not in [%d, %d]",
			ErrInvalidLanguage, len, minLanguageLen, maxLanguageLen)
	}
	return nil
}

// ValidateDuration 接受区间 [1, 86400] 秒的时长。对应 Zig 中的
// `pub fn validateDuration`。
func (Validator) ValidateDuration(seconds uint64) error {
	if seconds < minDurationSec || seconds > maxDurationSec {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidDuration, seconds, minDurationSec, maxDurationSec)
	}
	return nil
}

// ValidateLoopCount 接受区间 [0, 1000] 的循环次数，0 表示无限循环。
func (Validator) ValidateLoopCount(count uint32) error {
	if count > maxLoopCount {
		return fmt.Errorf("%w: %d > %d",
			ErrInvalidLoopCount, count, maxLoopCount)
	}
	return nil
}

// ValidateLoopInterval 接受区间 [0, 3600] 秒的循环间隔。对应 Zig 中的
// `pub fn validateLoopInterval`。
func (Validator) ValidateLoopInterval(seconds uint64) error {
	if seconds > maxLoopInterval {
		return fmt.Errorf("%w: %d > %d",
			ErrInvalidLoopInterval, seconds, maxLoopInterval)
	}
	return nil
}

// ValidateMaxSeconds 接受区间 (0, 31_536_000] 秒的最大值。对应 Zig 中的
// `pub fn validateMaxSeconds`。
func (Validator) ValidateMaxSeconds(maxSeconds uint64) error {
	if maxSeconds < minMaxSeconds || maxSeconds > maxMaxSeconds {
		return fmt.Errorf("%w: %d not in (%d, %d]",
			ErrInvalidMaxSeconds, maxSeconds, minMaxSeconds, maxMaxSeconds)
	}
	return nil
}

// ValidateTickInterval 接受区间 [100, 5000] 毫秒的滴答间隔。
func (Validator) ValidateTickInterval(intervalMs int64) error {
	if intervalMs < minTickIntervalMs || intervalMs > maxTickIntervalMs {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidTickInterval, intervalMs, minTickIntervalMs, maxTickIntervalMs)
	}
	return nil
}

// ValidatePresetName 接受长度为 1..64 字符的预设名称。
func (Validator) ValidatePresetName(name string) error {
	if len := len(name); len == 0 || len > maxPresetNameLen {
		return fmt.Errorf("%w: length %d not in [1, %d]",
			ErrInvalidPresetName, len, maxPresetNameLen)
	}
	return nil
}

// ValidatePresetCount 接受区间 [0, 999] 的预设数量。
func (Validator) ValidatePresetCount(count int) error {
	if count > maxPresetCount {
		return fmt.Errorf("%w: %d > %d",
			ErrPresetLimitExceeded, count, maxPresetCount)
	}
	return nil
}

// ValidateAuthToken 接受长度为 [32, 256] 字符的认证令牌。
func (Validator) ValidateAuthToken(token string) error {
	if len := len(token); len < minAuthTokenLen || len > maxAuthTokenLen {
		return fmt.Errorf("%w: length %d not in [%d, %d]",
			ErrInvalidToken, len, minAuthTokenLen, maxAuthTokenLen)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Safe conversions — `safeXFromJson` ports.
// -----------------------------------------------------------------------------

// SafeI8FromJson 对应 Zig 中的 `pub fn safeI8FromJson`：当 JSON 传入的整数
// 超出 [min, max] 区间或无法放入 int8 时返回 nil。
func (Validator) SafeI8FromJson(jsonInt int64, min, max int8) *int8 {
	if jsonInt < int64(min) || jsonInt > int64(max) {
		return nil
	}
	v := int8(jsonInt)
	return &v
}

// SafeU32FromJson 对应 Zig 中的 `pub fn safeU32FromJson`：拒绝负数以及
// 大于 max 的值。
func (Validator) SafeU32FromJson(jsonInt int64, max uint32) *uint32 {
	if jsonInt < 0 || jsonInt > int64(max) {
		return nil
	}
	v := uint32(jsonInt)
	return &v
}

// SafeU64FromJson 对应 Zig 中的 `pub fn safeU64FromJson`。
func (Validator) SafeU64FromJson(jsonInt, min, max uint64) *uint64 {
	if jsonInt < min || jsonInt > max {
		return nil
	}
	return &jsonInt
}

// SafeI64FromJson 对应 Zig 中的 `pub fn safeI64FromJson`。
func (Validator) SafeI64FromJson(jsonInt, min, max int64) *int64 {
	if jsonInt < min || jsonInt > max {
		return nil
	}
	return &jsonInt
}

// -----------------------------------------------------------------------------
// Package-level convenience — so callers don't have to allocate a Validator.
// -----------------------------------------------------------------------------

var defaultValidator = NewValidator()

// ValidateTimezone 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateTimezone(tz int8) error { return defaultValidator.ValidateTimezone(tz) }

// ValidateLanguage 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateLanguage(lang string) error { return defaultValidator.ValidateLanguage(lang) }

// ValidateDuration 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateDuration(seconds uint64) error { return defaultValidator.ValidateDuration(seconds) }

// ValidateLoopCount 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateLoopCount(count uint32) error { return defaultValidator.ValidateLoopCount(count) }

// ValidateLoopInterval 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateLoopInterval(seconds uint64) error { return defaultValidator.ValidateLoopInterval(seconds) }

// ValidateMaxSeconds 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateMaxSeconds(maxSeconds uint64) error {
	return defaultValidator.ValidateMaxSeconds(maxSeconds)
}

// ValidateTickInterval 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateTickInterval(intervalMs int64) error {
	return defaultValidator.ValidateTickInterval(intervalMs)
}

// ValidatePresetName 包级便捷函数，内部转发到默认 Validator 实例。
func ValidatePresetName(name string) error { return defaultValidator.ValidatePresetName(name) }

// ValidatePresetCount 包级便捷函数，内部转发到默认 Validator 实例。
func ValidatePresetCount(count int) error { return defaultValidator.ValidatePresetCount(count) }

// ValidateAuthToken 包级便捷函数，内部转发到默认 Validator 实例。
func ValidateAuthToken(token string) error { return defaultValidator.ValidateAuthToken(token) }