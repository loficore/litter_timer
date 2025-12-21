const std = @import("std");
const windows = @import("windows.zig");
const clock = @import("clock.zig");

const MainApplication = struct {
    var windows_manager: windows.WindowsManager = undefined;
    const windows_manager_ptr: *windows.WindowsManager = undefined;
    var clock_manager: clock.ClockManager = undefined;
    const clock_manager_ptr: *clock.ClockManager = undefined;

    pub fn init(self: *MainApplication) void {
        // 初始化主应用程序逻辑

        //初始化并且运行 GTK 应用程序
        self.windows_manager = .{};
        self.windows_manager_ptr = &self.windows_manager;
        try self.windows_manager.init();
        try self.windows_manager.createWindow(self.windows_manager_ptr, null, null);

        // 初始化并且运行时钟管理器
    }

    pub fn run(self: *MainApplication) void {
        // 运行主应用程序逻辑
        try self.windows_manager_ptr.run(self.windows_manager_ptr);
    }

    /// 处理所有注册任务的事件
    /// - **param** : **self** 主应用程序实例的指针
    pub fn handleEvent(self: *MainApplication) void {}
};
