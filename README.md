# Little Timer

Little Timer 是一个基于 Go、Gin、SQLite 和 WebView 开发的跨平台定时器应用，支持倒计时、正计时和世界时钟功能。前端使用 Preact、TypeScript、Vite 和 Tailwind CSS。

## 项目特点

- 🎯 **跨平台**：支持 Linux 和 Windows，Android 提供实验性的 Wails 构建流程
- ⚡ **可靠后端**：使用 Go、Gin 和 SQLite
- 🖥️ **桌面运行时**：可选 WebView 窗口，也可使用 HTTP-only 模式
- 🎨 **现代 UI**：基于 Preact + Tailwind CSS 的响应式界面
- 🔄 **模块化架构**：清晰的前后端分离设计
- 📱 **移动友好**：支持触摸操作和移动端适配

## 开源协议

本项目采用 [Apache License 2.0](./LICENSE) 协议，请遵照协议使用。

## 快速开始

### 桌面端（Linux / WSL / Windows）

默认开发流程会同时启动前端开发服务器和 Go HTTP 后端：

```bash
just go-dev
```

Linux 上也可以只运行 HTTP-only 后端：

```bash
cd neo-src
go run ./cmd/server serve --http-only
```

需要桌面 WebView 窗口时，使用 `just go-dev-webview`。Linux WebView 运行需要 `webkit2gtk-4.1` 或 `webkitgtk-6.0` 系统库。

### Android

⚠️ **当前状态**：Android 有基于 Wails 的实验性构建脚本，但仍需要本地 Android SDK、NDK 和 Gradle 环境，不代表 Android 发布版支持已经完成。可使用 `just apk` 构建调试 APK，或使用 `just apk-package` 仅执行 Gradle 打包。

## 依赖与环境要求

- **Go**：使用 `neo-src/go.mod` 声明的 Go 1.25.0
- **Node.js + pnpm**：用于前端开发与构建（前端代码位于 assets/）
- **系统库**：桌面 WebView 模式在 Linux 上需要 `webkit2gtk-4.1` 或 `webkitgtk-6.0`

> 若你只运行后端，HTTP-only 模式默认使用已存在的前端产物；需要修改 UI 时请看下方“前端开发与构建流程”。

## 前端开发与构建流程

进入前端目录并安装依赖：

```bash
cd assets
pnpm install
```

本地开发（HMR）：

```bash
pnpm run dev
```

生产构建（输出到 assets/dist）：

```bash
pnpm run build
```

代码检查：

```bash
pnpm run lint
```

## 脚本构建与打包

Linux / macOS Go 构建：

```bash
./scripts/build.sh --go --release
./scripts/build.sh --go --debug
```

Windows 构建脚本仍可用，但当前 PowerShell 脚本仍面向旧的桌面构建流程；Go 后端可直接在 `neo-src` 中构建：

```powershell
cd neo-src
go build -o bin/server ./cmd/server/
```

打包脚本：

```bash
./scripts/package_go.sh --version 1.0.0
```

## 配置说明

运行时配置已改为 SQLite 持久化，不再读取 `settings.toml`。

- 主数据库：`little_timer.db`（默认在程序工作目录）
- 设置项：存储在 SQLite 的 settings 相关表中
- 预设与习惯：统一存储在 SQLite 中

可通过接口查看/更新设置：

- `GET /api/settings`
- `POST /api/settings`

## 关于工具使用

```bash
# 构建 Go 后端并运行静态检查
just go-build
just go-vet

# 查看可用任务
just --list
```

## 常见问题

**Q：为什么编译失败？**
A：确认 Go 1.25.0、Node.js 和 pnpm 已安装。嵌入前端时还要先在 `assets/` 执行 `pnpm run build`。

**Q：编译很慢？**
A：首次构建会下载 Go 和前端依赖，后续构建会使用本地缓存。

**Q：我想了解更多技术细节？**
A：参考 [neo-src/](./neo-src/) 和 [android/](./android/) 目录。
