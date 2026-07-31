#!/bin/bash
# 构建 Android APK
# 用法: ./scripts/build-android.sh [--package-only]
#
#   --package-only   仅运行 Gradle 打包（假设 .so 已编译好）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

PACKAGE_ONLY=false
for arg in "$@"; do
    case $arg in
        --package-only) PACKAGE_ONLY=true ;;
    esac
done

echo "=== Building Android APK ==="

# Step 1: Generate Wails bindings
if [ "$PACKAGE_ONLY" = "false" ]; then
    echo "--- Generate bindings ---"
    bash "$SCRIPT_DIR/generate-bindings.sh" --android

    # Step 2: Frontend build (Vite will inline bindings into dist/index.html)
    echo "--- Frontend ---"
    cd assets
    pnpm install
    pnpm run build
    cd ..

    # Step 3: 同步到 Go embed 路径
    cp assets/dist/index.html neo-src/cmd/server/assets/index.html

    # 确保 runtime.js 注入存在 (幂等: 只加一次)
    if ! grep -q 'src="/wails/runtime.js"' neo-src/cmd/server/assets/index.html; then
        sed -i 's|</head>|<script type="module" src="/wails/runtime.js"></script></head>|' \
            neo-src/cmd/server/assets/index.html
    fi
fi

# Step 2: Copy runtime.js to Android assets (Wails runtime needed)
mkdir -p android/app/src/main/assets/wails
WAILS_MOD_DIR="$(cd "$PROJECT_ROOT/neo-src" && go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v3)"
if [ -z "$WAILS_MOD_DIR" ] || [ ! -f "$WAILS_MOD_DIR/internal/assetserver/bundledassets/runtime.js" ]; then
    echo "Error: could not resolve Wails v3 runtime.js ('go list -m' failed or file missing, module dir: ${WAILS_MOD_DIR:-<empty>})" >&2
    exit 1
fi
cp "$WAILS_MOD_DIR/internal/assetserver/bundledassets/runtime.js" android/app/src/main/assets/wails/runtime.js

# Step 3: Go → Android .so
NDK_ROOT="${ANDROID_NDK_HOME:-$ANDROID_HOME/ndk/26.3.11579264}"
if [ ! -d "$NDK_ROOT" ]; then
    echo "Error: Android NDK not found at $NDK_ROOT"
    echo "Set ANDROID_NDK_HOME or ANDROID_HOME/ndk/26.3.11579264"
    exit 1
fi

HOST_TAG="$(uname -s | grep -q Darwin && echo darwin-x86_64 || echo linux-x86_64)"
TOOLCHAIN="$NDK_ROOT/toolchains/llvm/prebuilt/$HOST_TAG"

mkdir -p android/app/src/main/jniLibs/arm64-v8a
mkdir -p android/app/src/main/jniLibs/x86_64

echo "--- Go shared lib: arm64-v8a ---"
(cd neo-src && \
CC="$TOOLCHAIN/bin/aarch64-linux-android21-clang" \
CXX="$TOOLCHAIN/bin/aarch64-linux-android21-clang++" \
CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
    go build -buildmode=c-shared -tags android,debug -buildvcs=false \
    -o ../android/app/src/main/jniLibs/arm64-v8a/libwails.so ./cmd/server)

echo "--- Go shared lib: x86_64 ---"
(cd neo-src && \
CC="$TOOLCHAIN/bin/x86_64-linux-android21-clang" \
CXX="$TOOLCHAIN/bin/x86_64-linux-android21-clang++" \
CGO_ENABLED=1 GOOS=android GOARCH=amd64 \
    go build -buildmode=c-shared -tags android,debug -buildvcs=false \
    -o ../android/app/src/main/jniLibs/x86_64/libwails.so ./cmd/server)

# Step 5: Gradle assemble
echo "--- Gradle assembleDebug ---"
cd android
./gradlew assembleDebug
cd ..
mkdir -p bin
cp android/app/build/outputs/apk/debug/app-debug.apk bin/LittleTimer.apk

echo "=== APK: bin/LittleTimer.apk ==="
