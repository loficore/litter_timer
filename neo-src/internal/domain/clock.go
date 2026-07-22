// Package domain — clock-related primitives.
//
// This file is the Go port of `src/core/clock.zig`. The Zig module was built
// around tagged unions and method-on-union; Go has neither, so we approximate:
//
//   - Zig `union(ModeEnumT) { COUNTDOWN_MODE, STOPWATCH_MODE }` becomes a
//     `ClockState` struct that holds pointers to the active variant plus the
//     discriminator. `Countdown()` / `Stopwatch()` expose the active variant;
//     methods like `GetTimeInfo` / `GetMode` switch internally — exactly the
//     same pattern the Zig source uses (`switch (self.*) { … }`).
//
//   - Zig `union(enum) { tick, user_start_timer, … }` (ClockEvent) is the
//     sealed interface declared in `types.go`. `handleEvent` does a Go type
//     switch over the concrete variant.
//
//   - Zig atomic `tick_count` is replaced with `sync/atomic.Uint64`.
//
//   - `std.time.nanoTimestamp()` becomes `NowMs()` (UnixMilli) — the Zig
//     source divides nanoseconds by 1_000_000, which is the same value.
package domain

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// -----------------------------------------------------------------------------
// Tick counter — port of `var tick_count: std.atomic.Value(usize)`.
// -----------------------------------------------------------------------------

// tickCount is the global tick counter shared across all ClockManager
// instances (mirrors Zig's file-scope `var tick_count`).
var tickCount atomic.Uint64

// -----------------------------------------------------------------------------
// CountdownState — port of `const CountdownState = struct { … };`.
// -----------------------------------------------------------------------------

// CountdownState 保存倒计时模式的内存运行状态。
type CountdownState struct {
	// DurationMs is the configured total duration in milliseconds.
	DurationMs uint64
	// RemainingMs is the live remaining duration; decremented by Tick().
	RemainingMs int64
	// Loop enables the pomodoro-style loop behaviour.
	Loop bool
	// LoopIntervalSeconds is the rest period between loops (seconds).
	LoopIntervalSeconds uint64
	// LoopCount is the configured loop total; 0 means infinite.
	LoopCount uint32
	// LoopRemaining is the live loop counter; 0 = infinite.
	LoopRemaining uint32
	// LoopCompleted flips true after the last configured loop runs out.
	LoopCompleted bool
	// InRest is true while the inter-loop rest period is ticking down.
	InRest bool
	// RestRemainingMs is the live rest countdown in milliseconds.
	RestRemainingMs int64
	// IsPaused gates Tick(); true means the clock is held.
	IsPaused bool
	// IsFinished flips true when RemainingMs has reached zero.
	IsFinished bool
	// StartTimeMs is the wall-clock ms when the clock last started/resumed.
	StartTimeMs int64
	// PausedMs is the cumulative wall-clock ms the clock has spent paused.
	PausedMs int64
	// ElapsedAtPause is the elapsed ms at the moment of the most recent pause.
	ElapsedAtPause int64
}

// Tick 将倒计时推进 deltaMs 毫秒。
// `CountdownState.tick(self, delta_ms)` exactly: guards on paused/finished,
// handles negative/zero deltas, runs the rest-phase counter, then the main
// counter with loop/rest rollover logic.
func (c *CountdownState) Tick(deltaMs int64) {
	if c.IsPaused || c.IsFinished {
		return
	}
	if deltaMs < 0 {
		// ponytail: warn-only in the Zig source (logs and ignores); in Go we
		// return silently to keep the domain layer I/O-free.
		return
	}
	if deltaMs == 0 {
		return
	}

	// Rest phase: only rest_remaining_ms counts down; the main counter is
	// reset when rest ends.
	if c.InRest {
		c.RestRemainingMs -= deltaMs
		if c.RestRemainingMs <= 0 {
			c.InRest = false
			c.IsFinished = false
			c.RemainingMs = int64(c.DurationMs)
		}
		return
	}

	c.RemainingMs -= deltaMs
	if c.RemainingMs <= 0 {
		c.RemainingMs = 0
		c.IsFinished = true

		if c.Loop {
			// Finite loop: decrement the remaining count and stop when zero.
			if c.LoopCount > 0 {
				if c.LoopRemaining > 0 {
					c.LoopRemaining--
				}
				if c.LoopRemaining == 0 {
					c.LoopCompleted = true
					return
				}
			}
			// Infinite loop (LoopCount == 0) or rounds still remaining:
			// either start a rest phase or reset straight into the next round.
			if c.LoopIntervalSeconds > 0 {
				c.InRest = true
				c.RestRemainingMs = int64(c.LoopIntervalSeconds * 1000)
				c.IsFinished = false
			} else {
				c.IsFinished = false
				c.RemainingMs = int64(c.DurationMs)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// StopwatchState — port of `const StopwatchState = struct { … };`.
// -----------------------------------------------------------------------------

// StopwatchState 保存正计时模式的内存运行状态。
type StopwatchState struct {
	// ElapsedMs is the live elapsed duration in milliseconds.
	ElapsedMs int64
	// MaxMs caps the stopwatch; once ElapsedMs >= MaxMs, IsFinished flips.
	MaxMs int64
	// IsPaused gates Tick().
	IsPaused bool
	// IsFinished flips true once MaxMs is reached.
	IsFinished bool
	// StartTimeMs is the wall-clock ms when the stopwatch last started.
	StartTimeMs int64
	// PausedMs is the cumulative wall-clock ms spent paused.
	PausedMs int64
	// ElapsedAtPause is the elapsed ms at the most recent pause.
	ElapsedAtPause int64
}

// Tick 将正计时推进 deltaMs 毫秒，到达 MaxMs 时封顶并标记完成。
// Mirrors the Zig `StopwatchState.tick(self, delta_ms)`.
func (s *StopwatchState) Tick(deltaMs int64) {
	if s.IsPaused || s.IsFinished {
		return
	}
	if deltaMs < 0 {
		return
	}
	if deltaMs == 0 {
		return
	}

	s.ElapsedMs += deltaMs
	if s.ElapsedMs >= s.MaxMs {
		s.ElapsedMs = s.MaxMs
		s.IsFinished = true
	}
}

// -----------------------------------------------------------------------------
// ClockState — port of `pub const ClockState = union(ModeEnumT) { … };`.
// -----------------------------------------------------------------------------

// ClockState 保存当前计时模式及其对应状态。
// Stopwatch is non-nil, matching the discriminant in Mode.
type ClockState struct {
	// Mode is the discriminant — keep in sync with whichever of Countdown or
	// Stopwatch is non-nil.
	Mode ModeEnum
	// Countdown is non-nil iff Mode == CountdownMode.
	Countdown *CountdownState
	// Stopwatch is non-nil iff Mode == StopwatchMode.
	Stopwatch *StopwatchState
}

// GetTimeInfo 返回当前秒级时间信息：倒计时为剩余秒数，正计时为已用秒数。
// (stopwatch) as whole seconds. Mirrors `ClockState.getTimeInfo`.
func (s *ClockState) GetTimeInfo() int64 {
	switch s.Mode {
	case CountdownMode:
		if s.Countdown == nil {
			return 0
		}
		return s.Countdown.RemainingMs / 1000
	case StopwatchMode:
		if s.Stopwatch == nil {
			return 0
		}
		return s.Stopwatch.ElapsedMs / 1000
	default:
		return 0
	}
}

// GetMode 返回当前激活的计时模式。
func (s *ClockState) GetMode() ModeEnum { return s.Mode }

// IsPaused 返回当前状态是否处于暂停。
func (s *ClockState) IsPaused() bool {
	switch s.Mode {
	case CountdownMode:
		return s.Countdown != nil && s.Countdown.IsPaused
	case StopwatchMode:
		return s.Stopwatch != nil && s.Stopwatch.IsPaused
	default:
		return true
	}
}

// IsFinished 返回当前状态是否已结束。
func (s *ClockState) IsFinished() bool {
	switch s.Mode {
	case CountdownMode:
		return s.Countdown != nil && s.Countdown.IsFinished
	case StopwatchMode:
		return s.Stopwatch != nil && s.Stopwatch.IsFinished
	default:
		return false
	}
}

// InRest 返回倒计时是否处于循环间的休息阶段。正计时始终为 false。
// Always false for stopwatch.
func (s *ClockState) InRest() bool {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return false
	}
	return s.Countdown.InRest
}

// GetRestRemainingTime 返回休息阶段剩余秒数。正计时始终返回 0。
// Always 0 for stopwatch.
func (s *ClockState) GetRestRemainingTime() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.RestRemainingMs / 1000
}

// GetLoopRemaining 返回实时剩余循环次数，0 表示无限循环。正计时始终返回 0。
// Always 0 for stopwatch.
func (s *ClockState) GetLoopRemaining() uint32 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.LoopRemaining
}

// GetLoopTotal 返回配置的循环总次数，0 表示无限循环。正计时始终返回 0。
func (s *ClockState) GetLoopTotal() uint32 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.LoopCount
}

// GetElapsedSeconds 计算已用时间（整秒），按 `(now - start - paused) / 1000` 公式。
// the same formula as the Zig source: `(now - start - paused) / 1000`.
func (s *ClockState) GetElapsedSeconds() int64 {
	now := NowMs()
	switch s.Mode {
	case CountdownMode:
		if s.Countdown == nil {
			return 0
		}
		c := s.Countdown
		return (now - c.StartTimeMs - c.PausedMs) / 1000
	case StopwatchMode:
		if s.Stopwatch == nil {
			return 0
		}
		w := s.Stopwatch
		return (now - w.StartTimeMs - w.PausedMs) / 1000
	default:
		return 0
	}
}

// GetRemainingSeconds 计算倒计时的实时剩余秒数，正计时返回 0。
// returns 0 for stopwatch (no notion of "remaining").
func (s *ClockState) GetRemainingSeconds() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	now := NowMs()
	c := s.Countdown
	elapsedMs := now - c.StartTimeMs - c.PausedMs
	remainingMs := int64(c.DurationMs) - elapsedMs
	return remainingMs / 1000
}

// GetCurrentRound 返回循环倒计时的当前轮次。正计时始终返回 0。
// countdown (`loop_count - loop_remaining + 1`). Always 0 for stopwatch.
func (s *ClockState) GetCurrentRound() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return int64(s.Countdown.LoopCount) - int64(s.Countdown.LoopRemaining) + 1
}

// -----------------------------------------------------------------------------
// ClockManager — port of `pub const ClockManager = struct { … };`.
// -----------------------------------------------------------------------------

// ClockManager 管理当前计时状态、复位用的初始配置快照以及事件通道。
// reset, and the event channel used for asynchronous dispatch.
//
// State is guarded by `mu` because Run() mutates it on a consumer goroutine
// while callers read it via Update() from other goroutines (http layer, UI,
// stats). Skipping the mutex leaves a real data race in production — `-race`
// flags it in the event-bus test below.
type ClockManager struct {
	mu            sync.Mutex
	state         ClockState
	initialConfig ClockTaskConfig

	// events is the async event bus. Producers send ClockEvent values to
	// Events(); Run() drains them in a separate goroutine. Tests use the
	// synchronous HandleEvent path instead to avoid races.
	events chan ClockEvent

	closeOnce sync.Once
}

// NewClockManager 根据 ClockTaskConfig 构造（但不启动）一个 ClockManager。
// Zig `ClockManager.init(clock_config)` path: it validates the duration
// against i64 overflow and falls back to safe defaults if necessary.
func NewClockManager(cfg ClockTaskConfig) *ClockManager {
	if durationOverflows(cfg.Countdown.DurationSeconds) ||
		durationOverflows(cfg.Stopwatch.MaxSeconds) {
		// ponytail: Zig logs an error and substitutes a safe config. In Go we
		// silently substitute; logging belongs to the log layer.
		cfg = ClockTaskConfig{
			Countdown: CountdownConfig{
				DurationSeconds:     25 * Minute,
				Loop:                false,
				LoopIntervalSeconds: 0,
				LoopCount:           0,
			},
			Stopwatch: StopwatchConfig{
				MaxSeconds: 24 * Hour,
			},
		}
	}
	return &ClockManager{
		state:         buildInitialState(cfg),
		initialConfig: cfg,
		events:        make(chan ClockEvent, 64),
	}
}

// Init 为兼容规范生命周期的空操作，实际构造在 NewClockManager 中完成。
// (Init/Update/Deinit). Construction happens in NewClockManager; this exists
// so callers that prefer an init-then-use pattern can call it without effect.
func (m *ClockManager) Init() {}

// Update 返回当前 ClockState 的指针，仅供只读检查使用。
// Mirrors `ClockManager.update(self) *ClockState`. The pointer is valid only
// until the next HandleEvent call; copy fields out if you need a stable view.
// Callers in concurrent contexts must hold m.mu themselves — Update does NOT
// hand back the lock with the pointer (the defer would release it on return).
func (m *ClockManager) Update() *ClockState { return &m.state }

// Deinit 关闭事件通道，使用 sync.Once 保证可安全重复调用。
// sync.Once; tests may close the channel manually without `defer Deinit()`.
func (m *ClockManager) Deinit() {
	m.closeOnce.Do(func() {
		if m.events != nil {
			close(m.events)
		}
	})
}

// Events 返回事件总线的写入端，供生产方发送 ClockEvent。
// goroutine in the http layer, user actions, etc.) push events here.
func (m *ClockManager) Events() chan<- ClockEvent { return m.events }

// Run 持续消费事件总线，直到 ctx 取消或通道关闭。
// Mirrors the spec's "HandleEvent selects on this channel" requirement. Holds
// the state mutex around each event dispatch so concurrent readers of
// Update() see consistent state.
func (m *ClockManager) Run(ctx context.Context, events <-chan ClockEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			m.mu.Lock()
			m.handleEvent(ev)
			m.mu.Unlock()
		}
	}
}

// HandleEvent 同步处理单个事件，绕过事件通道直接派发。
// Useful in tests where races against the consumer goroutine would be a
// problem, and useful in places where the producer already has the event in
// hand. Holds the state mutex for the duration of the mutation.
func (m *ClockManager) HandleEvent(ev ClockEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handleEvent(ev)
}

// -----------------------------------------------------------------------------
// Internal helpers.
// -----------------------------------------------------------------------------

// durationOverflows returns true if `durationSeconds * 1000` would overflow
// int64. The Zig source uses `std.math.maxInt(i64) / 1000`; Go's math.MaxInt64
// is the equivalent.
func durationOverflows(durationSeconds uint64) bool {
	const maxSafeDuration = uint64(math.MaxInt64) / 1000
	return durationSeconds > maxSafeDuration
}

// buildInitialState constructs the ClockState from the supplied config,
// matching Zig `initSafe`.
func buildInitialState(cfg ClockTaskConfig) ClockState {
	switch cfg.DefaultMode {
	case CountdownMode:
		durMs := cfg.Countdown.DurationSeconds * 1000
		return ClockState{
			Mode: CountdownMode,
			Countdown: &CountdownState{
				DurationMs:          durMs,
				RemainingMs:         int64(durMs),
				Loop:                cfg.Countdown.Loop,
				LoopIntervalSeconds: cfg.Countdown.LoopIntervalSeconds,
				LoopCount:           cfg.Countdown.LoopCount,
				LoopRemaining:       cfg.Countdown.LoopCount,
				IsPaused:            true,
			},
		}
	case StopwatchMode:
		maxMs := cfg.Stopwatch.MaxSeconds * 1000
		return ClockState{
			Mode: StopwatchMode,
			Stopwatch: &StopwatchState{
				ElapsedMs: 0,
				MaxMs:     int64(maxMs),
				IsPaused:  true,
			},
		}
	default:
		// ponytail: Zig's switch is exhaustive over an enum so this case is
		// unreachable; in Go we still need a fallback so the function
		// compiles. Fall back to a paused countdown at 25 min.
		durMs := uint64(25 * Minute * 1000)
		return ClockState{
			Mode: CountdownMode,
			Countdown: &CountdownState{
				DurationMs:  durMs,
				RemainingMs: int64(durMs),
				IsPaused:    true,
			},
		}
	}
}

// handleEvent dispatches a ClockEvent to the matching state mutation. The
// structure mirrors Zig `ClockManager.handleEvent(self, event)` exactly —
// same branches in the same order, same mutations, same hard-coded defaults
// on user_change_mode.
func (m *ClockManager) handleEvent(event ClockEvent) {
	switch ev := event.(type) {
	case TickEvent:
		// Bump the shared tick counter (every 10th tick in the Zig source is
		// followed by a debug log; in Go we keep just the counter bump so the
		// domain layer stays I/O-free).
		newCount := tickCount.Add(1)
		_ = newCount // placeholder for any future diagnostics
		m.onTick(ev.DeltaMs)

	case UserStartTimerEvent:
		now := NowMs()
		switch m.state.Mode {
		case CountdownMode:
			if c := m.state.Countdown; c.IsPaused {
				if c.StartTimeMs == 0 {
					c.StartTimeMs = now
				} else {
					c.StartTimeMs = now - c.ElapsedAtPause - c.PausedMs
				}
				c.IsPaused = false
			}
		case StopwatchMode:
			if w := m.state.Stopwatch; w.IsPaused {
				if w.StartTimeMs == 0 {
					w.StartTimeMs = now
				} else {
					w.StartTimeMs = now - w.ElapsedAtPause - w.PausedMs
				}
				w.IsPaused = false
			}
		}

	case UserPauseTimerEvent:
		now := NowMs()
		switch m.state.Mode {
		case CountdownMode:
			if c := m.state.Countdown; !c.IsPaused {
				c.ElapsedAtPause = now - c.StartTimeMs - c.PausedMs
				c.IsPaused = true
			}
		case StopwatchMode:
			if w := m.state.Stopwatch; !w.IsPaused {
				w.ElapsedAtPause = now - w.StartTimeMs - w.PausedMs
				w.IsPaused = true
			}
		}

	case UserResetTimerEvent:
		switch m.state.Mode {
		case CountdownMode:
			c := m.state.Countdown
			c.StartTimeMs = 0
			c.PausedMs = 0
			c.ElapsedAtPause = 0
			c.RemainingMs = int64(m.initialConfig.Countdown.DurationSeconds * 1000)
			c.LoopRemaining = m.initialConfig.Countdown.LoopCount
			c.LoopCompleted = false
			c.InRest = false
			c.RestRemainingMs = 0
			c.IsPaused = true
			c.IsFinished = false
		case StopwatchMode:
			w := m.state.Stopwatch
			w.StartTimeMs = 0
			w.PausedMs = 0
			w.ElapsedAtPause = 0
			w.ElapsedMs = 0
			w.IsPaused = true
			w.IsFinished = false
		}

	case UserFinishTimerEvent:
		switch m.state.Mode {
		case CountdownMode:
			c := m.state.Countdown
			c.IsPaused = true
			c.IsFinished = true
		case StopwatchMode:
			w := m.state.Stopwatch
			w.IsPaused = true
			w.IsFinished = true
		}

	case UserChangeModeEvent:
		// Zig ignores initial_config here and uses hard-coded defaults:
		// 25-minute countdown / 24-hour stopwatch.
		switch ev.Mode {
		case CountdownMode:
			const durSec uint64 = 25 * Minute
			durMs := durSec * 1000
			m.state = ClockState{
				Mode: CountdownMode,
				Countdown: &CountdownState{
					DurationMs:          durMs,
					RemainingMs:         int64(durMs),
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
					LoopRemaining:       0,
					LoopCompleted:       false,
					IsPaused:            true,
					StartTimeMs:         0,
					PausedMs:            0,
					ElapsedAtPause:      0,
				},
			}
			m.initialConfig = ClockTaskConfig{
				Countdown: CountdownConfig{
					DurationSeconds:     durSec,
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
				},
				Stopwatch: StopwatchConfig{
					MaxSeconds: 24 * Hour,
				},
			}
		case StopwatchMode:
			const maxSec uint64 = 24 * Hour
			maxMs := maxSec * 1000
			m.state = ClockState{
				Mode: StopwatchMode,
				Stopwatch: &StopwatchState{
					ElapsedMs:      0,
					MaxMs:          int64(maxMs),
					IsPaused:       true,
					StartTimeMs:    0,
					PausedMs:       0,
					ElapsedAtPause: 0,
				},
			}
			m.initialConfig = ClockTaskConfig{
				Countdown: CountdownConfig{
					DurationSeconds:     25 * Minute,
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
				},
				Stopwatch: StopwatchConfig{
					MaxSeconds: maxSec,
				},
			}
		}

	case UserChangeConfigEvent:
		newCfg := ev.Config
		m.state = buildInitialState(newCfg)
		m.initialConfig = newCfg

	default:
		// ponytail: sealed interface guarantees we know every variant; a panic
		// here indicates a programming error in the producer, not a runtime
		// condition we want to swallow.
		panic(fmt.Sprintf("clock: unknown ClockEvent variant %T", event))
	}
}

// onTick delegates a TickEvent delta to the active variant's Tick method.
func (m *ClockManager) onTick(deltaMs int64) {
	switch m.state.Mode {
	case CountdownMode:
		m.state.Countdown.Tick(deltaMs)
	case StopwatchMode:
		m.state.Stopwatch.Tick(deltaMs)
	}
}
