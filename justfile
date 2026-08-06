set shell := ["bash", "-eu", "-c"]

# ── 1. 动态加载本地私有配置 ──────────────────────────────────────────
import? "justfile.local"

SSH_TARGET := env_var_or_default("MY_ACT_SSH_TARGET", "")
REMOTE_PATH := env_var_or_default("MY_ACT_REMOTE_PATH", "")

# ── 2. 核心公共命令 ──────────────────────────────────────────────────
ci-run: go-build-check build-check

act:
        @if [ -n "{{SSH_TARGET}}" ]; then \
            echo "=== [远程调试] 通过 SSH 在 {{SSH_TARGET}} 运行 act ==="; \
            just r-act-start; \
        else \
            echo "=== [本地调试] 在当前机器运行 act ==="; \
            act --secret-file .act.env -P ubuntu-latest=catthehacker/ubuntu:act-latest; \
        fi

# ── 3. 原有公共命令（保持不变） ────────────────────────────────────────

frontend-dev:
        @./scripts/dev.sh

dev-webview:
        @./scripts/dev.sh --webview

build-check:
        @zig build test && cd assets && pnpm run lint && pnpm run build:checkc

frontend-build:
        @cd assets && pnpm install && pnpm run build

backend-dev:
        @zig build -Dembed_ui=false -Doptimize=Debug run -- --webview

# ── Go backend (neo-src) ──────────────────────────────────────────────

go_src := justfile_directory() / "neo-src"

go-build:
        @cd {{go_src}} && go build -o bin/server ./cmd/server

go-test:
        @cd {{go_src}} && go test ./...

go-test-race:
        @cd {{go_src}} && go test -race ./...

go-vet:
        @cd {{go_src}} && go vet ./...

go-tidy:
        @cd {{go_src}} && go mod tidy

go-lint: go-vet

go-build-check: go-tidy go-vet go-test

go-dev:
        #!/usr/bin/env bash
        set -e
        trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

        find_available_port() {
            local port=$1
            while ss -tlnp | grep -q ":${port} "; do
                port=$((port + 1))
            done
            echo "$port"
        }

        GO_PORT=$(find_available_port 8013)
        export BACKEND_PORT=$GO_PORT

        echo "=== 启动前端 Dev Server ==="
        cd assets && pnpm run dev &
        VITE_PID=$!
        cd ..
        echo "等待前端服务启动..."
        sleep 3

        echo "=== 启动 Go 后端 ==="
        cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --http-only --port $GO_PORT 2>&1 &
        GO_PID=$!

        echo ""
        echo "=== 服务已启动 ==="
        echo "前端: http://localhost:5173"
        echo "Go API: http://localhost:$GO_PORT"
        echo ""
        echo "按 Ctrl+C 停止所有服务"
        wait

go-dev-webview:
        #!/usr/bin/env bash
        set -e
        trap 'kill $VITE_PID $GO_PID 2>/dev/null; exit 0' INT TERM

        find_available_port() {
            local port=$1
            while ss -tlnp | grep -q ":${port} "; do
                port=$((port + 1))
            done
            echo "$port"
        }

        GO_PORT=$(find_available_port 8013)
        export BACKEND_PORT=$GO_PORT

        echo "=== 启动前端 Dev Server ==="
        cd assets && pnpm run dev &
        VITE_PID=$!
        cd ..
        echo "等待前端服务启动..."
        sleep 3

        echo "=== 启动 Go 后端 (webview) ==="
        cd {{go_src}} && go build -o bin/server ./cmd/server && bin/server serve --webview --port $GO_PORT &
        GO_PID=$!

        echo ""
        echo "=== 服务已启动 ==="
        echo "前端: http://localhost:5173"
        echo "Go API + WebView: http://localhost:$GO_PORT"
        echo ""
        echo "按 Ctrl+C 停止所有服务"
        wait

go-run: go-build
        @{{go_src}}/bin/server serve

go-clean:
        @rm -rf {{go_src}}/bin

go-build-embed:
        @cd {{go_src}} && go build -tags embed_ui -o bin/server ./cmd/server

# ── Android ────────────────────────────────────────────────────────────

apk:
        @./scripts/build-android.sh

apk-package:
        @./scripts/build-android.sh --package-only

bindings:
        @./scripts/generate-bindings.sh

default: go-dev