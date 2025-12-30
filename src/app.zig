const std = @import("std");
const windows = @import("windows.zig");
const clock = @import("clock.zig");

/// 全局 App 实例指针，用于回调函数访问
var global_app: ?*MainApplication = null;

/// 主应用程序结构体 - 单向数据流的协调中心
///
/// 数据流向：
/// 1. Windows 发送事件 → handleUserEvent → ClockManager
/// 2. ClockManager 处理逻辑 → update → ClockInterfaceT
/// 3. ClockInterfaceT → Windows.updateDisplay → UI 显示
pub const MainApplication = struct {
    clock_manager: clock.ClockManager,
    windows_manager: windows.WindowsManager,
    allocator: std.mem.Allocator,

    /// 初始化应用程序（在指针上原地初始化）
    pub fn init(self: *MainApplication, allocator: std.mem.Allocator, clock_config_param: clock.ClockTaskConfigT) !void {
        // 1. 创建默认时钟配置（倒计时 25 分钟 - 番茄钟）
        const clock_config = clock_config_param;

        // 2. 初始化时钟管理器
        self.clock_manager = clock.ClockManager.init(clock_config);

        // 3. 初始化窗口管理器
        self.windows_manager = .{ .application = null, .clock_label = null, .extern_param = undefined };
        self.allocator = allocator;

        // 4. 设置全局 App 指针，供回调函数访问
        self.setGlobalApp();

        // 5. 初始化 GTK，传入事件处理回调
        try self.windows_manager.init(handleUserEventWrapper, .{ .ctx = self, .tick_handler = MainApplication.tick });

        // 6. 立即更新显示，显示初始时间
        const initial_display = self.clock_manager.update();
        self.windows_manager.updateDisplay(initial_display);
    }

    /// 设置全局 App 指针供回调函数使用
    /// 必须在 init() 后，run() 前调用
    pub fn setGlobalApp(self: *MainApplication) void {
        global_app = self;
    }

    /// 运行应用程序主循环
    /// 注意：目前 GTK 会接管主循环，未来可以考虑使用 GLib 的超时回调来实现 tick
    pub fn run(self: *MainApplication) !void {
        // TODO: 在 GTK 主循环中添加定时器来调用 tick()
        // 目前先启动 GTK 主循环
        try self.windows_manager.run();
    }

    /// 应用程序 tick - 更新逻辑并刷新显示
    /// 应该每帧调用一次（例如 60 FPS = 16ms）
    pub fn tick(ctx: ?*anyopaque, delta_ms: i64) void {
        // 1. 发送 tick 事件给时钟
        const self: *MainApplication = @ptrCast(@alignCast(ctx));
        self.clock_manager.handleEvent(.{ .tick = delta_ms });

        // 2. 获取时钟的显示数据（无需分配，返回内部指针）
        const display_data = self.clock_manager.update();

        // 3. 更新窗口显示
        self.windows_manager.updateDisplay(display_data);
    }

    /// 处理来自用户界面的事件（回调函数）
    /// 这是事件向上冒泡的入口点
    fn handleUserEvent(_: *MainApplication, event: clock.ClockEvent) void {
        if (global_app) |app| {
            // 将用户事件转发给时钟管理器
            app.clock_manager.handleEvent(event);

            // 立即更新显示
            const display_data = app.clock_manager.update();
            app.windows_manager.updateDisplay(display_data);
        }
    }

    /// 事件处理器包装函数 - 适配窗口管理器期望的函数签名
    fn handleUserEventWrapper(event: clock.ClockEvent) void {
        if (global_app) |app| {
            app.handleUserEvent(event);
        }
    }
};
