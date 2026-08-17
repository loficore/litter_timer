package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

func newWallpaperTestApp(t *testing.T) (*app.App, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	wallpaperDir := tmpDir + "/wallpapers"

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })

	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}

	if err := os.MkdirAll(wallpaperDir, 0o700); err != nil {
		t.Fatalf("mkdir wallpapers: %v", err)
	}

	a := app.NewApp(
		domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm,
		sqlite,
		nil,
		dbPath,
	)
	return a, wallpaperDir
}

func createTestWallpaper(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write wallpaper: %v", err)
	}
	return name
}

// createTestPNG generates a valid PNG of the given size (pixels) and returns the bytes.
func createTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buf.Bytes()
}

// createTestJPEG generates a valid JPEG of the given size and returns the bytes.
func createTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	return buf.Bytes()
}

// testWebpBytes is a valid lossless webp from golang.org/x/image/testdata.
var testWebpBytes = []byte{
	0x52, 0x49, 0x46, 0x46, 0xb2, 0x01, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50, 0x56, 0x50, 0x38, 0x4c,
	0xa5, 0x01, 0x00, 0x00, 0x2f, 0x4a, 0xc0, 0x18, 0x00, 0x0f, 0x30, 0xff, 0xf3, 0x3f, 0xff, 0xf3,
	0x1f, 0x78, 0x90, 0x24, 0x6d, 0x7b, 0xda, 0x48, 0x6e, 0xe6, 0xf1, 0x0d, 0xc6, 0x7d, 0x84, 0x81,
	0x25, 0xe9, 0x30, 0x43, 0x3b, 0x66, 0xfc, 0x87, 0x19, 0x96, 0x0c, 0x27, 0x99, 0x62, 0x26, 0x9f,
	0x60, 0x4a, 0xed, 0xa1, 0x66, 0x06, 0xd9, 0xd5, 0x8a, 0xbe, 0xaa, 0xff, 0xff, 0x15, 0x3a, 0x41,
	0x44, 0xff, 0x19, 0xb8, 0x6d, 0xa4, 0xc8, 0xbb, 0xc7, 0x38, 0xf0, 0x0a, 0xc4, 0xa3, 0xaf, 0x81,
	0xdf, 0x31, 0x4a, 0x62, 0x59, 0xf7, 0xa6, 0xa0, 0xa5, 0x48, 0x22, 0x97, 0xd1, 0xb7, 0xa0, 0x15,
	0x30, 0x17, 0x14, 0xe2, 0xd7, 0x1d, 0x2c, 0x85, 0xf1, 0xc0, 0x8d, 0x71, 0x91, 0x06, 0xe0, 0xec,
	0xb0, 0xb8, 0x0e, 0x0a, 0x55, 0x57, 0xc9, 0x0a, 0x20, 0x2b, 0x53, 0xb1, 0x80, 0x80, 0x92, 0x3c,
	0xfa, 0x52, 0x4f, 0xfc, 0xe2, 0x8c, 0x4f, 0xf7, 0xc1, 0x02, 0x37, 0xaf, 0x83, 0x57, 0x18, 0x07,
	0xb6, 0x15, 0x90, 0x5b, 0x96, 0x81, 0xad, 0xa5, 0xc8, 0xf8, 0xb9, 0x23, 0x41, 0xc5, 0xcb, 0x96,
	0x13, 0xa5, 0x62, 0x07, 0x83, 0x44, 0x59, 0xa6, 0x49, 0xe2, 0x45, 0x55, 0xbd, 0xa1, 0xd1, 0xc0,
	0x28, 0xec, 0x28, 0xb1, 0x6b, 0x8e, 0x19, 0xdc, 0x48, 0xca, 0x7d, 0x8e, 0xbd, 0xa0, 0x83, 0xbe,
	0x18, 0x3f, 0xc1, 0xee, 0x93, 0xc1, 0xa7, 0x4f, 0x04, 0xf6, 0xea, 0x05, 0x5e, 0x7c, 0x32, 0xc2,
	0xe6, 0x30, 0x9f, 0x32, 0x66, 0x73, 0x96, 0x93, 0xc4, 0x91, 0xcf, 0x83, 0x7e, 0x42, 0x8c, 0x8f,
	0x2f, 0xe3, 0x27, 0x6a, 0x6c, 0xcc, 0xbd, 0xc1, 0x35, 0xac, 0x73, 0x44, 0xaf, 0xdd, 0x45, 0xf4,
	0x62, 0x99, 0x3d, 0x55, 0x1c, 0x4b, 0xdc, 0x3b, 0x3e, 0x18, 0x47, 0xdf, 0xab, 0x2e, 0x07, 0xda,
	0x8f, 0x79, 0x86, 0xff, 0xa0, 0xb9, 0x3a, 0x72, 0xe4, 0xe2, 0x27, 0x4c, 0x0e, 0x2b, 0x79, 0xb9,
	0x87, 0x57, 0x0a, 0x8d, 0x6e, 0x84, 0x55, 0x90, 0x98, 0x30, 0xae, 0xdd, 0xc5, 0xc2, 0x82, 0x05,
	0xd8, 0x0f, 0xf4, 0x79, 0x0a, 0xaf, 0xd8, 0x24, 0x00, 0xed, 0x8f, 0xf0, 0x62, 0x99, 0x19, 0x65,
	0x5d, 0x20, 0x06, 0xad, 0x41, 0xaf, 0xb5, 0x20, 0x3a, 0x6d, 0xea, 0xac, 0xa8, 0xad, 0x5c, 0x1d,
	0xcb, 0x4d, 0x71, 0x75, 0x6f, 0x09, 0x91, 0xf9, 0x3a, 0xc6, 0x31, 0x17, 0x99, 0x54, 0x10, 0xf8,
	0x74, 0x1d, 0x16, 0xbe, 0x8e, 0x2a, 0x12, 0x0d, 0xdf, 0x87, 0x57, 0x5a, 0xad, 0x3e, 0xd2, 0xaa,
	0xfa, 0x10, 0x94, 0x82, 0x79, 0xe5, 0x4b, 0x1f, 0xdf, 0xa0, 0xbc, 0x64, 0xcb, 0xca, 0xa3, 0x3a,
	0xe4, 0xf4, 0x38, 0xe2, 0x28, 0x73, 0x95, 0x35, 0xf1, 0x40, 0xa8, 0xca, 0x6c, 0x0b, 0xec, 0x85,
	0x78, 0x22, 0xaf, 0xb2, 0xe2, 0x97, 0xdc, 0x38, 0x2f, 0x66, 0xef, 0x33, 0x27, 0x26, 0x8d, 0x07,
	0x2a, 0x5d, 0xa3, 0x02, 0x3b, 0xa0, 0x65, 0x63, 0x6f, 0x22, 0xf8, 0x53, 0x8b, 0xcd, 0xb7, 0xc8,
	0xd6, 0xf1, 0x2a, 0xc4, 0x08, 0x68, 0xb6, 0x87, 0x00, 0x00,
}

// uuidFilenameRe matches UUID-based filenames: 32 lowercase hex chars + dot + extension.
var uuidFilenameRe = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|gif|svg|bmp)$`)

func TestHandleWallpaperUpload(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// Generate a real PNG so the decode pipeline works.
	pngData := createTestPNG(t, 100, 100)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, err = part.Write(pngData)
	if err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["filename"] == nil {
		t.Errorf("missing filename in response")
	}
	filename := got["filename"].(string)
	if !uuidFilenameRe.MatchString(filename) {
		t.Errorf("filename %q does not match UUID pattern %s", filename, uuidFilenameRe.String())
	}
	if !strings.HasSuffix(filename, ".png") {
		t.Errorf("expected .png extension, got %q", filename)
	}

	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) == 0 {
		t.Errorf("no wallpaper files created in %s", wallpaperDir)
	}
}

func TestHandleWallpaperUpload_MissingFile(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers", strings.NewReader(""))
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// TestHandleWallpaperUpload_InvalidImage verifies that non-image data
// (e.g. text pretending to be PNG) is rejected with 400.
func TestHandleWallpaperUpload_InvalidImage(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "fake.png")
	part.Write([]byte("not a real png"))
	writer.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400 for invalid image data", w.Code)
	}

	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) != 0 {
		t.Errorf("expected no files for invalid image, got %d", len(entries))
	}
}

// TestHandleWallpaperUpload_Resize verifies that images with a long edge
// >2560px are scaled down.
func TestHandleWallpaperUpload_Resize(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// 4000×3000 PNG — long edge 4000 > 2560, should be scaled to 2560×1920.
	pngData := createTestPNG(t, 4000, 3000)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "big.png")
	part.Write(pngData)
	writer.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	filename := got["filename"].(string)

	// Read back the saved file and check its dimensions.
	filePath := filepath.Join(wallpaperDir, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode saved image: %v", err)
	}
	if cfg.Width > 2560 || cfg.Height > 2560 {
		t.Errorf("expected dimensions ≤2560, got %d×%d", cfg.Width, cfg.Height)
	}
	// Long edge should be exactly 2560 (or at least close, allowing minor rounding).
	if cfg.Width != 2560 && cfg.Height != 2560 {
		t.Errorf("expected long edge 2560, got %d×%d", cfg.Width, cfg.Height)
	}
}

// TestHandleWallpaperUpload_DimensionsTooLarge verifies that images
// >12000px in either dimension are rejected with 413.
func TestHandleWallpaperUpload_DimensionsTooLarge(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// 12001×1 PNG — exceeds the 12000px limit.
	pngData := createTestPNG(t, 12001, 1)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "huge.png")
	part.Write(pngData)
	writer.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code = %d, want 413 for >12000px image", w.Code)
	}

	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) != 0 {
		t.Errorf("expected no partial files, got %d entries", len(entries))
	}
}

// TestHandleWallpaperUpload_GIFPassthrough verifies that GIF files are
// stored unchanged (no decode/re-encode).
func TestHandleWallpaperUpload_GIFPassthrough(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	gifData := []byte("GIF89a\000\000\000\000\000\000")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.gif")
	part.Write(gifData)
	writer.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	c.Set("app", a)

	WallpaperUpload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	filename := got["filename"].(string)
	if !uuidFilenameRe.MatchString(filename) {
		t.Errorf("filename %q does not match UUID pattern", filename)
	}
	if !strings.HasSuffix(filename, ".gif") {
		t.Errorf("expected .gif extension, got %q", filename)
	}

	// Verify content is unchanged.
	filePath := filepath.Join(wallpaperDir, filename)
	saved, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(saved, gifData) {
		t.Errorf("GIF content changed")
	}
}

// TestHandleWallpaperUpload_UniqueNames verifies that two uploads in the
// same second produce different filenames (no overwrite).
func TestHandleWallpaperUpload_UniqueNames(t *testing.T) {
	a, _ := newWallpaperTestApp(t)

	pngData := createTestPNG(t, 10, 10)

	upload := func() string {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "same.png")
		part.Write(pngData)
		writer.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/wallpapers", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		c.Request = req
		c.Set("app", a)
		WallpaperUpload(c)
		if w.Code != http.StatusOK {
			t.Fatalf("upload failed: %d", w.Code)
		}
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		return got["filename"].(string)
	}

	f1 := upload()
	f2 := upload()

	if f1 == f2 {
		t.Errorf("two uploads produced same filename %q", f1)
	}
	if !uuidFilenameRe.MatchString(f1) || !uuidFilenameRe.MatchString(f2) {
		t.Errorf("filenames don't match UUID pattern: %q, %q", f1, f2)
	}
}

func TestHandleWallpaperList(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	createTestWallpaper(t, wallpaperDir, "test1.png", []byte("content1"))
	createTestWallpaper(t, wallpaperDir, "test2.jpg", []byte("content2"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers", nil)
	c.Set("app", a)

	WallpaperList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected 2 wallpapers, got %d", len(got))
	}

	names := make(map[string]bool)
	for _, w := range got {
		if name, ok := w["name"].(string); ok {
			names[name] = true
		}
	}
	if !names["test1.png"] || !names["test2.jpg"] {
		t.Errorf("expected test1.png and test2.jpg in list, got %v", names)
	}
}

func TestHandleWallpaperList_SizeAndRefs(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// Two real PNG wallpapers on disk.
	fileA := createTestWallpaper(t, wallpaperDir, "refA.png", createTestPNG(t, 10, 10))
	fileB := createTestWallpaper(t, wallpaperDir, "refB.png", createTestPNG(t, 20, 20))

	// Seed a habit_set + habit that references local:<fileA>.  habits.set_id
	// is NOT NULL with FK → habit_sets, so create the set first.
	if _, err := a.SQLite.DB().Exec(`INSERT INTO habit_sets (name) VALUES (?)`, "test-set"); err != nil {
		t.Fatalf("insert habit_set: %v", err)
	}
	if _, err := a.SQLite.DB().Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper) VALUES (1, ?, 0, '#6366f1', ?)`,
		"habit-a", "local:"+fileA,
	); err != nil {
		t.Fatalf("insert habit: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers", nil)
	c.Set("app", a)

	WallpaperList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 wallpapers, got %d: %s", len(got), w.Body.String())
	}

	byName := make(map[string]map[string]any, len(got))
	for _, item := range got {
		byName[item["name"].(string)] = item
	}

	for _, f := range []string{fileA, fileB} {
		item, ok := byName[f]
		if !ok {
			t.Fatalf("wallpaper %q missing from list", f)
		}
		size, ok := item["size"].(float64)
		if !ok || size <= 0 {
			t.Errorf("wallpaper %q: size = %v, want > 0", f, item["size"])
		}
	}

	refsA, _ := byName[fileA]["refs"].(float64)
	if refsA != 1 {
		t.Errorf("wallpaper %q: refs = %v, want 1", fileA, byName[fileA]["refs"])
	}
	refsB, _ := byName[fileB]["refs"].(float64)
	if refsB != 0 {
		t.Errorf("wallpaper %q: refs = %v, want 0", fileB, byName[fileB]["refs"])
	}
}

func TestHandleWallpaperList_Empty(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers", nil)
	c.Set("app", a)

	WallpaperList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got []any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %d items", len(got))
	}
}

func TestHandleWallpaperServe(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)
	filename := createTestWallpaper(t, wallpaperDir, "serve_test.png", []byte("image content"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers/"+filename, nil)
	c.Params = gin.Params{{Key: "id", Value: filename}}
	c.Set("app", a)

	WallpaperServe(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}

	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want public, max-age=86400", cc)
	}
}

func TestHandleWallpaperServe_NotFound(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers/nonexistent.png", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent.png"}}
	c.Set("app", a)

	WallpaperServe(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandleWallpaperServe_InvalidFilename(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	tests := []string{"", "../etc/passwd", "path/with/slash.png"}

	for _, filename := range tests {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers/"+filename, nil)
		c.Set("app", a)

		WallpaperServe(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("filename %q: code = %d, want 400", filename, w.Code)
		}
	}
}

func TestHandleWallpaperDelete(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)
	filename := createTestWallpaper(t, wallpaperDir, "delete_test.png", []byte("to delete"))

	path := filepath.Join(wallpaperDir, filename)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("test file doesn't exist: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/wallpapers/"+filename, nil)
	c.Params = gin.Params{{Key: "id", Value: filename}}
	c.Set("app", a)

	WallpaperDelete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if unbound, ok := got["unbound"].(float64); !ok || unbound != 0 {
		t.Errorf("unbound = %v, want 0 for an unreferenced wallpaper", got["unbound"])
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after deletion")
	}
}

// TestHandleWallpaperDelete_UnbindsRefs verifies that deleting a wallpaper
// referenced by habits first clears the wallpaper column (via the
// transactional UnbindWallpaper) and returns the unbound count, then removes
// the file.  Two habits bound to the same wallpaper => unbound=2 and both
// rows' wallpaper column becomes ''.
func TestHandleWallpaperDelete_UnbindsRefs(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)
	filename := createTestWallpaper(t, wallpaperDir, "bound_delete.png", []byte("to delete"))

	// Seed a habit_set first — habits.set_id is NOT NULL FK to habit_sets.
	if _, err := a.SQLite.DB().Exec(`INSERT INTO habit_sets (name) VALUES (?)`, "set-A"); err != nil {
		t.Fatalf("insert habit_set: %v", err)
	}
	// Two habits referencing the same wallpaper.
	for _, name := range []string{"habit-1", "habit-2"} {
		if _, err := a.SQLite.DB().Exec(
			`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper) VALUES (1, ?, 0, '#000000', ?)`,
			name, "local:"+filename,
		); err != nil {
			t.Fatalf("insert habit %q: %v", name, err)
		}
	}

	// Sanity: refs before delete.
	refs, err := a.SQLite.CountWallpaperRefs("local:" + filename)
	if err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if refs != 2 {
		t.Fatalf("refs before delete = %d, want 2", refs)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/wallpapers/"+filename, nil)
	c.Params = gin.Params{{Key: "id", Value: filename}}
	c.Set("app", a)

	WallpaperDelete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if unbound, ok := got["unbound"].(float64); !ok || unbound != 2 {
		t.Errorf("unbound = %v, want 2", got["unbound"])
	}

	// File physically removed.
	if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); !os.IsNotExist(err) {
		t.Errorf("file still exists after deletion")
	}

	// Both habits' wallpaper column now empty.
	var w1, w2 string
	if err := a.SQLite.DB().QueryRow(`SELECT wallpaper FROM habits WHERE name = 'habit-1'`).Scan(&w1); err != nil {
		t.Fatalf("read habit-1 wallpaper: %v", err)
	}
	if err := a.SQLite.DB().QueryRow(`SELECT wallpaper FROM habits WHERE name = 'habit-2'`).Scan(&w2); err != nil {
		t.Fatalf("read habit-2 wallpaper: %v", err)
	}
	if w1 != "" || w2 != "" {
		t.Errorf("habits not unbound: habit-1=%q habit-2=%q, want both ''", w1, w2)
	}
}

// TestHandleWallpaperDelete_Unbound verifies that deleting a wallpaper with
// no DB references still succeeds and reports unbound=0.
func TestHandleWallpaperDelete_Unbound(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)
	filename := createTestWallpaper(t, wallpaperDir, "unbound_delete.png", []byte("to delete"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/wallpapers/"+filename, nil)
	c.Params = gin.Params{{Key: "id", Value: filename}}
	c.Set("app", a)

	WallpaperDelete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if unbound, ok := got["unbound"].(float64); !ok || unbound != 0 {
		t.Errorf("unbound = %v, want 0", got["unbound"])
	}

	if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); !os.IsNotExist(err) {
		t.Errorf("file still exists after deletion")
	}
}

func TestHandleWallpaperDelete_NotFound(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/wallpapers/nonexistent.png", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent.png"}}
	c.Set("app", a)

	WallpaperDelete(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestHandleWallpaperDelete_InvalidFilename(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	tests := []string{"", "../etc/passwd"}

	for _, filename := range tests {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/wallpapers/"+filename, nil)
		c.Set("app", a)

		WallpaperDelete(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("filename %q: code = %d, want 400", filename, w.Code)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"test.png", "test.png"},
		{"test file.png", "test_file.png"},
		{"test/../evil.png", "test_.._evil.png"},
		{"test<script>.png", "test_script_.png"},
		{"日本語.png", "___.png"},
		{"file with spaces.jpg", "file_with_spaces.jpg"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMimeByExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".png", "image/png"},
		{".gif", "image/gif"},
		{".webp", "image/webp"},
		{".svg", "image/svg+xml"},
		{".bmp", "image/bmp"},
		{".unknown", "application/octet-stream"},
		{".JPG", "image/jpeg"},
	}

	for _, tt := range tests {
		if got := mimeByExt(tt.ext); got != tt.want {
			t.Errorf("mimeByExt(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestWallpapersDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	dir, err := wallpapersDir(dbPath)
	if err != nil {
		t.Fatalf("wallpapersDir: %v", err)
	}

	expected := filepath.Join(tmpDir, "wallpapers")
	if dir != expected {
		t.Errorf("wallpapersDir = %q, want %q", dir, expected)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("wallpapers directory not created: %v", err)
	}
}

// ---------------------------------------------------------------------------
// processWallpaperImage unit tests
// ---------------------------------------------------------------------------

func TestProcessWallpaperImage_PNG(t *testing.T) {
	pngData := createTestPNG(t, 100, 100)
	out, outExt, err := processWallpaperImage(bytes.NewReader(pngData), ".png")
	if err != nil {
		t.Fatalf("processWallpaperImage: %v", err)
	}
	if outExt != ".png" {
		t.Errorf("outExt = %q, want .png", outExt)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if cfg.Width != 100 || cfg.Height != 100 {
		t.Errorf("output dimensions = %d×%d, want 100×100", cfg.Width, cfg.Height)
	}
}

func TestProcessWallpaperImage_JPEG(t *testing.T) {
	jpegData := createTestJPEG(t, 200, 150)
	out, outExt, err := processWallpaperImage(bytes.NewReader(jpegData), ".jpg")
	if err != nil {
		t.Fatalf("processWallpaperImage: %v", err)
	}
	if outExt != ".jpg" {
		t.Errorf("outExt = %q, want .jpg", outExt)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if cfg.Width != 200 || cfg.Height != 150 {
		t.Errorf("output dimensions = %d×%d, want 200×150", cfg.Width, cfg.Height)
	}
}

func TestProcessWallpaperImage_Resize(t *testing.T) {
	// 4000×3000 — long edge 4000, should scale to 2560×1920.
	pngData := createTestPNG(t, 4000, 3000)
	out, outExt, err := processWallpaperImage(bytes.NewReader(pngData), ".png")
	if err != nil {
		t.Fatalf("processWallpaperImage: %v", err)
	}
	if outExt != ".png" {
		t.Errorf("outExt = %q, want .png", outExt)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if cfg.Width != 2560 || cfg.Height != 1920 {
		t.Errorf("expected 2560×1920, got %d×%d", cfg.Width, cfg.Height)
	}
}

func TestProcessWallpaperImage_WebpToJPEG(t *testing.T) {
	// webp produces .jpg output.
	out, outExt, err := processWallpaperImage(bytes.NewReader(testWebpBytes), ".webp")
	if err != nil {
		t.Fatalf("processWallpaperImage webp: %v", err)
	}
	if outExt != ".jpg" {
		t.Errorf("outExt = %q, want .jpg (webp→jpeg transcode)", outExt)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		t.Errorf("expected non-zero dimensions, got %d×%d", cfg.Width, cfg.Height)
	}
}

func TestProcessWallpaperImage_GIFPassthrough(t *testing.T) {
	gifData := []byte("GIF89a\000\000\000\000\000\000")
	out, outExt, err := processWallpaperImage(bytes.NewReader(gifData), ".gif")
	if err != nil {
		t.Fatalf("processWallpaperImage: %v", err)
	}
	if outExt != ".gif" {
		t.Errorf("outExt = %q, want .gif", outExt)
	}
	if !bytes.Equal(out, gifData) {
		t.Errorf("GIF passthrough changed bytes")
	}
}

func TestProcessWallpaperImage_DimensionsTooLarge(t *testing.T) {
	// 12001×1
	pngData := createTestPNG(t, 12001, 1)
	_, _, err := processWallpaperImage(bytes.NewReader(pngData), ".png")
	if err != ErrWallpaperDimensionsTooLarge {
		t.Errorf("err = %v, want ErrWallpaperDimensionsTooLarge", err)
	}
}

func TestProcessWallpaperImage_UnsupportedFormat(t *testing.T) {
	_, _, err := processWallpaperImage(bytes.NewReader([]byte("data")), ".tiff")
	if err != ErrWallpaperUnsupportedFormat {
		t.Errorf("err = %v, want ErrWallpaperUnsupportedFormat", err)
	}
}

func TestNewUUIDHex(t *testing.T) {
	u := newUUIDHex()
	if len(u) != 32 {
		t.Errorf("uuid length = %d, want 32", len(u))
	}
	match, _ := regexp.MatchString(`^[0-9a-f]{32}$`, u)
	if !match {
		t.Errorf("uuid %q does not match hex pattern", u)
	}

	// Uniqueness check.
	u2 := newUUIDHex()
	if u == u2 {
		t.Errorf("two UUIDs are identical: %q", u)
	}
}

// ---------------------------------------------------------------------------
// POST /api/wallpapers/from-url  tests
// ---------------------------------------------------------------------------

func TestHandleWallpaperFromURL(t *testing.T) {
	t.Run("FromURL_Happy", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		pngData := createTestPNG(t, 100, 100)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngData)
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/test.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
		}
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("response not JSON: %v", err)
		}
		if got["filename"] == nil || got["filename"].(string) == "" {
			t.Errorf("missing or empty filename in response")
		}
		filename := got["filename"].(string)
		if !uuidFilenameRe.MatchString(filename) {
			t.Errorf("filename %q does not match UUID pattern", filename)
		}
		filePath := filepath.Join(wallpaperDir, filename)
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("file not saved: %v", err)
		}
	})

	t.Run("FromURL_ContentTypeWithParams", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		pngData := createTestPNG(t, 100, 100)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png; charset=binary")
			w.Write(pngData)
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/img.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
		}
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		filename := got["filename"].(string)
		if !strings.HasSuffix(filename, ".png") {
			t.Errorf("filename %q should end with .png", filename)
		}
		if !uuidFilenameRe.MatchString(filename) {
			t.Errorf("filename %q does not match UUID pattern", filename)
		}
		if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); err != nil {
			t.Errorf("file not saved: %v", err)
		}
	})

	t.Run("FromURL_Redirect", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		jpegData := createTestJPEG(t, 100, 100)
		finalSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpegData)
		}))
		defer finalSrv.Close()

		redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, finalSrv.URL+"/real.jpg", http.StatusMovedPermanently)
		}))
		defer redirectSrv.Close()

		body, _ := json.Marshal(map[string]string{"url": redirectSrv.URL + "/redirect.jpg"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
		}
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		if _, err := os.Stat(filepath.Join(wallpaperDir, got["filename"].(string))); err != nil {
			t.Errorf("file not saved after redirect: %v", err)
		}
	})

	t.Run("FromURL_NonImage", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>not an image</html>"))
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/page.html"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("code = %d, want 415", w.Code)
		}
		entries, _ := os.ReadDir(wallpaperDir)
		if len(entries) != 0 {
			t.Errorf("expected no files in wallpapers dir, got %d", len(entries))
		}
	})

	t.Run("FromURL_TooLarge", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			// Write 50MB + 1 byte
			chunk := make([]byte, 1024*1024)
			for i := 0; i < 50; i++ {
				w.Write(chunk)
			}
			w.Write([]byte("x"))
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/big.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("code = %d, want 413", w.Code)
		}
		entries, _ := os.ReadDir(wallpaperDir)
		if len(entries) != 0 {
			t.Errorf("expected no partial files, got %d entries", len(entries))
		}
	})

	t.Run("FromURL_Timeout", func(t *testing.T) {
		a, _ := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(35 * time.Second)
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/slow.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// ponytail: 5s client timeout for the test — 35s server sleep triggers it
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req := httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		req = req.WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		c.Request = req
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusGatewayTimeout {
			t.Errorf("code = %d, want 504", w.Code)
		}
	})

	t.Run("FromURL_UpstreamError", func(t *testing.T) {
		a, _ := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/gone.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusBadGateway {
			t.Errorf("code = %d, want 502", w.Code)
		}
	})

	t.Run("FromURL_MissingField", func(t *testing.T) {
		a, _ := newWallpaperTestApp(t)

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", strings.NewReader(""))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("code = %d, want 400", w.Code)
		}
	})

	t.Run("FromURL_BadScheme", func(t *testing.T) {
		a, _ := newWallpaperTestApp(t)

		body, _ := json.Marshal(map[string]string{"url": "file:///etc/passwd"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("code = %d, want 400", w.Code)
		}
	})

	t.Run("FromURL_WebpToJPEG", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		// Serve a valid webp; the handler should transcode to .jpg.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/webp")
			w.Write(testWebpBytes)
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/test.webp"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		WallpaperFromURL(c)

		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
		}
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		filename := got["filename"].(string)
		if !strings.HasSuffix(filename, ".jpg") {
			t.Errorf("webp→jpeg: expected .jpg suffix, got %q", filename)
		}
		if !uuidFilenameRe.MatchString(filename) {
			t.Errorf("filename %q does not match UUID pattern", filename)
		}
		if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); err != nil {
			t.Errorf("file not saved: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// SSRF guard tests (WallpaperFromURL host / redirect checks)
// ---------------------------------------------------------------------------

func TestIsBlockedHost(t *testing.T) {
	cases := []struct {
		name string
		host string
		want bool
	}{
		// Blocked: cloud metadata + non-routable private / link-local ranges.
		{"metadata ipv4", "169.254.169.254", true},
		{"private 10/8", "10.0.0.1", true},
		{"private 172.16/12", "172.16.0.1", true},
		{"private 172.31.255.254", "172.31.255.254", true},
		{"private 192.168/16", "192.168.1.1", true},
		{"link-local", "169.254.1.2", true},
		{"ipv6 link-local", "fe80::1", true},
		{"ipv6 unique local", "fd00::1", true},
		{"unspecified", "0.0.0.0", true},
		{"multicast", "224.0.0.1", true},
		// Allowed: loopback (dev/test httptest) + public addresses.
		{"loopback ipv4", "127.0.0.1", false},
		{"loopback ipv6", "::1", false},
		{"public ipv4", "8.8.8.8", false},
		{"public hostname", "example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBlockedHost(tc.host); got != tc.want {
				t.Errorf("isBlockedHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestIsBlockedHost_ResolutionFailure(t *testing.T) {
	// A hostname that cannot resolve must be treated as blocked (conservative).
	if !isBlockedHost("does-not-exist.invalid") {
		t.Errorf("isBlockedHost(does-not-exist.invalid) = false, want true (blocked)")
	}
}

func TestIsBlockedHost_MappedIPv6(t *testing.T) {
	// ::ffff:127.0.0.1 is loopback → allowed; ::ffff:10.0.0.1 is private → blocked.
	if isBlockedHost("::ffff:127.0.0.1") {
		t.Errorf("::ffff:127.0.0.1 should be allowed (loopback)")
	}
	if !isBlockedHost("::ffff:10.0.0.1") {
		t.Errorf("::ffff:10.0.0.1 should be blocked (private)")
	}
}

// postFromURL is a small helper: POST {"url": u} to WallpaperFromURL and
// return the recorder.
func postFromURL(t *testing.T, a *app.App, rawURL string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"url": rawURL})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("app", a)
	WallpaperFromURL(c)
	return w
}

func TestHandleWallpaperFromURL_SSRFBlocked(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// Real private / metadata IPs must be rejected before any connection is
	// attempted — no server needed, the block happens at parse time.
	for _, rawURL := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://[fd00::1]/",
	} {
		w := postFromURL(t, a, rawURL)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", rawURL, w.Code)
		}
		if !strings.Contains(w.Body.String(), "not allowed") {
			t.Errorf("%s: body %q missing 'not allowed'", rawURL, w.Body.String())
		}
	}

	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) != 0 {
		t.Errorf("expected no files saved for blocked hosts, got %d", len(entries))
	}
}

func TestHandleWallpaperFromURL_SSRFBlockedHostname(t *testing.T) {
	// A hostname resolving to a blocked IP must be rejected.  Stub the
	// resolver so the test is hermetic (no external DNS).
	a, wallpaperDir := newWallpaperTestApp(t)

	orig := netLookupIP
	netLookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.1")}, nil
	}
	defer func() { netLookupIP = orig }()

	w := postFromURL(t, a, "http://internal.example/x.png")
	if w.Code != http.StatusBadRequest {
		t.Errorf("hostname resolving to private IP: code = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("hostname resolving to private IP: body %q missing 'not allowed'", w.Body.String())
	}
	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) != 0 {
		t.Errorf("expected no files saved for blocked hostname, got %d", len(entries))
	}
}

func TestHandleWallpaperFromURL_SSRFRedirectBlocked(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	// Public-ish start (loopback httptest is allowed) that redirects into a
	// blocked private range → must be refused, not followed.
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://10.0.0.1/x.png", http.StatusFound)
	}))
	defer redirectSrv.Close()

	w := postFromURL(t, a, redirectSrv.URL+"/start.png")
	if w.Code != http.StatusBadRequest {
		t.Errorf("redirect to private host: code = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not allowed") {
		t.Errorf("redirect to private host: body %q missing 'not allowed'", w.Body.String())
	}

	entries, _ := os.ReadDir(wallpaperDir)
	if len(entries) != 0 {
		t.Errorf("expected no files saved after blocked redirect, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Content-Type case-insensitivity (RFC 7231 §3.1.1.1)
// ---------------------------------------------------------------------------

func TestContentTypeToExt_CaseInsensitive(t *testing.T) {
	cases := []struct{ in, want string }{
		{"image/png", ".png"},
		{"IMAGE/PNG", ".png"},
		{"Image/Png", ".png"},
		{"image/jpeg", ".jpg"},
		{"IMAGE/JPEG", ".jpg"},
		{"image/webp", ".webp"},
		{"IMAGE/WEBP", ".webp"},
		{"image/svg+xml", ".svg"},
		{"IMAGE/SVG+XML", ".svg"},
		{"text/html", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := contentTypeToExt(tc.in); got != tc.want {
			t.Errorf("contentTypeToExt(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHandleWallpaperFromURL_ContentTypeCaseInsensitive(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	pngData := createTestPNG(t, 100, 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Uppercase Content-Type per RFC 7231 (media types are case-insensitive).
		w.Header().Set("Content-Type", "IMAGE/PNG")
		w.Write(pngData)
	}))
	defer srv.Close()

	w := postFromURL(t, a, srv.URL+"/u.png")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s (want 200 for IMAGE/PNG)", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	filename := got["filename"].(string)
	if !strings.HasSuffix(filename, ".png") {
		t.Errorf("filename %q should end with .png for IMAGE/PNG", filename)
	}
	if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); err != nil {
		t.Errorf("file not saved: %v", err)
	}
}