//! 时钟模块
const std = @import("std");
const interface = @import("interface.zig");

// 重新导出接口模块的公共类型
pub const ClockInterfaceT = interface.ClockInterfaceT;
pub const ClockEvent = interface.ClockEvent;
pub const ModeEnumT = interface.ModeEnumT;
pub const ClockInterfaceUnion = interface.ClockInterfaceUnion;
pub const ClockInterfaceData = interface.ClockInterfaceData;

// 导出配置类型
pub const ClockTaskConfigT = union(enum) {
    countdown: struct {
        duration_seconds: u64 = 25 * 60, //倒计时秒
        loop: bool = false, //是否循环倒计时
        loop_interval_break_seconds: u64 = 0, //循环间隔休息秒数,暂时不实现
        loop_count: u32 = 0, //循环次数,0表示无限循环，暂时不实现
    },

    stopwatch: struct {
        max_seconds: u64, //正计时秒数
    },

    world_clock: struct {
        timezone: i8 = 8, // 默认东八区
    },
};

const CountdownState = struct {
    remaining_ms: i64, // 剩余毫秒数
    is_paused: bool = true, // 是否暂停
    is_finished: bool = false,

    // 逻辑：更新时间
    pub fn tick(self: *CountdownState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;
        self.remaining_ms -= delta_ms;
        if (self.remaining_ms <= 0) {
            self.remaining_ms = 0;
            self.is_finished = true;
        }
    }

    pub fn pasue(self: *CountdownState) void {
        self.is_paused = true;
    }
};

const StopwatchState = struct {
    esplased_ms: i64,
    max_ms: i64,
    is_paused: bool = true, // 是否暂停
    is_finished: bool = false,

    pub fn tick(self: *StopwatchState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;
        self.esplased_ms += delta_ms;
        if (self.esplased_ms > self.max_ms) {
            self.esplased_ms = self.max_ms;
            self.is_finished = true;
        }
    }
};

// 以上类型从 interface.zig 重新导出
const ClockState = union(ModeEnumT) {
    COUNTDOWN_MODE: CountdownState,
    STOPWATCH_MODE: StopwatchState,
    WORLD_CLOCK_MODE: void,
};

pub const ClockManager = struct {
    state: ClockState,
    last_tick_time: i64 = 0,
    display_data: ClockInterfaceData = undefined, // 复用的显示数据

    pub fn init(clock_config: ClockTaskConfigT) ClockManager {
        const state = switch (clock_config) {
            .countdown => ClockState{
                .COUNTDOWN_MODE = CountdownState{
                    .remaining_ms = @as(i64, @intCast(clock_config.countdown.duration_seconds * 1000)),
                    .is_paused = true,
                },
            },
            .stopwatch => ClockState{
                .STOPWATCH_MODE = StopwatchState{
                    .esplased_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                    .max_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                },
            },
            .world_clock => ClockState{
                .WORLD_CLOCK_MODE = {},
            },
        };
        return .{
            .state = state,
        };
    }

    pub fn handleEvent(self: *ClockManager, event: ClockEvent) void {
        switch (event) {
            .tick => {
                self.OnTick(event);
            },
            .user_press_pause => {
                // 切换暂停状态
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        self.state.COUNTDOWN_MODE.is_paused = !self.state.COUNTDOWN_MODE.is_paused;
                    },
                    .STOPWATCH_MODE => {
                        // TODO: 实现正计时暂停逻辑
                        self.state.STOPWATCH_MODE.is_paused = !self.state.STOPWATCH_MODE.is_paused;
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持暂停
                    },
                }
            },
            .user_set_duration => {
                // TODO: 实现设置时长逻辑
            },
            .system_low_battery => {
                // TODO: 处理电量低事件
            },
        }
    }

    fn OnTick(self: *ClockManager, tick: ClockEvent) void {
        switch (self.state) {
            .COUNTDOWN_MODE => {
                self.state.COUNTDOWN_MODE.tick(tick.tick);
            },
            .STOPWATCH_MODE => {
                // TODO: 实现正计时逻辑
                self.state.STOPWATCH_MODE.tick(tick.tick);
            },
            .WORLD_CLOCK_MODE => {
                // 世界时钟不需要 tick
            },
        }
    }

    /// 对外部更新数据的函数
    /// 返回内部数据的指针，无需分配和释放
    pub fn update(self: *ClockManager) *ClockInterfaceT {
        switch (self.state) {
            .COUNTDOWN_MODE => {
                self.display_data.mode = ModeEnumT.COUNTDOWN_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .countdown = .{
                        .remaining_ms = self.state.COUNTDOWN_MODE.remaining_ms,
                        .is_paused = self.state.COUNTDOWN_MODE.is_paused,
                        .is_finished = self.state.COUNTDOWN_MODE.is_finished,
                    },
                };
            },
            .STOPWATCH_MODE => {
                self.display_data.mode = ModeEnumT.STOPWATCH_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .stopwatch = .{
                        .esplased_ms = self.state.STOPWATCH_MODE.esplased_ms,
                        .max_ms = self.state.STOPWATCH_MODE.max_ms,
                        .is_paused = self.state.STOPWATCH_MODE.is_paused,
                        .is_finished = self.state.STOPWATCH_MODE.is_finished,
                    },
                };
            },
            .WORLD_CLOCK_MODE => {
                self.display_data.mode = ModeEnumT.WORLD_CLOCK_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .worldclock = .{
                        .timezone = 8,
                        .time = 0,
                    },
                };
            },
        }
        return @ptrCast(&self.display_data);
    }
};
