// Package handlers — Wallpaper upload / list / serve / delete.
//
// File `wallpaper.go` ports the wallpaper handlers from std_server.zig.
// Routes:
//
//	POST   /api/wallpapers        → handleWallpaperUpload
//	GET    /api/wallpapers        → handleWallpaperList
//	GET    /api/wallpapers/:id    → handleWallpaperServe
//	DELETE /api/wallpapers/:id    → handleWallpaperDelete
//
// Wallpapers are stored as plain files under
// `<db_dir>/wallpapers/<uuid32>.<ext>`.  The DB is consulted only to derive
// `db_dir` (the parent of the SQLite file).  The file list endpoint scans
// the directory directly.
package handlers

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// wallpapersDir returns the absolute path to the wallpapers directory,
// creating it on first use.  Mirrors `getWallpapersDir` in std_server.zig.
func wallpapersDir(dbPath string) (string, error) {
	dbDir := filepath.Dir(dbPath)
	if dbDir == "" {
		dbDir = "."
	}
	dir := filepath.Join(dbDir, "wallpapers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sanitizeFilename replaces every character outside [a-zA-Z0-9._-] with
// an underscore.  Mirrors Zig `sanitizeFilename`.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// handleWallpaperUpload mirrors `handleUploadWallpaper`.  Accepts a
// `multipart/form-data` request with a single "file" field.
// Uses temp-file + 50MB cap + UUID naming + image compression.
func WallpaperUpload(c *gin.Context) {
	a := appFromCtx(c)

	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "wallpapers dir not available"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "missing file"})
		return
	}
	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}
	defer src.Close()

	// Extension from original filename (sanitised).
	safe := sanitizeFilename(fileHeader.Filename)
	ext := filepath.Ext(filepath.Base(safe))

	// Stream to temp file with 50MB limit.
	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to create temp file"})
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // clean up temp on any exit path

	limitReader := io.LimitReader(src, 50*1024*1024+1)
	written, err := io.Copy(tmp, limitReader)
	if err != nil {
		tmp.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to receive file"})
		return
	}
	if err := tmp.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to flush temp file"})
		return
	}
	if written > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "file too large"})
		return
	}

	// Process the image (decode, scale, re-encode).
	tmpFile, err := os.Open(tmpName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to open temp file"})
		return
	}
	defer tmpFile.Close()

	processed, outExt, err := processWallpaperImage(tmpFile, ext)
	if err != nil {
		switch err {
		case ErrWallpaperDimensionsTooLarge:
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "image dimensions too large"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"err": "invalid image: " + err.Error()})
		}
		return
	}

	finalName := newUUIDHex() + outExt
	dstPath := filepath.Join(dir, finalName)
	if err := os.WriteFile(dstPath, processed, 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to save wallpaper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filename": finalName})
}

// handleWallpaperList mirrors `handleListWallpapers`.  Returns a JSON
// array of {name, size, refs} objects — filename only (no path exposure),
// the on-disk byte size, and the number of rows across habits/habit_sets/
// settings that reference the wallpaper via `local:<name>`.
func WallpaperList(c *gin.Context) {
	a := appFromCtx(c)
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, []any{})
		return
	}
	out := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil {
			// Skip unreadable entries — consistent with "only readable files".
			continue
		}
		refs, err := a.SQLite.CountWallpaperRefs("local:" + e.Name())
		if err != nil {
			// Reference count query failed; treat as unreferenced.
			refs = 0
		}
		out = append(out, gin.H{"name": e.Name(), "size": info.Size(), "refs": refs})
	}
	c.JSON(http.StatusOK, out)
}

// handleWallpaperServe mirrors `handleServeWallpaper`.  Sets the
// Content-Type based on the file extension.
func WallpaperServe(c *gin.Context) {
	a := appFromCtx(c)
	filename := c.Param("id")
	if filename == "" || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid filename"})
		return
	}
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Wallpapers dir not found"})
		return
	}
	filePath := filepath.Join(dir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"err": "File not found"})
		return
	}
	if info.Size() > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "File too large"})
		return
	}
	c.Header("Content-Type", mimeByExt(filepath.Ext(filename)))
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(filePath)
}

// handleWallpaperDelete mirrors `handleDeleteWallpaper`.
//
// Order is critical for consistency: first unbind every DB reference to the
// wallpaper (habits / habit_sets / settings) in a single transaction, then
// physically remove the file.  If the unbind fails we return 500 and leave
// the file untouched — no dangling `local:` refs.  If the file removal fails
// (e.g. missing file) we still return 500; the DB is already unbound, so a
// retry is safe.
func WallpaperDelete(c *gin.Context) {
	a := appFromCtx(c)
	filename := c.Param("id")
	if filename == "" || strings.Contains(filename, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid filename"})
		return
	}
	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Wallpapers dir not found"})
		return
	}
	unbound, err := a.SQLite.UnbindWallpaper("local:" + filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to unbind wallpaper"})
		return
	}
	if err := os.Remove(filepath.Join(dir, filename)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "unbound": unbound})
}

// mimeByExt maps a file extension to a MIME type.  Mirrors the Zig
// extension switch.
func mimeByExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// contentTypeToExt maps a Content-Type header value to a file extension.
// Returns "" for unrecognised types.
//
// The match is case-insensitive per RFC 7231 §3.1.1.1 (media types are
// case-insensitive): "IMAGE/PNG" must map to ".png" just like "image/png".
func contentTypeToExt(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/svg+xml":
		return ".svg"
	case "image/bmp":
		return ".bmp"
	default:
		return ""
	}
}

// errURLHostNotAllowed marks an SSRF-blocked URL host.
var errURLHostNotAllowed = errors.New("URL host not allowed")

// netLookupIP is the hostname resolver used by isBlockedHost; a package var
// so tests can stub it without touching the real DNS.
var netLookupIP = net.LookupIP

// isBlockedIP reports whether an IP falls in a network range that the
// wallpaper fetcher must never contact.  Loopback (127.0.0.0/8, ::1) is
// deliberately ALLOWED: this is a personal desktop app where an attacker
// could already reach the local machine, and keeping loopback open lets the
// dev/test flow use httptest servers bound to 127.0.0.1.  The real SSRF
// targets are cloud metadata (169.254.169.254) and internal network ranges,
// and those are all blocked below.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1) carry the IPv4
	// address inside; normalize so the private/loopback checks see it.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	// Link-local IPv4 (169.254.0.0/16) is NOT covered by net.IP.IsPrivate()
	// and holds the cloud-metadata endpoint 169.254.169.254.
	if ip.IsLinkLocalUnicast() {
		return true
	}
	// Explicitly allow loopback, then block the rest of the non-routable
	// ranges: RFC 1918 private (10/8, 172.16/12, 192.168/16), IPv6 unique
	// local (fc00::/7), link-local, multicast and unspecified addresses.
	if ip.IsLoopback() {
		return false
	}
	return ip.IsPrivate() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalMulticast()
}

// isBlockedHost resolves host (an authority string, port already stripped)
// and reports whether the connection target is SSRF-blocked.  A host that is
// a bare IP literal is parsed directly; otherwise every resolved address is
// checked and the host is blocked if ANY address is blocked (an attacker
// controlling DNS could otherwise resolve a name to a mix of public and
// private addresses and still hit the internal one).  Resolution failures
// are treated conservatively as blocked: if we cannot prove the host is safe,
// we refuse to fetch it.
func isBlockedHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isBlockedIP(ip)
	}
	addrs, err := netLookupIP(host)
	if err != nil {
		return true
	}
	if len(addrs) == 0 {
		return true
	}
	for _, ip := range addrs {
		if isBlockedIP(ip) {
			return true
		}
	}
	return false
}

// WallpaperFromURL downloads a wallpaper from a URL, processes it through
// the same decode/scale/encode pipeline as WallpaperUpload, and saves it
// with a UUID filename.
func WallpaperFromURL(c *gin.Context) {
	a := appFromCtx(c)

	dir, err := wallpapersDir(a.DBPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "wallpapers dir not available"})
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "missing url field"})
		return
	}

	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"err": "invalid url scheme"})
		return
	}

	// SSRF guard: block fetches to private / link-local / metadata ranges
	// for the initial host.  Redirect targets are re-checked in
	// CheckRedirect below (redirects can move the fetch to an internal
	// host even when the original URL was public).
	if isBlockedHost(u.Hostname()) {
		c.JSON(http.StatusBadRequest, gin.H{"err": errURLHostNotAllowed.Error()})
		return
	}

	redirects := 0
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirects++
			if redirects > 5 {
				return fmt.Errorf("too many redirects")
			}
			// Refuse to follow a redirect into a blocked range.
			if isBlockedHost(req.URL.Hostname()) {
				return errURLHostNotAllowed
			}
			return nil
		},
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, req.URL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "invalid url"})
		return
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		if errors.Is(err, errURLHostNotAllowed) {
			c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "Timeout") {
			c.JSON(http.StatusGatewayTimeout, gin.H{"err": "upstream timeout"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"err": "upstream fetch failed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"err": "upstream returned non-2xx"})
		return
	}

	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	ext := contentTypeToExt(ct)
	if ext == "" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"err": "unsupported content type"})
		return
	}

	// Stream to temp file with 50MB limit.
	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to create temp file"})
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	limitReader := io.LimitReader(resp.Body, 50*1024*1024+1)
	written, err := io.Copy(tmp, limitReader)
	if err != nil {
		tmp.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to download file"})
		return
	}
	if err := tmp.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to flush temp file"})
		return
	}
	if written > 50*1024*1024 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "file too large"})
		return
	}

	// Process the image (decode, scale, re-encode).
	tmpFile, err := os.Open(tmpName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to open temp file"})
		return
	}
	defer tmpFile.Close()

	processed, outExt, err := processWallpaperImage(tmpFile, ext)
	if err != nil {
		switch err {
		case ErrWallpaperDimensionsTooLarge:
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"err": "image dimensions too large"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"err": "invalid image: " + err.Error()})
		}
		return
	}

	finalName := newUUIDHex() + outExt
	dstPath := filepath.Join(dir, finalName)
	if err := os.WriteFile(dstPath, processed, 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "failed to save wallpaper"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"filename": finalName})
}