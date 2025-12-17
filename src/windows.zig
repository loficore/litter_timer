const gtk = @cImport({
    @cInclude("gtk/gtk.h");
});

// GTK 应用程序指针
var app: ?*gtk.GtkApplication = undefined;

const Constants = struct {
    pub const APP_ID = "com.example.LittleTimer";
    pub const windowsAttributes = struct {
        pub const width: u16 = 300;
        pub const height: u16 = 200;
        pub const title: [*:0]const u8 = "Little Timer";
    };
};

/// 按钮点击回调函数
/// 当用户点击按钮时，这个函数会被调用
/// @param button: 被点击的按钮控件
/// @param user_data: 传递给回调函数的用户数据（这里不使用）
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
pub fn CreateGTKApplication() !void {
    // 创建应用程序窗口
    const window = gtk.gtk_application_window_new(app.?);

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

    // 创建文本标签，显示 "click the button"
    const label = gtk.gtk_label_new("click the button");

    // 创建按钮，按钮上显示 "click me"
    const button = gtk.gtk_button_new_with_label("click me");

    // 连接按钮的 "clicked" 信号到我们的回调函数
    // 当按钮被点击时，onButtonClicked 函数会被调用
    // window 作为用户数据传递给回调函数，这样我们可以在对话框中使用它
    _ = gtk.g_signal_connect_data(
        button, // 信号源：按钮
        "clicked", // 信号名称：点击事件
        @ptrCast(&onButtonClicked), // 回调函数
        window, // 用户数据：窗口指针
        null, // 销毁通知函数（不需要）
        0, // 连接标志
    );

    // 将标签和按钮添加到垂直盒子中
    // 它们会按照添加顺序从上到下排列
    gtk.gtk_box_append(@ptrCast(vbox), label);
    gtk.gtk_box_append(@ptrCast(vbox), button);

    // 将垂直盒子设置为窗口的子控件
    // 在 GTK4 中使用 gtk_window_set_child 来设置窗口内容
    gtk.gtk_window_set_child(@ptrCast(window), vbox);

    // 显示窗口及其所有子控件
    gtk.gtk_window_present(@ptrCast(window));
}

/// 应用程序激活回调函数
/// 当 GTK 应用程序启动后，这个函数会被自动调用
/// 我们在这里创建窗口和 UI
fn onActivate(application: ?*gtk.GtkApplication, user_data: ?*anyopaque) callconv(.c) void {
    _ = user_data; // 不使用用户数据

    // 将应用程序指针保存到全局变量，供 CreateGTKApplication 使用
    app = application;

    // 创建并显示窗口
    CreateGTKApplication() catch |err| {
        std.debug.print("创建窗口失败: {}\n", .{err});
    };
}

/// 运行 GTK 应用程序
/// 这是主入口函数，负责初始化 GTK 应用程序并运行主循环
/// @return: 返回应用程序的退出状态码
pub fn runGTKApplication() !void {
    // 创建 GTK 应用程序实例
    // APP_ID 是应用程序的唯一标识符，采用反向域名格式
    // G_APPLICATION_DEFAULT_FLAGS 表示使用默认的应用程序标志
    const application = gtk.gtk_application_new(
        Constants.APP_ID,
        gtk.G_APPLICATION_DEFAULT_FLAGS,
    );

    // 如果应用程序创建失败，返回错误
    if (application == null) {
        return error.FailedToCreateApplication;
    }

    // 连接应用程序的 "activate" 信号
    // 当应用程序启动后，onActivate 函数会被自动调用
    _ = gtk.g_signal_connect_data(
        application, // 信号源：应用程序
        "activate", // 信号名称：激活事件
        @ptrCast(&onActivate), // 回调函数
        null, // 用户数据（不需要）
        null, // 销毁通知函数（不需要）
        0, // 连接标志
    );

    // 运行应用程序主循环
    // 这会阻塞程序，直到窗口关闭
    // 返回值是应用程序的退出状态码（0 表示正常退出）
    const status = gtk.g_application_run(@ptrCast(application), 0, null);

    // 释放应用程序对象
    gtk.g_object_unref(application);

    // 如果状态码不为 0，表示应用程序异常退出
    if (status != 0) {
        return error.ApplicationExitedWithError;
    }
}

// 需要导入 std 来处理错误
const std = @import("std");
