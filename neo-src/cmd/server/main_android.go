//go:build android
// +build android

// Android entrypoint for little-timer.
//
// Replaces the desktop main.go on Android builds (e.g.
// `GOOS=android GOARCH=arm64 go build -tags android`).  We do not
// own the Go runloop on Android — main.go's `func main()` still
// links in (so the package compiles for Android), but the Wails
// Android host drives the lifecycle:
//
//   - The Kotlin/Java Activity creates a `WailsBridge` instance.
//   - The bridge's `nativeInit` JNI entrypoint stores the JavaVM +
//     bridge global reference, then invokes the function we register
//     here via `application.RegisterAndroidMain` in a goroutine
//     (`pkg/application/application_android.go`).
//   - Subsequent bridge callbacks (`nativeOnStart`, `nativeOnResume`,
//     `nativeOnPause`, `nativeHandleRuntimeCall`, ...) dispatch to
//     the Wails message processor and into our services.
//
// Because the host owns the runloop, we deliberately do NOT call
// `wailsApp.Run()` — it would block here forever and never return.
//
// Why no `func main()`?  The package already has `main()` in
// main.go; adding a second one for Android would conflict.  Instead
// we install the Wails setup as an `init()`-registered callback —
// the same trick gomobile-based libraries use to defer startup to
// the host.
package main

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"little-timer/internal/domain"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/log"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

//go:embed all:assets
var assets embed.FS

// wailsApp is the *application.App the Android JNI bridge hands
// incoming runtime calls to.  Built once by bootWails; read by the
// Wails message processor when the WebView posts a runtime call
// (see `handleRuntimeCallForAndroid` in `application_android.go`).
var wailsApp *application.App

// bootWails builds the Wails App + service wrappers.  Invoked by
// the Android JNI bridge after `nativeInit` stores the bridge
// global ref — see `Java_com_wails_app_WailsBridge_nativeInit`.
//
// ponytail: the service registration list mirrors `bindings/.../
// wailsbindings.ts` exactly — every method the Wails client calls
// must have a corresponding exported method on one of these types.
// Add new methods to `wails_services.go`, not here.
func bootWails() {
	storagePath := application.Android.StoragePath()

	if err := log.Init(filepath.Join(storagePath, "logs")); err != nil {
		log.Error("log.Init failed", "error", err.Error())
	}

	log.Info(fmt.Sprintf("[bootWails] StoragePath=%q", storagePath))
	log.Info("[bootWails] starting")

	dbPath := filepath.Join(storagePath, "little_timer.db")
	log.Info(fmt.Sprintf("[bootWails] dbPath=%q backupDir=%q", dbPath, filepath.Join(storagePath, "backups")))

	sqlite := storage.NewSqliteManager().Init(dbPath)
	log.Debug("[bootWails] sqlite manager created")
	if err := sqlite.Open(); err != nil {
		log.Error(fmt.Sprintf("[bootWails] sqlite.Open FAILED: %v", err))
		panic(fmt.Sprintf("open sqlite: %v", err))
	}
	log.Debug("[bootWails] sqlite opened")
	if err := sqlite.Migrate(); err != nil {
		log.Error(fmt.Sprintf("[bootWails] migrate FAILED: %v", err))
		panic(fmt.Sprintf("migrate: %v", err))
	}
	log.Debug("[bootWails] sqlite migrated")

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		log.Error(fmt.Sprintf("[bootWails] settings FAILED: %v", err))
		panic(fmt.Sprintf("settings: %v", err))
	}
	log.Debug("[bootWails] settings created")

	clk := domain.NewClockManager(sm.BuildClockConfig())
	log.Debug("[bootWails] clock created")

	// RebuildBackup derives the backup dir from dbPath (`<storage>/backups`)
	// and honours the persisted BackupConfig, falling back to local on failure.
	a := httpapp.NewApp(clk, sm, sqlite, nil, dbPath)
	if err := a.RebuildBackup(context.Background()); err != nil {
		log.Info(fmt.Sprintf("[bootWails] backup disabled: %v", err))
	}

	log.Debug(fmt.Sprintf("[bootWails] app created a=%p sqlite=%p sm=%p clk=%p bm=%p",
		a, sqlite, sm, clk, a.BackupManager()))

	wailsApp = application.New(application.Options{
		Services: []application.Service{
			application.NewService(httpapp.NewTimerService(a)),
			application.NewService(httpapp.NewHabitService(a)),
			application.NewService(httpapp.NewSettingsService(a)),
			application.NewService(httpapp.NewBackupService(a)),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
	})
	log.Debug("[bootWails] wailsApp created")

	go func() {
		log.Info("[bootWails] wailsApp.Run starting")
		if err := wailsApp.Run(); err != nil {
			log.Error("wails runtime error", "error", err.Error())
		}
		log.Info("[bootWails] wailsApp.Run exited")
	}()
	log.Info("[bootWails] done, goroutine started")
}

// init wires

// init wires `bootWails` into the Wails Android lifecycle before
// any other code runs.  The host calls our registered func in a
// goroutine after `nativeInit`; we don't need to do anything else
// from Go's perspective.
func init() {
	application.RegisterAndroidMain(bootWails)
}
