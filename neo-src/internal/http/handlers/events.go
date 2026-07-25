// Package handlers — Server-Sent Events stream.
//
// File `events.go` ports `handleSSE` from std_server.zig.  Streams
// `state_changed` / `tick` events to the connected client at 1Hz,
// matching the original loop.
//
// Wire format: `event: <type>\ndata: <json>\n\n`, where each event has
// an `event:` line naming its type.  Mirrors the Zig `body_writer.print
// ("data: {s}\n\n", ...)` shape — we add an explicit `event:` line so
// the frontend's EventSource can route on event name.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleEvents mirrors `handleSSE`.  Sends one `state_changed` event
// immediately on connect, then one `tick` event per second.
//
// On the Zig source the loop runs `app.clock_manager.update()` and
// emits the resulting `state_json` as `data:`.  We follow the same
// shape but emit both an `event:` line and a `data:` line so the
// browser's EventSource can `addEventListener('tick', …)` etc.
//
// Connection limits (1h max session, 30s max heartbeat gap) match the
// Zig source's `max_session_seconds` / `max_heartbeat_gap_seconds`.
func Events(c *gin.Context) {
	a := appFromCtx(c)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "streaming unsupported"})
		return
	}

	// Initial state push so the client doesn't sit waiting for the
	// first tick.
	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()
	state := a.Clock.Update()
	writeSSE(c.Writer, "state_changed", buildStateResponse(state, modeKey(state.GetMode()), a.Settings.Config().Basic.Timezone, habitID))
	flusher.Flush()

	const (
		maxSession = time.Hour
		heartbeat  = 10 * time.Second
		maxGap     = 30 * time.Second
	)
	start := time.Now()
	lastHeartbeat := time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			if time.Since(start) > maxSession {
				return
			}
			if time.Since(lastHeartbeat) > maxGap {
				return
			}
			a.RLock()
			habitID := a.CurrentHabitID
			a.RUnlock()
			state := a.Clock.Update()
			writeSSE(c.Writer, "tick", buildStateResponse(state, modeKey(state.GetMode()), a.Settings.Config().Basic.Timezone, habitID))
			if time.Since(lastHeartbeat) >= heartbeat {
				lastHeartbeat = time.Now()
				_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
			}
			flusher.Flush()
		}
	}
}

// writeSSE writes a single SSE frame of the form
//
//	event: <name>\ndata: <json>\n\n
//
// to the supplied writer.  The data line is a single line of JSON;
// embedded newlines are not escaped (matches the Zig behaviour — the
// state JSON never contains a literal newline).
func writeSSE(w http.ResponseWriter, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		// Fall back to an empty event so the stream stays alive.
		_, _ = fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
}

// handleFrontendLog mirrors `handleFrontendLog` — receives a JSON
// log entry from the browser and re-emits it via the server logger.
// The handler is exposed at `POST /api/log` (public, no auth).
func FrontendLog(c *gin.Context) {
	var entry struct {
		Category string `json:"category"`
		Level    string `json:"level"`
		Message  string `json:"message"`
		Runtime  string `json:"runtime"`
	}
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "err": "empty body"})
		return
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "err": "invalid json"})
		return
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	prefix := fmt.Sprintf("[frontend:%s][%s]", entry.Category, entry.Runtime)
	switch entry.Level {
	case "error":
		// ponytail: gin's release-mode logger strips debug lines,
		// so we route everything through fmt.Fprintf to stderr.
		fmt.Fprintf(c.Writer, "%s ERROR: %s\n", prefix, entry.Message)
	default:
		fmt.Fprintf(c.Writer, "%s %s: %s\n", prefix, entry.Level, entry.Message)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
