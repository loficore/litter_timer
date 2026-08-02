# Little Timer 安卓调试手册

这份手册面向在安卓设备上调试 Little Timer 的开发者，主要排查"读不出"、"写不进去"、"重启后数据丢了"这类问题。所有命令、日志路径、tag 名都来自当前代码库的实际实现，不是凭空设定。

## 1. 前置条件

开始排查之前，先把环境备齐。下面任何一条不满足都会让你在错误的地方浪费时间。

### 1.1 调试 APK

调试 APK 由 `just apk` 构建：

```bash
just apk
```

如果构建中途卡在 Go 模块下载（`proxy.golang.org` 在某些网络环境下会超时），加 `GOPROXY=direct`：

```bash
GOPROXY=direct just apk
```

这是一个 debug APK，`BuildConfig.DEBUG = true`，意味着：
- `MainActivity.java:117-119` 启用了 `WebView.setWebContentsDebuggingEnabled(true)`，可以用 `chrome://inspect` 远程调试前端
- 应用是 `debuggable`，可以用 `adb run-as` 进它的私有目录读日志文件

### 1.2 adb 与设备

```bash
# adb 在 NDK/SDK 路径下
ADB="$ANDROID_HOME/platform-tools/adb"

# 设备或模拟器至少要列出一行
$ADB devices
# List of devices attached
# emulator-5554   device
```

如果 `adb devices` 一行设备都没有，先解决模拟器或 USB 调试，别往下走。

### 1.3 NDK 版本

需要的 NDK 版本是 `26.3.11579264`。版本不匹配会在跨编译阶段报错。

### 1.4 包名

包名是 `com.littletimer`，来自 `android/app/build.gradle`。后面所有 `adb shell am`、`run-as` 命令都依赖这个包名。

### 1.5 跨编译头文件问题

如果 `go build` 跨编译到 `GOOS=android GOARCH=arm64` 时报：

```
android/log.h: 没有那个文件或目录
```

是因为 cgo 找不到 NDK sysroot 里的头文件。设两个环境变量即可：

```bash
NDK="$ANDROID_HOME/ndk/26.3.11579264"
export CGO_CFLAGS="-isystem $NDK/sysroot/usr/include -isystem $NDK/sysroot/usr/include/aarch64-linux-android"
```

`-isystem` 两个路径分别覆盖通用头文件和 arm64 架构专用头文件。

## 2. 日志来源表

安卓端的日志分散在好几个来源里，单个 tag 抓不全。先看总览：

| 来源 | 怎么找 | 对应代码 |
|---|---|---|
| Go 应用（logcat） | `adb logcat -s LittleTimer:V` | `neo-src/internal/log/sink_android.go`（cgo `__android_log_print`，tag 写死 `LittleTimer`） |
| Go 应用（文件） | `adb exec-out run-as com.littletimer cat files/logs/<时间戳>.log` | `neo-src/cmd/server/main_android.go:65` 的 `log.Init` + `internal/log/logger.go:42` 的 `openLogDir` |
| Wails 框架 | `adb logcat -s Wails:V` | Wails v3 模块（只读参考，不改） |
| Java 桥接器/Activity | `adb logcat -s WailsBridge:V MainActivity:V` | `android/app/src/main/java/com/littletimer/` |
| 前端 JS 桥接 | `adb logcat -s WailsJSBridge/JS:V` | `WailsJSBridge.java:75-94`（注意 tag 是 `TAG + "/JS"`，不是 `WailsJSBridge`） |
| 前端 console | `chrome://inspect` 远程调试，或 `adb logcat -s chromium:V` | `MainActivity.java:117-119` 启用了 `setWebContentsDebuggingEnabled(true)` |

### 2.1 合并过滤命令

要一次性覆盖所有来源，用 logcat 的 `-s` 加 `*:S`（其余全静默）：

```bash
adb logcat -s LittleTimer:V Wails:V WailsBridge:V MainActivity:V WailsJSBridge/JS:V chromium:V *:S
```

这行命令是排查 90% 问题的起点，建议存成 shell alias。

### 2.2 ⚠️ `WailsJSBridge/JS` 的 `/JS` 后缀是必须的

这是最容易踩的坑。前端通过 `window.wails.log(level, message)` 调到 `WailsJSBridge.java` 的 `log` 方法（第 75-94 行），这个方法把 tag 拼成：

```java
Log.d(TAG + "/JS", message);   // TAG = "WailsJSBridge"，所以实际 tag 是 "WailsJSBridge/JS"
```

所以：
- `adb logcat -s WailsJSBridge:V` **抓不到** 前端日志
- `adb logcat -s WailsJSBridge/JS:V` **才能抓到** 前端日志

如果你只看到 `WailsJSBridge` tag 下的 `Invoke called: ...` / `InvokeAsync called: ...`（那是 `invoke` / `invokeAsync` 方法打的，第 40、54 行），但看不到前端主动 `console.log` 的内容，原因就是这个 tag 拼错了。

## 3. 日志文件路径

**这一段是排查"读不出/写不进"问题的核心。** Go 后端的日志不仅写到 logcat，还写到文件里。文件比 logcat 稳定，不会被系统缓冲冲掉，也不会因为 logcat ring buffer 满了被覆盖。

### 3.1 路径是怎么来的

在 `neo-src/cmd/server/main_android.go:63-65`：

```go
storagePath := application.Android.StoragePath()
if err := log.Init(filepath.Join(storagePath, "logs")); err != nil {
    log.Error("log.Init failed", "error", err.Error())
}
```

`application.Android.StoragePath()` 在 Wails v3 里最终调到 Android `Activity.getFilesDir()`。所以日志目录是：

```
/data/data/com.littletimer/files/logs/
```

**完整路径**：`/data/data/com.littletimer/files/logs/<时间戳>.log`

其中 `<时间戳>` 形如 `2026-08-03_15-04-05`，来自 `internal/log/logger.go:81` 的 `time.Now().Format("2006-01-02_15-04-05")`。

### 3.2 文件轮转规则

`openLogDir`（`logger.go:42-88`）的逻辑：

1. 列出 `logs/` 目录下所有文件，按文件大小**升序**排序（最小的在最后）
2. 取最小的那个文件打开，准备 append
3. 如果这个文件已经超过 10MB（`logger.go:74` 的 `info.Size() > 10*1024*1024`），关掉它，新建一个带当前时间戳的新文件
4. 如果目录是空的（首次启动），直接新建带时间戳的文件

所以一个日志文件最多长到约 10MB，然后下次启动时会换新文件。**排查时优先看最新的文件。**

### 3.3 拉取命令

```bash
# 列出所有日志文件（看哪个最新、最大）
adb exec-out run-as com.littletimer ls -la files/logs/

# 实时跟踪最新文件（注意把 <时间戳> 换成上一步看到的文件名）
adb exec-out run-as com.littletimer tail -F files/logs/2026-08-03_15-04-05.log

# 一次性拉到本地
adb exec-out run-as com.littletimer cat files/logs/2026-08-03_15-04-05.log > go.log
```

**`run-as` 的前提**：应用必须是 `debuggable`。debug APK 默认满足。**release 包上 `run-as` 会失败**，报 `run-as: package not debuggable`。如果必须在 release 包上排查，只能靠 logcat（`adb logcat -s LittleTimer:V`），拿不到文件日志。

### 3.4 文件格式

每行格式（`logger.go:33-35` 的 `formatLine`）：

```
[<ISO 时间戳>] [<LEVEL>]  <message> key1=value1 key2=value2
```

**注意 `LEVEL` 后面是两个空格**，因为格式串是 `fmt.Sprintf("[%s] [%s]  %s", timestamp, level, msg)`，两个 `%s` 之间有两个空格。用 `grep` 或 `awk` 切字段时要小心，别按单空格切。

`key=value` 部分是 slog 的 attrs，每行写完前先写 attrs，再写主体行（见 `logger.go:25-30` 的 `Handle` 方法）。

### 3.5 样例

下面是几行真实格式的日志（前缀对照表见第 4 节 N4）：

```
[2026-08-03T15:04:05Z] [INFO]  [bootWails] starting
[2026-08-03T15:04:05Z] [INFO]  storage.open db_path=/data/data/com.littletimer/files/little_timer.db
[2026-08-03T15:04:05Z] [INFO]  storage.migrate duration_ms=12
[2026-08-03T15:05:22Z] [INFO]  habit.create method=HabitService.CreateHabit set_id=1 name=晨跑 goal_seconds=1800 args=[]
[2026-08-03T15:05:22Z] [INFO]  habit.create ok method=HabitService.CreateHabit id=42 args=[]
```

注意最后两行的 `args=[]` 是 `log.Info(msg, args...)` 包装器（`logger.go:106-108`）自动塞进去的，不是业务字段，可以忽略。

## 4. 排查决策树（N1-N8）

针对"读不出/写不进"故障，按 N1 到 N8 顺序排查。每个节点都给出"期望看到什么"和"看不到怎么办"。

### N1 App 启动

**期望**：`adb logcat -s LittleTimer:V` 至少出现一行：

```
[2026-08-03T15:04:05Z] [INFO]  [bootWails] starting
```

这行来自 `main_android.go:70` 的 `log.Info("[bootWails] starting")`。

**看不到怎么办**：完全没有任何 `LittleTimer` tag 的行，说明 Go 进程在 `log.Init` 之前就崩了。这时候去看：

```bash
adb logcat -b crash
adb logcat -s WailsBridge:V MainActivity:V
```

`-b crash` 是 Android 的崩溃缓冲区，native 崩溃（SIGSEGV、JNI 链接失败）会在这里。`WailsBridge` 的 `nativeInit` 失败也会在它的 tag 下报错。

### N2 Wails 桥就绪

**期望**：logcat 里**不出现** `Bridge not initialized` 字样。这个字符串来自 Wails 框架本身，表示 JNI `nativeInit` 没有成功初始化 Go 侧的 bridge。不出现反过来说明桥接 OK。

**看不到怎么办**（即出现了 `Bridge not initialized`）：
- 检查 Go 库是否正确打包进 APK（`adb shell run-as com.littletimer ls lib/arm64-v8a/` 应该有 `liblittletimer.so`）
- 检查 NDK 版本对不对（见 1.3）
- 看 `WailsBridge:V` tag 下有没有 `nativeInit` 相关的 Java 异常

### N3 runtime.js 加载

**期望**：在 Chrome 远程调试里（`chrome://inspect` -> Little Timer WebView -> Console）**没有** `import("/wails/runtime.js")` 的红色错误。

**报错怎么办**：检查 `android/app/src/main/assets/wails/runtime.js` 是否存在。这个文件由构建脚本从 Wails 模块复制过去（参考 `scripts/build-android.sh`）。如果文件缺失，说明构建脚本没跑完整，或者 Wails 模块版本不对。重新 `just apk`。

### N4 JS 调用到 Go

**期望**：`adb logcat -s LittleTimer:V` 下出现形如 `<前缀>.<方法名>` 的 DEBUG 行。这一步证明前端的调用确实穿透到了 Go 服务层。

**前缀对照表**（来自 `neo-src/internal/http/app/wails_services.go` 的实际埋点）：

| Service | 前缀 | 示例 logcat 行 |
|---|---|---|
| `SettingsService` | `settings.` | `settings.get`、`settings.update`、`settings.update ok`、`settings.update failed` |
| `HabitService` | `habit.` | `habit.create`、`habit.create ok`、`habit.list`、`habit.list_sessions`、`habit.create_session` |
| `TimerService` | `timer.` | `timer.start`、`timer.start ok`、`timer.get_state`、`timer.pause`、`timer.finish` |
| `BackupService`（普通方法） | `backup.` | `backup.create`、`backup.create ok`、`backup.list`、`backup.verify`、`backup.update_config` |
| `BackupService`（凭据方法） | 4 个净化字面量 | `backup.unlock`、`backup.set_master`、`backup.lock`、`backup.master_status` |

**⚠️ BackupService 凭据方法的特殊处理**：`UnlockCredentials`、`SetMasterPassword`、`LockCredentials`、`GetMasterPasswordStatus` 这 4 个方法**不**把方法名写进 message 字段，而是用净化过的字面量。比如 `UnlockCredentials` 调用打的是：

```
[2026-08-03T15:06:11Z] [DEBUG]  backup.unlock method=BackupService.UnlockCredentials
[2026-08-03T15:06:11Z] [INFO]   backup.unlock ok method=BackupService.UnlockCredentials success=true
```

注意 message 是 `backup.unlock`（字面量），而真正的 Go 方法名 `UnlockCredentials` 在 `method` attr 里。这样设计是为了避免凭据相关字眼（`unlock`、`password` 等）被禁用词 grep 误伤到 message 字段。实际方法名仍然可追溯，只是位置在 `method=` 属性。

### N5 SQLite 打开

**期望**：`LittleTimer` tag 下出现：

```
[2026-08-03T15:04:05Z] [INFO]  storage.open db_path=/data/data/com.littletimer/files/little_timer.db
```

这行来自 `neo-src/internal/storage/sqlite.go:160` 的 `log.Info("storage.open", "db_path", m.dbPath)`。**注意 message 是 `storage.open`，没有 `ok` 后缀**。成功路径只打这一行；紧跟着会有一行 `storage.migrate duration_ms=...`（`sqlite.go:172`）。

**失败怎么办**：如果看到：

```
[2026-08-03T15:04:05Z] [ERROR]  storage.open failed db_path=... error=...
```

说明 SQLite 打不开。常见原因：
- 磁盘满（`error=no space left on device`）
- 路径权限问题（`error=permission denied`，通常是 `storagePath` 拿错了）
- 数据库文件损坏（`error=database disk image is malformed`）

看 `error=` 字段直接给原因。

### N6 读操作

**期望**：前端执行读操作后，logcat 出现对应的 `<前缀>.<方法> ok` 行。对照表：

| 前端动作 | 期望日志 |
|---|---|
| 打开设置页 | `settings.get` |
| 打开习惯列表 | `habit.list`、`habit.list_sets` |
| 查看会话记录 | `habit.list_sessions` |
| 查看计时器状态 | `timer.get_state`、`timer.get_progress` |
| 查看备份列表 | `backup.list`、`backup.master_status` |

注意：部分读方法是 `log.Debug` 级别（如 `settings.get` 在 `wails_services.go:552`），logcat 默认不显示 Debug。用 `adb logcat -s LittleTimer:V`（`V` = Verbose，包含 Debug）才能看到。

**失败**：如果是 `<前缀>.<方法> failed error=...`，看 `error=` 字段。如果 N5 通过但 N6 失败，问题在 SQL 查询层（表不存在、列不匹配、查询语法），不在连接层。

### N7 写操作

**期望**：前端执行写操作后，logcat 出现 `<前缀>.<方法> ok` 行，**带业务字段**：

| 前端动作 | 期望日志 |
|---|---|
| 创建习惯 | `habit.create ok method=HabitService.CreateHabit id=<N>` |
| 创建会话 | `habit.create_session ok method=HabitService.CreateSession id=<N>` |
| 更新设置 | `settings.update ok method=SettingsService.UpdateSettings` |
| 启动计时器 | `timer.start ok method=TimerService.StartTimer session_id=<N> habit_id=<N>` |
| 创建备份 | `backup.create ok method=BackupService.CreateBackup target_type=... backup_name=...` |

**失败**：`<前缀>.<方法> failed error=...`，**没有** `id` / `session_id` 等业务字段（因为还没创建出来）。

**关键诊断**：N5 通过但 N7 失败 -> 问题在 SQL 写入或业务校验层，不是连接问题。失败行的 `error=` 字段直接给原因，比如：
- `error=UNIQUE constraint failed: habits.name` -> 重复名
- `error=FOREIGN KEY constraint failed` -> 外键引用的对象不存在
- `error=disk I/O error` -> 磁盘问题

### N8 重启持久化

**期望**：

```bash
# 1. 写入一些数据（N7 通过）
# 2. 杀掉 App
adb shell am force-stop com.littletimer

# 3. 重新启动 App
adb shell am start -n com.littletimer/.MainActivity

# 4. 执行对应的读操作（N6），应该能读到 N7 之前写入的行
```

**失败怎么办**：N7 显示 `ok` 但 N8 重启后读不到，问题在持久化层。可能原因：
- commit 未落盘（理论上 SQLite 默认 WAL 模式不会这样，但检查 PRAGMA journal_mode）
- 数据库文件路径不对（每次启动 `storage.open` 的 `db_path` 应该一致）
- 磁盘满（写入只在 page cache，没刷盘）

**关键检查**：看 `storage.migrate duration_ms=...` 那行是否在重启后出现。如果 migrate 没跑，可能是数据库 schema 不匹配，或者 migrate 静默失败了。`duration_ms` 应该是个位数到几十毫秒，如果是 0 或者根本没这行，说明 migrate 流程有问题。

## 5. 标准复现流程

排查 bug 时，按下面 6 步走，保证每次都能拿到完整的证据：

```bash
# Step 1: 清空 logcat 缓冲
adb logcat -c

# Step 2: 启动 logcat 捕获到文件（后台运行）
adb logcat -s LittleTimer:V Wails:V WailsBridge:V MainActivity:V WailsJSBridge/JS:V chromium:V *:S > capture.txt &

# Step 3: 启动 App（如果已经在跑，先 force-stop）
adb shell am force-stop com.littletimer
adb shell am start -n com.littletimer/.MainActivity

# Step 4: 复现问题（手动操作 UI，或者在 adb shell 里触发）

# Step 5: 停止后台 logcat，拉取 Go 日志文件
kill %1
adb exec-out run-as com.littletimer ls -la files/logs/
# 找到最新的 .log 文件名
adb exec-out run-as com.littletimer cat files/logs/<最新时间戳>.log > go.log

# Step 6: 把 capture.txt 和 go.log 一起交给排查的人
```

**为什么需要两份**：logcat（`capture.txt`）覆盖所有来源（Go、Java、前端），但可能被 ring buffer 冲掉早期日志。Go 日志文件（`go.log`）只覆盖 Go，但更稳定、不会被冲掉，而且包含完整的 attrs 字段。两者交叉验证。

## 6. 证据收集建议

要"证明问题"，至少需要这 4 类证据：

1. **logcat 快照**：用第 5 节的合并过滤命令抓的 `capture.txt`，覆盖所有 tag
2. **Go 日志文件**：`adb exec-out run-as com.littletimer cat files/logs/<时间戳>.log > go.log`，包含完整的 bootWails 启动序列和 attrs
3. **复现步骤**：具体到点击了哪个按钮、输入了什么、期望什么、实际什么。不要写"改设置不生效"这种模糊描述
4. **设备信息**：`adb shell getprop ro.product.model`、`adb shell getprop ro.build.version.sdk`、`adb shell getprop ro.build.version.release`。不同 Android 版本对 WebView、文件权限的处理有差异

少任何一项，排查都会卡在"猜测"阶段。

## 7. 不要做这些事

### 7.1 不要在日志里 dump 整个配置 JSON

不要在日志里 `cat` 整个 `SettingsConfig` 或 `BackupConfig` 的 JSON。这些结构里可能包含：
- 加密密钥
- S3 凭据（access key / secret key）
- 主密码相关字段

代码库的日志埋点已经刻意避开了凭据字段（见第 4 节 N4 的 BackupService 净化字面量），但如果你手动 `log.Info(fmt.Sprintf("%+v", cfg))`，就会绕过这层保护。如果你要确认配置是否正确，只打 `target_type`、`has_cred` 这种布尔或枚举字段，不要打整个结构。

### 7.2 不要改 `go/pkg/mod` 下的 Wails 模块代码

Wails v3 是第三方依赖，只读参考模式。如果你在排查 N2（桥就绪）问题时需要看 Wails 的源码，去 `go/pkg/mod/github.com/wailsapp/wails/` 下读，但**不要改**。改了下次 `go mod tidy` 会被冲掉，而且会污染整个团队的构建。如果确认是 Wails 的 bug，去上游提 issue。

### 7.3 不要假设 `WailsJSBridge:V` 能匹配前端日志

这一点第 2.2 节已经强调过，再说一遍：前端 `window.wails.log()` 打的日志 tag 是 `WailsJSBridge/JS`，**不是** `WailsJSBridge`。用 `adb logcat -s WailsJSBridge:V` 抓不到前端日志，必须用 `adb logcat -s WailsJSBridge/JS:V`。

### 7.4 不要在 release 包上用 `run-as`

`adb run-as` 只对 `debuggable=true` 的包生效。release 包默认不可调试，`run-as` 会报：

```
run-as: package 'com.littletimer' not debuggable
```

release 包上只能靠 logcat（`adb logcat -s LittleTimer:V`），拿不到 Go 日志文件。如果必须在 release 包上排查文件日志，需要重新打 debug 包。

## 8. 排障示例

下面用一个具体例子演示决策树怎么用。

### 8.1 症状

用户报告："我改了设置，保存成功了，重启 App 后设置又变回去了。"

### 8.2 走 N1-N8

**N1 App 启动**：

```bash
adb logcat -s LittleTimer:V | grep bootWails
```

期望看到 `[bootWails] starting`。假设看到了，N1 通过。

**N2 Wails 桥就绪**：

```bash
adb logcat -s Wails:V WailsBridge:V | grep "Bridge not initialized"
```

期望没有输出。假设没有，N2 通过。

**N3 runtime.js 加载**：

`chrome://inspect` -> Little Timer WebView -> Console。期望没有红色 `import("/wails/runtime.js")` 错误。假设没有，N3 通过。

**N4 JS 调用到 Go**：

用户在 UI 上改设置并点保存。期望 logcat 出现：

```
[2026-08-03T15:10:00Z] [DEBUG]  settings.update method=SettingsService.UpdateSettings
```

假设出现了，N4 通过。

**N5 SQLite 打开**：

```bash
adb logcat -s LittleTimer:V | grep "storage.open"
```

期望：

```
[2026-08-03T15:04:05Z] [INFO]  storage.open db_path=/data/data/com.littletimer/files/little_timer.db
```

假设看到了，N5 通过。注意 `db_path` 的值，后面 N8 要对比。

**N6 读操作**：

App 启动时应该会 `settings.get`。期望：

```
[2026-08-03T15:04:06Z] [DEBUG]  settings.get method=SettingsService.GetSettings
```

假设看到了，N6 通过。

**N7 写操作**：

用户点保存后，期望：

```
[2026-08-03T15:10:00Z] [INFO]  settings.update ok method=SettingsService.UpdateSettings
```

假设看到了 `ok`，N7 通过。

**N8 重启持久化**：

```bash
adb shell am force-stop com.littletimer
adb shell am start -n com.littletimer/.MainActivity
adb logcat -s LittleTimer:V | grep "settings.get"
```

期望重启后 `settings.get` 读到的就是用户改后的值。但用户报告"变回去了"，说明 N8 失败。

**N8 失败的根因候选**：

1. **commit 没落盘**：看 `storage.open` 的 `db_path` 重启前后是否一致。如果不一致，说明 `storagePath` 不稳定，每次启动拿到不同路径，写入写到 A 文件，读取读 B 文件
2. **migrate 静默回滚了 schema**：看 `storage.migrate duration_ms=...`。如果重启后这行没出现，或者 `duration_ms=0`，可能是 migrate 流程有问题，schema 被重置了
3. **磁盘满**：`adb shell df /data`。如果 `/data` 满了，写入只在 page cache，没刷盘
4. **WAL checkpoint 没触发**：`adb exec-out run-as com.littletimer ls -la databases/`，看有没有 `-wal` 和 `-shm` 文件。WAL 模式下未 checkpoint 的写入在 `-wal` 文件里，如果 App 被 `force-stop` 杀掉，可能没来得及 checkpoint。但这种情况通常 SQLite 下次启动会自动恢复，不至于"变回去"

按这个候选列表逐个排查，基本能定位到根因。这就是 N1-N8 决策树的用法：从最外层（App 启动）一步步往里走，每一步要么通过、要么在失败处拿到 `error=` 字段直接定位问题。
