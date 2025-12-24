const std = @import("std");
const app = @import("app.zig");

pub fn main() !void {
    // 1. 创建内存分配器
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // 2. 在堆上分配应用程序实例
    const main_app = try allocator.create(app.MainApplication);
    defer allocator.destroy(main_app);

    // 3. 初始化应用程序（在指针上原地初始化）
    try main_app.init(allocator);

    // 4. 设置全局指针（这样回调函数才能访问 app 实例）
    main_app.setGlobalApp();

    // 5. 运行应用程序（启动 GTK 主循环）
    try main_app.run();
}

test "simple test" {
    const gpa = std.testing.allocator;
    var list: std.ArrayList(i32) = .empty;
    defer list.deinit(gpa); // Try commenting this out and see if zig detects the memory leak!
    try list.append(gpa, 42);
    try std.testing.expectEqual(@as(i32, 42), list.pop());
}

test "fuzz example" {
    const Context = struct {
        fn testOne(context: @This(), input: []const u8) anyerror!void {
            _ = context;
            // Try passing `--fuzz` to `zig build test` and see if it manages to fail this test case!
            try std.testing.expect(!std.mem.eql(u8, "canyoufindme", input));
        }
    };
    try std.testing.fuzz(Context{}, Context.testOne, .{});
}
