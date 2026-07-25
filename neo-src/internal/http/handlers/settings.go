// Package handlers — Settings endpoints.
//
// File `settings.go` ports `handleGetSettings` / `handleUpdateSettings`
// from std_server.zig.  Both routes return the entire SettingsConfig
// blob (basic, clock_defaults, logging, auth) as JSON.
//
// Routes:
//
//	GET  /api/settings
//	POST /api/settings
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
)

// handleSettingsGet mirrors `handleGetSettings`.
func SettingsGet(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.Config()
	c.JSON(http.StatusOK, cfg)
}

// handleSettingsUpdate mirrors `handleUpdateSettings`.  Body is the
// partial SettingsConfig object — SettingsManager parses it via
// `parseSettingsFromJSON` which is tolerant of missing fields.
func SettingsUpdate(c *gin.Context) {
	a := appFromCtx(c)
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}
	if err := a.Settings.HandleSettingsEvent(domain.SettingsChangeEvent{JSON: string(raw)}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	// Sanity check: response must be valid JSON object (Gin's c.JSON
	// refuses to encode a nil interface).
	out := gin.H{"status": "settings_updated"}
	c.JSON(http.StatusOK, out)
}
