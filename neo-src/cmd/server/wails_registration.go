//go:build bindings
// +build bindings

// Package main — Wails v3 service bindings registration for bindings codegen.
//
// This file exists ONLY so wails3's static analysis (the `just bindings`
// step) can find the application.NewService() calls when generating TS
// bindings WITHOUT the android build tag.  It is gated behind the `bindings`
// build tag so the normal desktop binary — which never uses Wails at runtime
// (it serves HTTP + webview_go) — does NOT compile the wails v3 application
// package, which drags in the gtk4 + webkitgtk-6.0 cgo dependency that is
// unavailable on EL9 (AlmaLinux 9) and minimal/container builds.
//
// The android-specific registration lives in main_android.go (`//go:build
// android`); android builds register the real services in bootWails() and do
// not need this file.  `scripts/generate-bindings.sh` passes
// `-tags=bindings` in desktop mode so the codegen still sees these calls.

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	httpapp "little-timer/internal/http/app"
)

var _ = application.NewService(httpapp.NewTimerService(nil))
var _ = application.NewService(httpapp.NewHabitService(nil))
var _ = application.NewService(httpapp.NewSettingsService(nil))
var _ = application.NewService(httpapp.NewBackupService(nil))
