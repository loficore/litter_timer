// Package http — Gin router.
//
// File `router.go` registers every endpoint the Zig std_server.zig
// exposed, grouped under `/api/<area>`.  Routes are intentionally
// 1:1 with the Zig path table — same method, same shape, same JSON
// field names — so the existing Preact frontend works unchanged.
//
// Middleware order: CORS (outermost) → recovery (Gin default) → auth.
// The auth middleware sets the per-request "app" key so handlers can
// fetch the App bundle via `c.MustGet("app")`.
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/http/handlers"
	"little-timer/internal/http/middleware"
)

// NewRouter 创建并注册全部 HTTP 路由的 Gin 引擎。
// `corsOrigin` controls the Access-Control-Allow-Origin header; pass
// "*" for development, the concrete origin in production.
func NewRouter(a *app.App, corsOrigin string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(corsOrigin))
	r.Use(middleware.Auth(a))

	registerRoot(r)
	registerTimer(r)
	registerHabits(r)
	registerSettings(r)
	registerBackup(r)
	registerWallpapers(r)
	registerEvents(r)

	return r
}

// -----------------------------------------------------------------------------
// GET /  (SPA fallback — same shape as Zig `handleRoot`).
// -----------------------------------------------------------------------------

func registerRoot(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<html><body><h1>Little Timer</h1><p>Build the frontend (cd assets && pnpm run build) or open the dev server.</p></body></html>")
	})
}

// -----------------------------------------------------------------------------
// Timer routes.
// -----------------------------------------------------------------------------

func registerTimer(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/state", handlers.TimerState)
	g.GET("/timer/state", handlers.TimerState)
	g.GET("/timer/progress", handlers.TimerProgress)
	g.GET("/timer/config", handlers.TimerConfig)
	g.POST("/start", handlers.TimerStart)
	g.POST("/pause", handlers.TimerPause)
	g.POST("/reset", handlers.TimerReset)
	g.POST("/finish", handlers.TimerFinish)
	g.POST("/mode", handlers.TimerMode)
	g.POST("/timer/finish", handlers.TimerFinish)
	g.POST("/timer/rest", handlers.TimerStartRest)
	g.POST("/timer/config", handlers.TimerUpdateConfig)
}

// -----------------------------------------------------------------------------
// Habit routes.
// -----------------------------------------------------------------------------

func registerHabits(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/habit-sets", handlers.HabitSetList)
	g.POST("/habit-sets", handlers.HabitSetCreate)
	g.PUT("/habit-sets/:id", handlers.HabitSetUpdate)
	g.DELETE("/habit-sets/:id", handlers.HabitSetDelete)

	g.GET("/habits", handlers.HabitList)
	g.POST("/habits", handlers.HabitCreate)
	g.PUT("/habits/:id", handlers.HabitUpdate)
	g.DELETE("/habits/:id", handlers.HabitDelete)
	g.GET("/habits/:id/detail", handlers.HabitDetail)
	g.GET("/habits/:id/stats", handlers.HabitStats)

	g.GET("/sessions", handlers.SessionList)
	g.POST("/sessions", handlers.SessionCreate)
	g.DELETE("/sessions/:id", handlers.SessionDelete)

	g.GET("/timer-sessions", handlers.TimerSessionList)
	g.POST("/timer-sessions", handlers.TimerSessionCreate)
	g.PUT("/timer-sessions/:id", handlers.TimerSessionUpdate)
	g.DELETE("/timer-sessions/:id", handlers.TimerSessionDelete)
}

// -----------------------------------------------------------------------------
// Settings routes.
// -----------------------------------------------------------------------------

func registerSettings(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/settings", handlers.SettingsGet)
	g.POST("/settings", handlers.SettingsUpdate)
}

// -----------------------------------------------------------------------------
// Backup routes.
// -----------------------------------------------------------------------------

func registerBackup(r *gin.Engine) {
	g := r.Group("/api")
	g.GET("/backup/config", handlers.BackupConfigGet)
	g.POST("/backup/config", handlers.BackupConfigUpdate)
	g.POST("/backup/create", handlers.BackupCreate)
	g.POST("/backup/restore", handlers.BackupRestore)
	g.POST("/backup/restore/:name", handlers.BackupRestoreByName)
	g.GET("/backup/list", handlers.BackupList)
	g.GET("/backup/info", handlers.BackupInfo)
	g.DELETE("/backup/delete/:name", handlers.BackupDeleteByName)
	g.DELETE("/backup/:id", handlers.BackupDelete)
	g.POST("/backup/verify", handlers.BackupVerify)
	g.POST("/backup/unlock", handlers.BackupUnlock)
	g.POST("/backup/lock", handlers.BackupLock)
	g.GET("/backup/master-password", handlers.MasterPasswordGet)
	g.POST("/backup/master-password", handlers.MasterPasswordSet)

	g.GET("/auth/status", handlers.AuthStatus)
	g.POST("/auth/enable", handlers.AuthEnable)
	g.POST("/auth/disable", handlers.AuthDisable)
}

// -----------------------------------------------------------------------------
// Wallpaper routes.
// -----------------------------------------------------------------------------

func registerWallpapers(r *gin.Engine) {
	g := r.Group("/api/wallpapers")
	g.POST("", handlers.WallpaperUpload)
	g.GET("", handlers.WallpaperList)
	g.GET("/:id", handlers.WallpaperServe)
	g.DELETE("/:id", handlers.WallpaperDelete)
}

// -----------------------------------------------------------------------------
// SSE + frontend-log routes.
// -----------------------------------------------------------------------------

func registerEvents(r *gin.Engine) {
	r.GET("/api/events", handlers.Events)
	r.POST("/api/log", handlers.FrontendLog)
}
