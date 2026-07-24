package handlers

import (
	"github.com/gin-gonic/gin"
)

// This file exposes the package-internal handler functions as exported
// names so the router can reference them directly.  Each export is a
// one-line wrapper that preserves the handler signature so middleware
// chaining remains straightforward.
//
// The lower-case originals stay where they are — that's where the
// actual logic lives.

// -----------------------------------------------------------------------------
// Timer.
// -----------------------------------------------------------------------------

func TimerState(c *gin.Context)         { handleGetState(c) }
func TimerProgress(c *gin.Context)      { handleGetProgress(c) }
func TimerStart(c *gin.Context)         { handleStart(c) }
func TimerPause(c *gin.Context)         { handlePause(c) }
func TimerReset(c *gin.Context)         { handleReset(c) }
func TimerFinish(c *gin.Context)        { handleFinish(c) }
func TimerMode(c *gin.Context)          { handleModeSwitch(c) }
func TimerStartRest(c *gin.Context)     { handleStartRest(c) }
func TimerConfig(c *gin.Context)        { handleConfig(c) }
func TimerUpdateConfig(c *gin.Context)  { handleUpdateConfig(c) }

// -----------------------------------------------------------------------------
// Habit sets / habits / sessions / timer-sessions.
// -----------------------------------------------------------------------------

func HabitSetList(c *gin.Context)            { handleHabitSetList(c) }
func HabitSetCreate(c *gin.Context)          { handleHabitSetCreate(c) }
func HabitSetUpdate(c *gin.Context)          { handleHabitSetUpdate(c) }
func HabitSetDelete(c *gin.Context)          { handleHabitSetDelete(c) }

func HabitList(c *gin.Context)               { handleHabitList(c) }
func HabitCreate(c *gin.Context)             { handleHabitCreate(c) }
func HabitUpdate(c *gin.Context)             { handleHabitUpdate(c) }
func HabitDelete(c *gin.Context)             { handleHabitDelete(c) }
func HabitDetail(c *gin.Context)             { handleHabitDetail(c) }

func SessionList(c *gin.Context)             { handleSessionList(c) }
func SessionCreate(c *gin.Context)           { handleSessionCreate(c) }

func TimerSessionList(c *gin.Context)        { handleTimerSessionList(c) }
func TimerSessionCreate(c *gin.Context)      { handleTimerSessionCreate(c) }
func TimerSessionUpdate(c *gin.Context)      { handleTimerSessionUpdate(c) }
func TimerSessionDelete(c *gin.Context)      { handleTimerSessionDelete(c) }

// -----------------------------------------------------------------------------
// Settings.
// -----------------------------------------------------------------------------

func SettingsGet(c *gin.Context)             { handleSettingsGet(c) }
func SettingsUpdate(c *gin.Context)          { handleSettingsUpdate(c) }

// -----------------------------------------------------------------------------
// Backup + master password + auth.
// -----------------------------------------------------------------------------

func BackupConfigGet(c *gin.Context)         { handleBackupConfigGet(c) }
func BackupConfigUpdate(c *gin.Context)      { handleBackupConfigUpdate(c) }
func BackupCreate(c *gin.Context)            { handleBackupCreate(c) }
func BackupRestore(c *gin.Context)           { handleBackupRestore(c) }
func BackupRestoreByName(c *gin.Context)     { handleBackupRestoreByName(c) }
func BackupList(c *gin.Context)              { handleBackupList(c) }
func BackupInfo(c *gin.Context)              { handleBackupInfo(c) }
func BackupDeleteByName(c *gin.Context)     { handleBackupDeleteByName(c) }
func BackupDelete(c *gin.Context)            { handleBackupDelete(c) }
func BackupVerify(c *gin.Context)            { handleBackupVerify(c) }
func BackupUnlock(c *gin.Context)            { handleBackupUnlock(c) }
func BackupLock(c *gin.Context)              { handleBackupLock(c) }

func MasterPasswordGet(c *gin.Context)       { handleMasterPasswordGet(c) }
func MasterPasswordSet(c *gin.Context)       { handleMasterPasswordSet(c) }

func AuthStatus(c *gin.Context)              { handleAuthStatus(c) }
func AuthEnable(c *gin.Context)              { handleAuthEnable(c) }
func AuthDisable(c *gin.Context)             { handleAuthDisable(c) }

// -----------------------------------------------------------------------------
// Wallpapers.
// -----------------------------------------------------------------------------

func WallpaperUpload(c *gin.Context)         { handleWallpaperUpload(c) }
func WallpaperList(c *gin.Context)           { handleWallpaperList(c) }
func WallpaperServe(c *gin.Context)          { handleWallpaperServe(c) }
func WallpaperDelete(c *gin.Context)         { handleWallpaperDelete(c) }
func WallpaperFromURL(c *gin.Context)        { handleWallpaperFromURL(c) }

// -----------------------------------------------------------------------------
// SSE + frontend log.
// -----------------------------------------------------------------------------

func Events(c *gin.Context)                  { handleEvents(c) }
func FrontendLog(c *gin.Context)             { handleFrontendLog(c) }