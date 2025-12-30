//! 公共接口模块 - clock 和 windows 都通过这个接口交互
const std = @import("std");

/// 时钟模式枚举
pub const ModeEnumT = enum {
    COUNTDOWN_MODE,
    STOPWATCH_MODE,
    WORLD_CLOCK_MODE,
};

/// 时钟导出的数据类型
pub const ClockInterfaceUnion = union(enum) {
    countdown: struct {
        remaining_ms: i64,
        is_paused: bool,
        is_finished: bool,
    },
    stopwatch: struct {
        esplased_ms: i64,
        max_ms: i64,
        is_paused: bool,
        is_finished: bool,
    },
    worldclock: struct {
        timezone: i8 = 8,
        time: i64,
    },
};

/// 私有的接口数据结构
pub const ClockInterfaceData = struct {
    mode: ModeEnumT,
    info: ClockInterfaceUnion,
};

/// 时钟接口 - 隐藏实现细节
pub const ClockInterfaceT = opaque {
    pub fn getTimeInfo(self: *ClockInterfaceT) i64 {
        const data: *ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => @divTrunc(data.info.countdown.remaining_ms, 1000),
            .STOPWATCH_MODE => @divTrunc(data.info.stopwatch.esplased_ms, 1000),
            .WORLD_CLOCK_MODE => data.info.worldclock.time,
        };
    }

    pub fn getMode(self: *ClockInterfaceT) ModeEnumT {
        const data: *ClockInterfaceData = @ptrCast(@alignCast(self));
        return data.mode;
    }
};

/// 时钟事件 - 从 windows 发送给 app/clock
pub const ClockEvent = union(enum) {
    tick: i64, // 毫秒增量
    user_press_pause: void, // 用户按了暂停
    user_set_duration: u64, // 用户修改了总时长
    system_low_battery: void, // 系统电量低
};
