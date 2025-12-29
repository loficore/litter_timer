const std = @import("std");

// 传入 b (构建环境) 和命令数组，返回命令的输出字符串
fn pkgConfigQuery(b: *std.Build, args: []const []const u8) []const u8 {
    const result = std.process.Child.run(.{
        .allocator = b.allocator,
        .argv = args,
    }) catch {
        @panic("无法运行 pkg-config，请确保它已安装在 PATH 中");
    };

    // 检查退出状态码是否为 0
    switch (result.term) {
        .Exited => |code| {
            if (code != 0) {
                // 如果 pkg-config 报错了，我们打印出它的错误信息并崩溃
                std.debug.print("pkg-config 错误输出: {s}\n", .{result.stderr});
                std.debug.panic("pkg-config 执行失败 (错误码 {d})", .{result.term.Exited});
            }
        },
        else => std.debug.panic("pkg-config 异常终止。\n", .{}),
    }

    return std.mem.trim(u8, result.stdout, " \n\r");
}

fn readEnvFile(allocator: std.mem.Allocator) ![]u8 {
    // 1. 获取当前工作目录的句柄
    const cwd = std.fs.cwd();

    // 2. 打开文件（.{} 里可以放额外的权限设置，默认是只读）
    const file = try cwd.openFile(".env", .{});
    // 确保函数结束时关闭文件句柄
    defer file.close();

    // 3. 读取内容到内存中
    // 我们需要一个上限（比如 4096 字节），防止读取到超大文件导致崩溃
    const content = try file.readToEndAlloc(allocator, 4096);

    return content;
}

// 将 MSYS/MinGW 的 POSIX 风格路径（例如 "/mingw64/include" 或 "\mingw64\include"）
// 规范化为 Windows 绝对路径（例如 "C:\\msys64\\mingw64\\include"）。
// 仅在 Windows 目标时使用。
fn normalizeMsysPathForWindows(b: *std.Build, p: []const u8) []const u8 {
    // 处理 /mingw64/ 或 \mingw64\ 开头的路径
    if (std.mem.startsWith(u8, p, "/mingw64/") or std.mem.startsWith(u8, p, "\\mingw64\\") or std.mem.startsWith(u8, p, "\\mingw64/")) {
        // 去掉开头的 / 或 \，然后拼上 C:\msys64\
        const without_leading_slash = if (p[0] == '/' or p[0] == '\\') p[1..] else p;
        const with_prefix = b.fmt("C:\\msys64\\{s}", .{without_leading_slash});
        const buf = b.allocator.dupe(u8, with_prefix) catch @panic("内存分配失败");
        // 统一替换所有 / 为 \\
        for (buf) |*ch| {
            if (ch.* == '/') ch.* = '\\';
        }
        return buf;
    }
    // 未识别的路径保持原样（复制一份以便生命周期安全）
    return b.allocator.dupe(u8, p) catch @panic("内存分配失败");
}

// Although this function looks imperative, it does not perform the build
// directly and instead it mutates the build graph (`b`) that will be then
// executed by an external runner. The functions in `std.Build` implement a DSL
// for defining build steps and express dependencies between them, allowing the
// build runner to parallelize the build automatically (and the cache system to
// know when a step doesn't need to be re-run).
pub fn build(b: *std.Build) void {
    // Standard target options allow the person running `zig build` to choose
    // what target to build for. Here we do not override the defaults, which
    // means any target is allowed, and the default is native. Other options
    // for restricting supported target set are available.
    const target = b.standardTargetOptions(.{});
    // Standard optimization options allow the person running `zig build` to select
    // between Debug, ReleaseSafe, ReleaseFast, and ReleaseSmall. Here we do not
    // set a preferred release mode, allowing the user to decide how to optimize.
    const optimize = b.standardOptimizeOption(.{});
    // It's also possible to define more custom flags to toggle optional features
    // of this build script using `b.option()`. All defined flags (including
    // target and optimize options) will be listed when running `zig build --help`
    // in this directory.

    // This creates a module, which represents a collection of source files alongside
    // some compilation options, such as optimization mode and linked system libraries.
    // Zig modules are the preferred way of making Zig code available to consumers.
    // addModule defines a module that we intend to make available for importing
    // to our consumers. We must give it a name because a Zig package can expose
    // multiple modules and consumers will need to be able to specify which
    // module they want to access.
    const mod = b.addModule("little_timer", .{
        // The root source file is the "entry point" of this module. Users of
        // this module will only be able to access public declarations contained
        // in this file, which means that if you have declarations that you
        // intend to expose to consumers that were defined in other files part
        // of this module, you will have to make sure to re-export them from
        // the root file.
        .root_source_file = b.path("src/root.zig"),
        // Later on we'll use this module as the root module of a test executable
        // which requires us to specify a target.
        .target = target,
    });

    // Here we define an executable. An executable needs to have a root module
    // which needs to expose a `main` function. While we could add a main function
    // to the module defined above, it's sometimes preferable to split business
    // logic and the CLI into two separate modules.
    //
    // If your goal is to create a Zig library for others to use, consider if
    // it might benefit from also exposing a CLI tool. A parser library for a
    // data serialization format could also bundle a CLI syntax checker, for example.
    //
    // If instead your goal is to create an executable, consider if users might
    // be interested in also being able to embed the core functionality of your
    // program in their own executable in order to avoid the overhead involved in
    // subprocessing your CLI tool.
    //
    // If neither case applies to you, feel free to delete the declaration you
    // don't need and to put everything under a single module.

    const exe = b.addExecutable(.{
        .name = "little_timer",
        .root_module = b.createModule(.{
            // b.createModule defines a new module just like b.addModule but,
            // unlike b.addModule, it does not expose the module to consumers of
            // this package, which is why in this case we don't have to give it a name.
            .root_source_file = b.path("src/main.zig"),
            // Target and optimization levels must be explicitly wired in when
            // defining an executable or library (in the root module), and you
            // can also hardcode a specific target for an executable or library
            // definition if desireable (e.g. firmware for embedded devices).
            .target = target,
            .optimize = optimize,
            // List of modules available for import in source files part of the
            // root module.
            .imports = &.{
                // Here "little_timer" is the name you will use in your source code to
                // import this module (e.g. `@import("little_timer")`). The name is
                // repeated because you are allowed to rename your imports, which
                // can be extremely useful in case of collisions (which can happen
                // importing modules from different packages).
                .{ .name = "little_timer", .module = mod },
            },
        }),
    });

    // ========== GTK4 依赖配置 ==========
    // 1. 链接 libc (必要，因为使用了 C 库)
    exe.linkLibC();

    if (target.result.os.tag == .windows) {
        const mingw_lib = "C:\\msys64\\mingw64\\lib";

        // 2. 使用 pkg-config 获取 GTK4 的编译标志（包含头文件路径等）
        const gtk_cflags = pkgConfigQuery(b, &.{ "pkg-config", "--dont-define-prefix", "--cflags", "--libs", "gtk4" });
        var gtk_cflags_split = std.mem.tokenizeAny(u8, gtk_cflags, " ");

        while (gtk_cflags_split.next()) |raw_flag| {
            const flag = std.mem.trim(u8, raw_flag, " \n\r");
            if (flag.len < 2) continue; //忽略无效标志

            if (std.mem.startsWith(u8, flag, "-I")) {
                const include_path_raw = flag[2..];
                const include_path = normalizeMsysPathForWindows(b, include_path_raw);
                // 使用绝对路径，避免生成 "\\mingw64\\..." 这样的无效根路径
                exe.addIncludePath(.{ .cwd_relative = include_path });
            } else if (std.mem.startsWith(u8, flag, "-l")) {
                const lib_name = b.fmt("lib{s}.dll.a", .{flag[2..]});
                const lib_full_path = std.fs.path.join(b.allocator, &.{ mingw_lib, lib_name }) catch {
                    std.debug.panic("无法构建库文件路径。\n", .{});
                };
                // 直接把 .dll.a 文件喂给链接器，避开 Zig 的搜索策略问题
                exe.addObjectFile(.{ .cwd_relative = lib_full_path });
            } else if (std.mem.startsWith(u8, flag, "-L")) {
                const lib_path_raw = flag[2..];
                const lib_path = normalizeMsysPathForWindows(b, lib_path_raw);
                exe.addLibraryPath(.{ .cwd_relative = lib_path });
            }
        }
    } else {
        // Linux 和其他 Unix 平台：使用 pkg-config
        exe.linkSystemLibrary("gtk4");
        exe.linkSystemLibrary("glib-2.0");
    }
    // ==================================

    // This declares intent for the executable to be installed into the
    // install prefix when running `zig build` (i.e. when executing the default
    // step). By default the install prefix is `zig-out/` but can be overridden
    // by passing `--prefix` or `-p`.
    b.installArtifact(exe);

    // This creates a top level step. Top level steps have a name and can be
    // invoked by name when running `zig build` (e.g. `zig build run`).
    // This will evaluate the `run` step rather than the default step.
    // For a top level step to actually do something, it must depend on other
    // steps (e.g. a Run step, as we will see in a moment).
    const run_step = b.step("run", "Run the app");

    // This creates a RunArtifact step in the build graph. A RunArtifact step
    // invokes an executable compiled by Zig. Steps will only be executed by the
    // runner if invoked directly by the user (in the case of top level steps)
    // or if another step depends on it, so it's up to you to define when and
    // how this Run step will be executed. In our case we want to run it when
    // the user runs `zig build run`, so we create a dependency link.
    const run_cmd = b.addRunArtifact(exe);
    run_step.dependOn(&run_cmd.step);

    // By making the run step depend on the default step, it will be run from the
    // installation directory rather than directly from within the cache directory.
    run_cmd.step.dependOn(b.getInstallStep());

    // This allows the user to pass arguments to the application in the build
    // command itself, like this: `zig build run -- arg1 arg2 etc`
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    // Creates an executable that will run `test` blocks from the provided module.
    // Here `mod` needs to define a target, which is why earlier we made sure to
    // set the releative field.
    const mod_tests = b.addTest(.{
        .root_module = mod,
    });

    // A run step that will run the test executable.
    const run_mod_tests = b.addRunArtifact(mod_tests);

    // Creates an executable that will run `test` blocks from the executable's
    // root module. Note that test executables only test one module at a time,
    // hence why we have to create two separate ones.
    const exe_tests = b.addTest(.{
        .root_module = exe.root_module,
    });

    // A run step that will run the second test executable.
    const run_exe_tests = b.addRunArtifact(exe_tests);

    // A top level step for running all tests. dependOn can be called multiple
    // times and since the two run steps do not depend on one another, this will
    // make the two of them run in parallel.
    const test_step = b.step("test", "Run tests");
    test_step.dependOn(&run_mod_tests.step);
    test_step.dependOn(&run_exe_tests.step);

    // Just like flags, top level steps are also listed in the `--help` menu.
    //
    // The Zig build system is entirely implemented in userland, which means
    // that it cannot hook into private compiler APIs. All compilation work
    // orchestrated by the build system will result in other Zig compiler
    // subcommands being invoked with the right flags defined. You can observe
    // these invocations when one fails (or you pass a flag to increase
    // verbosity) to validate assumptions and diagnose problems.
    //
    // Lastly, the Zig build system is relatively simple and self-contained,
    // and reading its source code will allow you to master it.
}
