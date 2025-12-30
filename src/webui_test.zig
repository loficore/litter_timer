const std = @import("std");
const webui = @import("webui");

pub fn main() !void {
    std.debug.print("WebUI 测试应用启动\n", .{});

    // 创建一个新的 WebUI 窗口
    var window = webui.newWindow();
    defer webui.clean();

    // 设置 HTML 内容
    const html =
        \\<!DOCTYPE html>
        \\<html>
        \\<head>
        \\    <title>Little Timer - WebUI Test</title>
        \\    <style>
        \\        body {
        \\            font-family: Arial, sans-serif;
        \\            display: flex;
        \\            justify-content: center;
        \\            align-items: center;
        \\            height: 100vh;
        \\            margin: 0;
        \\            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        \\        }
        \\        .container {
        \\            text-align: center;
        \\            background: white;
        \\            padding: 40px;
        \\            border-radius: 15px;
        \\            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.3);
        \\        }
        \\        h1 {
        \\            color: #333;
        \\            margin: 0 0 20px 0;
        \\        }
        \\        .time {
        \\            font-size: 48px;
        \\            font-weight: bold;
        \\            color: #667eea;
        \\            font-family: 'Courier New', monospace;
        \\            margin: 20px 0;
        \\        }
        \\        button {
        \\            margin: 10px 5px;
        \\            padding: 10px 20px;
        \\            font-size: 16px;
        \\            border: none;
        \\            border-radius: 5px;
        \\            cursor: pointer;
        \\            background: #667eea;
        \\            color: white;
        \\            transition: background 0.3s;
        \\        }
        \\        button:hover {
        \\            background: #764ba2;
        \\        }
        \\    </style>
        \\</head>
        \\<body>
        \\    <div class="container">
        \\        <h1>Little Timer - WebUI</h1>
        \\        <div class="time" id="time">25:00</div>
        \\        <div>
        \\            <button onclick="alert('开始定时器')">开始</button>
        \\            <button onclick="alert('暂停定时器')">暂停</button>
        \\            <button onclick="alert('重置定时器')">重置</button>
        \\        </div>
        \\    </div>
        \\</body>
        \\</html>
    ;

    // 显示 HTML 内容
    _ = try window.show(html);

    std.debug.print("打开窗口...\n", .{});

    // 阻塞直到所有窗口关闭
    webui.wait();

    std.debug.print("WebUI 测试应用结束\n", .{});
}
