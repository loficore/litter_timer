const gtk = @cImport({
    @cInclude("gtk/gtk.h");
});

const app = @import("app.zig");

// 需要导入 std 来处理错误
const std = @import("std");

const ClockInterfaceT = @import("clock.zig").ClockInterfaceT;
const ModeEnumT = @import("clock.zig").ModeEnumT;
const UserEventT = @import("clock.zig").ClockEvent;

// 全局变量：用于在 GTK 回调中访问（因为 GTK 是 C API）
var global_time_string: [9:0]u8 = .{ '0', '0', ':', '0', '0', ':', '0', '0', 0 };
var global_on_user_event: ?*const fn (UserEventT) void = null;
var global_clock_label: ?*gtk.GtkLabel = null;
var global_windows_manager: ?*WindowsManager = null;

const Constants = struct {
    pub const APP_ID = "com.example.LittleTimer";
    pub const windowsAttributes = struct {
        pub const width: u16 = 300;
        pub const height: u16 = 200;
        pub const title: [*:0]const u8 = "Little Timer";
    };
};

/// 窗口管理器 - 负责 GTK UI 显示和用户事件收集
pub const WindowsManager = struct {
    application: ?*gtk.GtkApplication, // 实例字段（不是静态！）
    clock_label: ?*gtk.GtkLabel = null, // 保存时钟标签的指针，用于更新显示

    /// 初始化 UI（创建 GTK 应用，但不启动主循环）
    pub fn init(self: *WindowsManager, on_user_event_param: ?*const fn (UserEventT) void, main_app: *app.MainApplication) !void {
        const temp_string = "00:00:00";
        // 复制初始时间字符串
        std.mem.copyForwards(u8, &global_time_string, temp_string);

        // 保存回调函数到全局变量（供 GTK C 回调使用）
        global_on_user_event = on_user_event_param;

        // 创建 GTK 应用程序实例
        // APP_ID 是应用程序的唯一标识符，采用反向域名格式
        // G_APPLICATION_DEFAULT_FLAGS 表示使用默认的应用程序标志
        self.application = gtk.gtk_application_new(
            Constants.APP_ID,
            gtk.G_APPLICATION_DEFAULT_FLAGS,
        );

        // 如果应用程序创建失败，返回错误
        if (self.application == null) {
            std.debug.print("Failed to create GTK Application\n", .{});
            return error.FailedToCreateApplication;
        }

        // 连接应用程序的 "activate" 信号
        // 当应用程序启动后，onActivate 函数会被自动调用
        _ = gtk.g_signal_connect_data(
            self.application, // 信号源：应用程序
            "activate", // 信号名称：激活事件
            @ptrCast(&onActivate), // 回调函数
            self, // 将 WindowsManager 指针作为用户数据传递
            null, // 销毁通知函数（不需要）
            0, // 连接标志
        );

        // 保存 WindowsManager 指针到全局变量，供 GTK 回调访问
        global_windows_manager = self;

        // 设置主应用程序指针到 GTK 应用程序的用户数据中
        // 这样在回调中可以访问 main_app 实例
        setupTimer(main_app);
    }

    // 更新显示（app 每帧调用）
    pub fn updateDisplay(self: *WindowsManager, display_data: ClockInterfaceT) void {
        // 根据 display_data 更新 global_time_string
        const hours = display_data.remaining_seconds / 3600;
        const minutes = (display_data.remaining_seconds % 3600) / 60;
        const seconds = display_data.remaining_seconds % 60;

        // 手动填充时间字符串为 "HH:MM:SS"
        const h1: u8 = @intCast((hours / 10) % 10);
        const h2: u8 = @intCast(hours % 10);
        const m1: u8 = @intCast((minutes / 10) % 10);
        const m2: u8 = @intCast(minutes % 10);
        const s1: u8 = @intCast((seconds / 10) % 10);
        const s2: u8 = @intCast(seconds % 10);

        global_time_string[0] = '0' + h1;
        global_time_string[1] = '0' + h2;
        global_time_string[2] = ':';
        global_time_string[3] = '0' + m1;
        global_time_string[4] = '0' + m2;
        global_time_string[5] = ':';
        global_time_string[6] = '0' + s1;
        global_time_string[7] = '0' + s2;
        global_time_string[8] = 0;

        // 更新 GTK 标签的显示
        if (self.clock_label != null) {
            gtk.gtk_label_set_text(@ptrCast(self.clock_label.?), &global_time_string);
        }
    }

    // 处理用户事件（如按钮点击）
    pub fn handleUserEvent(event: UserEventT) void {
        if (global_on_user_event == null) {
            std.debug.print("No user event handler defined\n", .{});
            return;
        }
        switch (event) {
            .user_press_pause => {
                // 处理用户按下暂停按钮的事件
                global_on_user_event.?(.user_press_pause);
            },
            .user_set_duration => {
                // 处理用户设置持续时间的事件
                global_on_user_event.?(.user_set_duration);
            },
            else => {},
        }
    }

    // 启动主循环（main.zig 最后调用）
    pub fn run(self: *WindowsManager) !void {
        // 运行应用程序主循环
        // 这会阻塞程序，直到窗口关闭
        // 返回值是应用程序的退出状态码（0 表示正常退出）
        if (self.application == null) {
            std.debug.print("Error: GTK Application is null\n", .{});
            return error.ApplicationNotInitialized;
        }

        const status = gtk.g_application_run(@ptrCast(self.application), 0, null);
        // 释放应用程序对象
        gtk.g_object_unref(self.application);

        // 如果状态码不为 0，表示应用程序异常退出
        if (status != 0) {
            std.debug.print("GTK Application exited with status code: {}\n", .{status});
            return error.ApplicationExitedWithError;
        }
    }
};

/// GTK "activate" 信号回调函数
/// 当 GTK 应用程序启动时调用
fn onActivate(app_ptr: ?*gtk.GtkApplication, user_data: ?*anyopaque) callconv(.c) void {
    if (app_ptr == null or user_data == null) {
        std.debug.print("Error: onActivate received null pointers\n", .{});
        return;
    }

    const self: *WindowsManager = @ptrCast(@alignCast(user_data.?));
    self.application = app_ptr;

    CreateGTKApplication(app_ptr) catch |err| {
        std.debug.print("创建窗口失败: {}\n", .{err});
        // 关闭应用程序
        gtk.g_application_quit(@ptrCast(app_ptr));
    };
}

fn onPauseButtonClicked(button: ?*gtk.GtkButton, user_data: ?*anyopaque) callconv(.c) void {
    _ = button; // 不使用按钮参数
    _ = user_data; // 不使用用户数据

    const event_handler = global_on_user_event;

    if (event_handler != null) {
        event_handler.?(.user_press_pause);
    }
}

/// 按钮点击回调函数
/// 当用户点击按钮时，这个函数会被调用
/// - **param** : **button**: 被点击的按钮控件
/// - **param** : **user_data**: 传递给回调函数的用户数据（这里不使用）
fn onButtonClicked(button: ?*gtk.GtkButton, user_data: ?*anyopaque) callconv(.c) void {
    _ = button; // 不使用 button 参数
    _ = user_data; // 不使用用户数据

    // 创建一个简单的对话框窗口
    // GTK_WINDOW_TOPLEVEL 表示这是一个顶级窗口
    const dialog_window = gtk.gtk_window_new();

    // 设置窗口标题
    gtk.gtk_window_set_title(@ptrCast(dialog_window), "提示");

    // 设置窗口大小
    gtk.gtk_window_set_default_size(@ptrCast(dialog_window), 250, 100);

    // 创建一个垂直盒子来放置消息和按钮
    const dialog_vbox = gtk.gtk_box_new(gtk.GTK_ORIENTATION_VERTICAL, 10);
    gtk.gtk_widget_set_margin_start(dialog_vbox, 20);
    gtk.gtk_widget_set_margin_end(dialog_vbox, 20);
    gtk.gtk_widget_set_margin_top(dialog_vbox, 20);
    gtk.gtk_widget_set_margin_bottom(dialog_vbox, 20);

    // 创建一个标签显示消息
    const message_label = gtk.gtk_label_new("hello, world");

    // 创建关闭按钮
    const close_button = gtk.gtk_button_new_with_label("关闭");

    // 连接关闭按钮的点击事件，使窗口关闭
    // 我们使用 G_OBJECT 宏将对话框窗口转换为 GObject
    _ = gtk.g_signal_connect_data(
        close_button,
        "clicked",
        @ptrCast(&onCloseDialogClicked),
        dialog_window,
        null,
        0,
    );

    // 将标签和按钮添加到垂直盒子
    gtk.gtk_box_append(@ptrCast(dialog_vbox), message_label);
    gtk.gtk_box_append(@ptrCast(dialog_vbox), close_button);

    // 将盒子设置为窗口内容
    gtk.gtk_window_set_child(@ptrCast(dialog_window), dialog_vbox);

    // 显示对话框窗口
    gtk.gtk_window_present(@ptrCast(dialog_window));
}

/// 对话框关闭按钮回调函数
/// 点击关闭按钮时关闭对话框窗口
fn onCloseDialogClicked(button: ?*gtk.GtkButton, user_data: ?*anyopaque) callconv(.c) void {
    _ = button; // 不使用按钮参数

    // user_data 是对话框窗口指针
    const dialog_window: ?*gtk.GtkWindow = @ptrCast(@alignCast(user_data));

    // 关闭窗口
    gtk.gtk_window_close(dialog_window);
}

/// 创建并显示 GTK 应用程序窗口
/// 这个函数设置窗口属性，创建 UI 布局，并显示窗口
/// @return: 如果创建窗口失败，返回错误
/// 否则返回 void
pub fn CreateGTKApplication(gtk_app: ?*gtk.GtkApplication) !void {
    // 创建应用程序窗口
    const window = gtk.gtk_application_window_new(gtk_app.?);

    // 设置窗口属性
    gtk.gtk_window_set_title(@ptrCast(window), Constants.windowsAttributes.title);
    gtk.gtk_window_set_default_size(@ptrCast(window), Constants.windowsAttributes.width, Constants.windowsAttributes.height);

    // ========== 创建 UI 布局 ==========

    // 创建一个垂直盒子容器，用于纵向排列控件
    // spacing: 10 表示子控件之间的间距为 10 像素
    const vbox = gtk.gtk_box_new(gtk.GTK_ORIENTATION_VERTICAL, 10);
    // 设置盒子的边距，让内容距离窗口边缘有 20 像素的空白
    gtk.gtk_widget_set_margin_start(vbox, 20);
    gtk.gtk_widget_set_margin_end(vbox, 20);
    gtk.gtk_widget_set_margin_top(vbox, 20);
    gtk.gtk_widget_set_margin_bottom(vbox, 20);

    // 创建文本标签，显示 "简易时钟"
    const label = gtk.gtk_label_new("简易时钟");

    //创建一个文本标签，用于显示时钟
    const clock_label = gtk.gtk_label_new(&global_time_string);

    // 保存 clock_label 的引用，方便后续更新
    if (global_windows_manager) |wm| {
        wm.clock_label = @ptrCast(clock_label);
    }

    const pause_button = gtk.gtk_button_new_with_label("暂停/继续");

    // 绑定暂停按钮的点击回调函数
    _ = gtk.g_signal_connect_data(
        pause_button,
        "clicked",
        @ptrCast(&onPauseButtonClicked),
        null,
        null,
        0,
    );

    // 将标签和按钮添加到垂直盒子中
    // 它们会按照添加顺序从上到下排列
    gtk.gtk_box_append(@ptrCast(vbox), label);
    gtk.gtk_box_append(@ptrCast(vbox), clock_label);
    gtk.gtk_box_append(@ptrCast(vbox), pause_button);

    // 将垂直盒子设置为窗口的子控件
    // 在 GTK4 中使用 gtk_window_set_child 来设置窗口内容
    gtk.gtk_window_set_child(@ptrCast(window), vbox);

    // 显示窗口及其所有子控件
    gtk.gtk_window_present(@ptrCast(window));
}

fn setupTimer(main_app: *app.MainApplication) void {
    _ = gtk.g_timeout_add(
        16, // 16ms = ~60 FPS
        timerCallback,
        @ptrCast(main_app),
    );
}

fn timerCallback(user_data: ?*anyopaque) callconv(.c) c_int {
    if (user_data == null) return 0;

    const main_app_temp: *app.MainApplication = @ptrCast(@alignCast(user_data.?));
    _ = main_app_temp.tick(16);
    return 1; // 返回 1 继续定时器
}
