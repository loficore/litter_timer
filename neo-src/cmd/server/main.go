// Package main — entry point for the little-timer HTTP server.
//
// Replaces the W1 stub.  Wiring:
//
//	┌─────────┐  os.Args   ┌──────────┐  ServeOptions   ┌────────────┐
//	│  main() │ ─────────► │ cli.Cobra│ ──────────────► │ runServer  │
//	└─────────┘            └──────────┘                 └────────────┘
//	                                                           │
//	                                                           ▼
//	                                  SQLite → Settings → Clock → Backup → App
//	                                                           │
//	                                                           ▼
//	                                              Gin router + http.Server
//	                                              (background goroutine)
//
// Platform behaviour:
//
//   - Linux/macOS default: HTTP-only.  The webview window is opt-in.
//   - Windows default:     Webview.  Pass --http-only to skip the window.
//   - The webview package is gated behind `-tags webview`; without that
//     tag, the Run() call returns an error and the HTTP server keeps
//     serving on its own.  This keeps the binary buildable on machines
//     without GTK/webkit dev packages (CI, minimal containers).
//
// Signals:
//
//   - SIGINT / SIGTERM: graceful shutdown (5s deadline for HTTP server)
//   - webview window close: treated like a shutdown signal in webview mode
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"little-timer/internal/cli"
	"little-timer/internal/domain"
	httpx "little-timer/internal/http"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/webview"
)

// shutdownTimeout is the grace period for the HTTP server to drain
// in-flight requests before being forced shut.  Mirrors the Zig
// source's 5-second stop window.
const shutdownTimeout = 5 * time.Second

func main() {
	root := cli.NewRootCmd(runServer)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runServer is the serve callback handed to the CLI.  It wires up the
// App bundle, starts the HTTP server, optionally blocks in the
// webview window, then performs a graceful shutdown.
//
// Returns an error instead of os.Exit so Cobra's Execute() can format
// and report it (matches the layered-error convention used elsewhere
// in the CLI / settings packages).
func runServer(opts *cli.ServeOptions) error {
	// 1. Bootstrap the App: SQLite → Settings → Clock → Backup → App.
	app, cleanup, err := bootstrapApp(opts.DBPath)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer cleanup()

	// 2. Start the HTTP server in a background goroutine.  ListenAndServe
	//    blocks, so we run it concurrently and surface fatal errors via
	//    the serverErr channel.
	router := httpx.NewRouter(app, opts.CORSOrigin)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", opts.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stdout, "HTTP server listening on :%d (db=%s, cors-origin=%s)\n", opts.Port, opts.DBPath, opts.CORSOrigin)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// 3. Wait for shutdown.  In http-only mode we block on signals.  In
	//    webview mode we block on signals OR window-close.  The webview
	//    call runs in its own goroutine because Run() doesn't take a
	//    "block but return on signal" parameter — we want to react to
	//    SIGTERM even when the user hasn't closed the window.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var webviewErrCh chan error
	if !opts.HTTPOnly {
		webviewErrCh = make(chan error, 1)
		go func() {
			webviewErrCh <- webview.Run()
		}()
		fmt.Fprintln(os.Stdout, "webview mode: window opened (close it or send SIGTERM to quit)")
	} else {
		fmt.Fprintln(os.Stdout, "http-only mode: send SIGTERM (Ctrl+C) to quit")
	}

	// 4. Block until one of: signal, server error, webview exit.
	var (
		sig           os.Signal
		srvErr        error
		winErr        error
		webviewClosed bool
	)
	select {
	case sig = <-sigCh:
		fmt.Fprintf(os.Stdout, "\nreceived %s, shutting down...\n", sig)
	case srvErr = <-serverErr:
		fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", srvErr)
	case winErr = <-webviewErrCh:
		webviewClosed = true
		fmt.Fprintf(os.Stdout, "webview window closed, shutting down...\n")
		if winErr != nil {
			fmt.Fprintf(os.Stderr, "webview error: %v\n", winErr)
		}
	}

	// 5. If we exited via signal in webview mode, the webview goroutine
	//    is still running.  Give it a moment to notice the server is
	//    gone, then close the window from the main goroutine.  webview_go
	//    shuts down cleanly when the parent process exits anyway.
	_ = webviewClosed // silence unused warning

	// 6. Graceful HTTP shutdown.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP shutdown error: %v\n", err)
	}

	// If we got a server error during shutdown, prefer that as the
	// return value so the caller (Cobra) prints the right message.
	if srvErr != nil {
		return srvErr
	}
	return winErr
}

// bootstrapApp wires up the SQLite → Settings → Clock → Backup → App
// chain.  Returns the App plus a cleanup func that closes the DB and
// deinitialises the clock manager.  Mirrors the Zig
// `MainApplication.init(allocator)` flow but doesn't allocate a
// general-purpose allocator (Go manages memory for us).
func bootstrapApp(dbPath string) (*httpapp.App, func(), error) {
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlite.Migrate(); err != nil {
		_ = sqlite.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		_ = sqlite.Close()
		return nil, nil, fmt.Errorf("settings: %w", err)
	}

	clk := domain.NewClockManager(sm.BuildClockConfig())

	// Backup is optional — rebuild from the persisted BackupConfig;
	// RebuildBackup falls back to a local adapter, and only returns an
	// error when that also fails (backup disabled for this process).
	a := httpapp.NewApp(clk, sm, sqlite, nil, dbPath)
	if err := a.RebuildBackup(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: backup disabled (%v)\n", err)
	}

	cleanup := func() {
		clk.Deinit()
		if err := sqlite.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "sqlite close: %v\n", err)
		}
	}
	return a, cleanup, nil
}
