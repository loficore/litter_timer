// Package middleware contains Gin middleware used by the little-timer HTTP
// server.
//
// File `cors.go` is a permissive CORS handler intended for development
// (frontend running on a different origin).  In production a stricter
// allow-list would be appropriate.
package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that sets the standard CORS headers and
// short-circuits OPTIONS preflight requests.
//
// The headers mirror the Zig std_server.zig behaviour: it ran as a single
// HTTP listener with no CORS layer because the embedded webview shared
// origin with the server.  Now that the webview is optional and the
// frontend can be served from `localhost:5173` (Vite dev) or any other
// host, the http layer needs to allow those origins.
//
// `allowOrigin` controls `Access-Control-Allow-Origin`.  Use "*" for
// "allow everything"; pass a concrete origin to lock down.  Empty string
// is treated as "*".
func CORS(allowOrigin string) gin.HandlerFunc {
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", allowOrigin)
		header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With")
		header.Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		header.Set("Access-Control-Max-Age", "86400")

		// CORS spec forbids Allow-Credentials with wildcard origin.
		if allowOrigin != "*" {
			header.Set("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}