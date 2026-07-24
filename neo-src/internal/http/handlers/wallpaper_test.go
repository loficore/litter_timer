package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestHandleWallpaperUpload(t *testing.T) {
	a, wallpaperDir := newWallpaperTestApp(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, err = part.Write([]byte("fake png content"))
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

	handleWallpaperUpload(c)

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

	handleWallpaperUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
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

	handleWallpaperList(c)

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

func TestHandleWallpaperList_Empty(t *testing.T) {
	a, _ := newWallpaperTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/wallpapers", nil)
	c.Set("app", a)

	handleWallpaperList(c)

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

	handleWallpaperServe(c)

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

	handleWallpaperServe(c)

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

		handleWallpaperServe(c)

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

	handleWallpaperDelete(c)

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

	if _, err := os.Stat(path); !os.IsNotExist(err) {
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

	handleWallpaperDelete(c)

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

		handleWallpaperDelete(c)

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

func TestSaveMultipart(t *testing.T) {
	tmpDir := t.TempDir()
	dstPath := filepath.Join(tmpDir, "test.txt")

	src := &testMultipartFile{Reader: strings.NewReader("test content")}

	err := saveMultipart(src, dstPath)
	if err != nil {
		t.Fatalf("saveMultipart: %v", err)
	}

	content, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("file content = %q, want 'test content'", string(content))
	}
}

type testMultipartFile struct {
	io.Reader
}

func (f *testMultipartFile) Close() error                  { return nil }
func (f *testMultipartFile) ReadAt(p []byte, off int64) (int, error) { return f.Reader.Read(p) }
func (f *testMultipartFile) Seek(offset int64, whence int) (int64, error) { return 0, nil }

// ---------------------------------------------------------------------------
// POST /api/wallpapers/from-url  tests
// ---------------------------------------------------------------------------

func TestHandleWallpaperFromURL(t *testing.T) {
	t.Run("FromURL_Happy", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("fake png bytes"))
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/test.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		handleWallpaperFromURL(c)

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
		filePath := filepath.Join(wallpaperDir, filename)
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("file not saved: %v", err)
		}
	})

	t.Run("FromURL_ContentTypeWithParams", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png; charset=binary")
			w.Write([]byte("fake png with params"))
		}))
		defer srv.Close()

		body, _ := json.Marshal(map[string]string{"url": srv.URL + "/img.png"})
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallpapers/from-url", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("app", a)

		handleWallpaperFromURL(c)

		if w.Code != http.StatusOK {
			t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
		}
		var got map[string]any
		json.Unmarshal(w.Body.Bytes(), &got)
		filename := got["filename"].(string)
		if !strings.HasSuffix(filename, ".png") {
			t.Errorf("filename %q should end with .png", filename)
		}
		if _, err := os.Stat(filepath.Join(wallpaperDir, filename)); err != nil {
			t.Errorf("file not saved: %v", err)
		}
	})

	t.Run("FromURL_Redirect", func(t *testing.T) {
		a, wallpaperDir := newWallpaperTestApp(t)

		finalSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("redirected jpeg"))
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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

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

		handleWallpaperFromURL(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("code = %d, want 400", w.Code)
		}
	})
}
