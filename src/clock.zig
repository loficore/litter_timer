//! 时钟模块
const std = @import("std");

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
};

pub const ModeEnumT = enum {
    COUNTDOWN_MODE,
    STOPWATCH_MODE,
    WORLD_CLOCK_MODE,
};

const ClockState = union(ModeEnumT) {
    COUNTDOWN_MODE: CountdownState,
    STOPWATCH_MODE: void, // 待扩展
    WORLD_CLOCK_MODE: void,
};

// 导出事件类型，供 App 和 Windows 使用
pub const ClockEvent = union(enum) {
    tick: i64, // 毫秒增量
    user_press_pause: void, // 用户按了暂停
    user_set_duration: u64, // 用户修改了总时长
    system_low_battery: void, // 系统电量低（可能触发自动暂停）,暂时不实现相关的事件逻辑
};

// 导出接口类型，这是 Clock 提供给外部的显示数据
pub const ClockInterfaceT = struct {
    mode: ModeEnumT, // 当前时钟模式

    // 倒计时模式数据
    remaining_seconds: u64 = 0, // 剩余秒数

    // 状态标志
    is_paused: bool = true,
    is_finished: bool = false,
};

pub const ClockManager = struct {
    state: ClockState,
    last_tick_time: i64 = 0,

    pub fn init(clock_config: ClockTaskConfigT) ClockManager {
        const state = switch (clock_config) {
            .countdown => ClockState{
                .COUNTDOWN_MODE = CountdownState{
                    .remaining_ms = @as(i64, @intCast(clock_config.countdown.duration_seconds * 1000)),
                },
            },
            .stopwatch => ClockState{
                .STOPWATCH_MODE = {},
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
            },
            .WORLD_CLOCK_MODE => {
                // 世界时钟不需要 tick
            },
        }
    }

    pub fn update(self: *ClockManager) ClockInterfaceT {
        var display_data: ClockInterfaceT = undefined;
        switch (self.state) {
            .COUNTDOWN_MODE => {
                display_data = ClockInterfaceT{
                    .mode = ModeEnumT.COUNTDOWN_MODE,
                    .remaining_seconds = @as(u64, @intCast(@divTrunc(self.state.COUNTDOWN_MODE.remaining_ms, 1000))),
                    .is_paused = self.state.COUNTDOWN_MODE.is_paused,
                    .is_finished = self.state.COUNTDOWN_MODE.is_finished,
                };
            },
            .STOPWATCH_MODE => {
                display_data = ClockInterfaceT{
                    .mode = ModeEnumT.STOPWATCH_MODE,
                };
            },
            .WORLD_CLOCK_MODE => {
                display_data = ClockInterfaceT{
                    .mode = ModeEnumT.WORLD_CLOCK_MODE,
                };
            },
        }
        return display_data;
    }
};
