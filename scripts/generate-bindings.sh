#!/bin/bash
# Generate and fix Wails bindings for Android compatibility
# ponytail: run this after updating wails_bindings.go to regenerate bindings

set -e

BINDINGS_VERSION="v3.0.0-alpha2.114"
WORKDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$WORKDIR")"
ANDROID_MODE=false
for arg in "$@"; do
    case $arg in
        --android) ANDROID_MODE=true ;;
    esac
done

# Install Go if not present (needed for docker exec containers)
if ! hash go 2>/dev/null; then
    echo "=== Go not found, installing Go 1.23.0 ==="
    GO_INSTALL_DIR="/tmp/go-install"
    GO_TARBALL="https://go.dev/dl/go1.23.0.linux-amd64.tar.gz"

    mkdir -p "$GO_INSTALL_DIR"

    if [[ ! -f "${GO_INSTALL_DIR}/go/bin/go" ]]; then
        echo "Downloading Go from ${GO_TARBALL}..."
        wget -q -O /tmp/go.tar.gz "${GO_TARBALL}" || curl -sSL -o /tmp/go.tar.gz "${GO_TARBALL}"

        echo "Extracting Go to ${GO_INSTALL_DIR}..."
        tar -C "$GO_INSTALL_DIR" -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz

        echo "Go installed successfully"
    fi

    export GOROOT="${GO_INSTALL_DIR}/go"
    export PATH="${GOROOT}/bin:${PATH}"
    echo "Using Go from ${GOROOT}"
fi

# Generate wails3 bindings
echo "=== Generating Wails bindings ==="
cd "$PROJECT_ROOT/neo-src"

if [ "$ANDROID_MODE" = "true" ]; then
    # Android mode: generate bindings with -tags=android
    go run github.com/wailsapp/wails/v3/cmd/wails3@${BINDINGS_VERSION} generate bindings \
        -ts -clean \
        -f '-tags=android' \
        -d /tmp/wails-bindings \
        ./cmd/server
    BINDINGS_DIR="/tmp/wails-bindings"
else
    # Desktop mode: standard bindings.  Pass -tags=bindings so the codegen
    # sees wails_registration.go (gated behind //go:build bindings), without
    # forcing the wails gtk4/webkitgtk-6.0 cgo dep into the normal build.
    go run github.com/wailsapp/wails/v3/cmd/wails3@${BINDINGS_VERSION} generate bindings \
        -ts -clean \
        -f '-tags=bindings' \
        -d /tmp/wails-bindings \
        ./cmd/server
    BINDINGS_DIR="/tmp/wails-bindings"
fi

# Store Go environment for docker exec containers
export GODEBUG=netdns=go

echo "=== Copying and fixing bindings ==="
BINDINGS_SRC="/tmp/wails-bindings/little-timer/internal/http/app"
mkdir -p "$PROJECT_ROOT/assets/src/bindings/little-timer/internal/app"
mkdir -p "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings/little-timer/internal/app"

# Copy fresh bindings
if [ "$ANDROID_MODE" = "true" ]; then
    cp "$BINDINGS_DIR/little-timer/internal/http/app"/*.ts "$PROJECT_ROOT/assets/src/bindings/little-timer/internal/app/" 2>/dev/null || true
    cp "$BINDINGS_DIR/little-timer/internal/http/app"/*.ts "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings/little-timer/internal/app/" 2>/dev/null || true
else
    cp "$BINDINGS_DIR/little-timer/internal/http/app"/*.ts "$PROJECT_ROOT/assets/src/bindings/little-timer/internal/app/" 2>/dev/null || true
    cp "$BINDINGS_DIR/little-timer/internal/http/app"/*.ts "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings/little-timer/internal/app/" 2>/dev/null || true
fi

# Validate bindings were generated
BINDINGS_SRC="$BINDINGS_DIR/little-timer/internal/http/app"
if [ ! -d "$BINDINGS_SRC" ] || [ "$(ls -1 "$BINDINGS_SRC"/*.ts 2>/dev/null | wc -l)" -lt 5 ]; then
    echo "ERROR: No bindings generated (found $(ls -1 "$BINDINGS_SRC"/*.ts 2>/dev/null | wc -l) .ts files, expected ≥5)"
    echo "wails3 output:"
    ls -la "$BINDINGS_SRC" 2>/dev/null || echo "Directory does not exist"
    exit 1
fi
echo "Bindings validation passed: $(ls -1 "$BINDINGS_SRC"/*.ts 2>/dev/null | wc -l) files found"

# Fix @wailsio/runtime import paths
find "$PROJECT_ROOT/assets/src/bindings" -name "*.ts" -exec sed -i 's|@wailsio/runtime|/wails/runtime.js|g' {} \;
find "$PROJECT_ROOT/neo-src/cmd/server/assets/bindings" -name "*.ts" -exec sed -i 's|@wailsio/runtime|/wails/runtime.js|g' {} \;

echo "=== Done ==="
echo "Bindings generated and fixed at:"
echo "  - assets/src/bindings/little-timer/internal/app/"
echo "  - neo-src/cmd/server/assets/bindings/little-timer/internal/app/"
echo "Mode: ${ANDROID_MODE:+Android (with -tags=android)}${ANDROID_MODE:+, }desktop (without -tags=android)"
