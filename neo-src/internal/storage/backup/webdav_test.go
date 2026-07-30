package backup

// Tests for WebDAVAdapter using httptest.NewServer for full round-trip
// coverage of all adapter methods.
//
// Test cases:
//   1. TestWebDAVBackupRestoreRoundTrip — PUT backup -> GET restore -> byte-identical
//   2. TestWebDAVTestConnection_WriteProbe — TestConnection does PUT/GET/DELETE probe
//   3. TestWebDAVTestConnection_AuthFailure — server returns 401; assert ErrAuthenticationFail
//   4. TestWebDAVTestConnection_PermissionDenied — server returns 403; assert ErrPermissionDenied
//   5. TestWebDAVList — PROPFIND multistatus XML with 2 backups; assert parsed
//   6. TestWebDAVDelete — DELETE succeeds; assert server received DELETE
//   7. TestWebDAVWriteManifest — WriteManifest PUTs manifest.json; assert file exists

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newWebDAVTestServer creates a WebDAVAdapter wired to an httptest server
// that stores PUT bodies in a map.  The caller can use the store map to
// inspect what the adapter wrote or to seed data for GET requests.
//
// The server handles PUT, GET, DELETE, and PROPFIND (empty multistatus).
// Tests that need custom behavior (auth failures, specific PROPFIND
// responses) should create their own server inline.
func newWebDAVTestServer(t *testing.T) (*WebDAVAdapter, *httptest.Server, map[string][]byte) {
	t.Helper()
	store := make(map[string][]byte)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			store[key] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
		case http.MethodDelete:
			delete(store, key)
			w.WriteHeader(http.StatusNoContent)
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:"/>`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()
	return adapter, srv, store
}

// -----------------------------------------------------------------------------
// Test 1 — Backup / Restore round-trip
// -----------------------------------------------------------------------------

func TestWebDAVBackupRestoreRoundTrip(t *testing.T) {
	adapter, _, store := newWebDAVTestServer(t)

	payload := []byte("SQLite-format-3\x00webdav-round-trip-payload")
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// The server must have received the PUT body.
	gotStored := store["/backups/"+name]
	if string(gotStored) != string(payload) {
		t.Errorf("stored payload mismatch:\n got: %q\nwant: %q", gotStored, payload)
	}

	// Restore into a fresh destination and confirm byte-identical copy.
	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != string(payload) {
		t.Errorf("restored content mismatch:\n got: %q\nwant: %q", got, payload)
	}
}

// -----------------------------------------------------------------------------
// Test 2 — TestConnection write-probe (PUT → GET → DELETE)
// -----------------------------------------------------------------------------

func TestWebDAVTestConnection_WriteProbe(t *testing.T) {
	var ops []string
	store := make(map[string][]byte)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ops = append(ops, r.Method)
		key := r.URL.Path
		switch r.Method {
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			store[key] = body
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			data, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(data)
		case http.MethodDelete:
			delete(store, key)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	if err := adapter.TestConnection(); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}

	if len(ops) < 3 {
		t.Fatalf("expected at least 3 operations (PUT, GET, DELETE), got %d: %v", len(ops), ops)
	}
	if ops[0] != http.MethodPut {
		t.Errorf("first operation: got %s, want PUT", ops[0])
	}
	if ops[1] != http.MethodGet {
		t.Errorf("second operation: got %s, want GET", ops[1])
	}
	if ops[2] != http.MethodDelete {
		t.Errorf("third operation: got %s, want DELETE", ops[2])
	}
}

// -----------------------------------------------------------------------------
// Test 3 — TestConnection auth failure (401)
// -----------------------------------------------------------------------------

func TestWebDAVTestConnection_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 401 on PUT (and any subsequent DELETE from best-effort cleanup).
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Errorf("error: got %v, want errors.Is ErrAuthenticationFail", err)
	}
}

// -----------------------------------------------------------------------------
// Test 4 — TestConnection permission denied (403)
// -----------------------------------------------------------------------------

func TestWebDAVTestConnection_PermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 403 on PUT maps to ErrPermissionDenied, matching classifyS3Error.
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("error: got %v, want errors.Is ErrPermissionDenied", err)
	}
}

// -----------------------------------------------------------------------------
// Test 4b — TestConnection GET probe returns server error (500)
// -----------------------------------------------------------------------------

func TestWebDAVTestConnection_GETProbeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			// PUT succeeds so the probe reaches the GET step.
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			// 500 on the GET probe must map to ErrConnectionFailed,
			// not a misleading "probe content mismatch".
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for GET probe 500, got nil")
	}
	if !errors.Is(err, ErrConnectionFailed) {
		t.Errorf("error: got %v, want errors.Is ErrConnectionFailed", err)
	}
}

// -----------------------------------------------------------------------------
// Test 4c — TestConnection GET probe returns permission denied (403)
// -----------------------------------------------------------------------------

func TestWebDAVTestConnection_GETProbePermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			// PUT succeeds so the probe reaches the GET step.
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for GET probe 403, got nil")
	}
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("error: got %v, want errors.Is ErrPermissionDenied", err)
	}
}

// -----------------------------------------------------------------------------
// Test 5 — List (PROPFIND multistatus)
// -----------------------------------------------------------------------------

func TestWebDAVList(t *testing.T) {
	ts1 := int64(1000)
	ts2 := int64(2000)

	multistatusXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<D:multistatus xmlns:D="DAV:">
  <D:response>
    <D:href>/backups/</D:href>
    <D:propstat>
      <D:prop>
        <D:resourcetype><D:collection/></D:resourcetype>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>42</D:getcontentlength>
        <D:getlastmodified>Mon, 01 Jan 2024 00:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/%s</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>99</D:getcontentlength>
        <D:getlastmodified>Tue, 02 Jan 2024 00:00:00 GMT</D:getlastmodified>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
  <D:response>
    <D:href>/backups/not_a_backup.txt</D:href>
    <D:propstat>
      <D:prop>
        <D:getcontentlength>7</D:getcontentlength>
      </D:prop>
      <D:status>HTTP/1.1 200 OK</D:status>
    </D:propstat>
  </D:response>
</D:multistatus>`, backupName(ts1), backupName(ts2))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			w.Write([]byte(multistatusXML))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("List len: got %d, want 2 (entries: %+v)", len(got), got)
	}

	// The directory entry and the wrong-prefix entry must be filtered out.
	byName := map[string]BackupInfo{}
	for _, info := range got {
		byName[info.Name] = info
	}

	info1, ok := byName[backupName(ts1)]
	if !ok {
		t.Errorf("missing backup %s in results", backupName(ts1))
	} else {
		if info1.Timestamp != ts1 {
			t.Errorf("timestamp for %s: got %d, want %d", backupName(ts1), info1.Timestamp, ts1)
		}
		if info1.SizeBytes != 42 {
			t.Errorf("size for %s: got %d, want 42", backupName(ts1), info1.SizeBytes)
		}
	}

	info2, ok := byName[backupName(ts2)]
	if !ok {
		t.Errorf("missing backup %s in results", backupName(ts2))
	} else {
		if info2.Timestamp != ts2 {
			t.Errorf("timestamp for %s: got %d, want %d", backupName(ts2), info2.Timestamp, ts2)
		}
		if info2.SizeBytes != 99 {
			t.Errorf("size for %s: got %d, want 99", backupName(ts2), info2.SizeBytes)
		}
	}
}

// -----------------------------------------------------------------------------
// Test 6 — Delete
// -----------------------------------------------------------------------------

func TestWebDAVDelete(t *testing.T) {
	var deletedPath string
	store := make(map[string][]byte)

	name := backupName(time.Now().Unix())
	store["/backups/"+name] = []byte("doomed")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			deletedPath = r.URL.Path
			delete(store, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	if err := adapter.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	expectedPath := "/backups/" + name
	if deletedPath != expectedPath {
		t.Errorf("DELETE path: got %q, want %q", deletedPath, expectedPath)
	}
	if _, exists := store[expectedPath]; exists {
		t.Errorf("file %q still exists in store after Delete", expectedPath)
	}
}

// -----------------------------------------------------------------------------
// Test 7 — WriteManifest
// -----------------------------------------------------------------------------

func TestWebDAVWriteManifest(t *testing.T) {
	adapter, _, store := newWebDAVTestServer(t)

	manifestData := `{"version":1,"backups":[]}`
	if err := adapter.WriteManifest(manifestData); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	manifestKey := "/backups/manifest.json"
	got, ok := store[manifestKey]
	if !ok {
		t.Fatalf("manifest not found at key %q; store keys: %v", manifestKey, store)
	}
	if string(got) != manifestData {
		t.Errorf("manifest data mismatch:\n got: %q\nwant: %q", got, manifestData)
	}
}

// -----------------------------------------------------------------------------
// Idempotent delete of nonexistent backup (bonus edge case)
// -----------------------------------------------------------------------------

func TestWebDAVDeleteNonexistentIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	adapter := NewWebDAVAdapter(WebDAVConfig{
		URL:      srv.URL,
		BasePath: "/backups",
	})
	adapter.client = srv.Client()

	// Delete of a missing file should succeed (idempotent).
	if err := adapter.Delete(backupName(9999)); err != nil {
		t.Errorf("Delete on missing file: got %v, want nil", err)
	}
}

// -----------------------------------------------------------------------------
// Target discriminator
// -----------------------------------------------------------------------------

func TestWebDAVTarget(t *testing.T) {
	adapter, _, _ := newWebDAVTestServer(t)
	if got := adapter.Target(); got != TargetWebDAV {
		t.Errorf("Target: got %q, want %q", got, TargetWebDAV)
	}
}

// -----------------------------------------------------------------------------
// Restore of nonexistent backup returns ErrFileNotFound
// -----------------------------------------------------------------------------

func TestWebDAVRestoreNotFound(t *testing.T) {
	adapter, _, _ := newWebDAVTestServer(t)

	dst := filepath.Join(t.TempDir(), "dst.db")
	err := adapter.Restore("presets_backup_does_not_exist.db", dst)
	if err == nil {
		t.Fatalf("Restore of missing backup should fail")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("Restore error: got %v, want errors.Is ErrFileNotFound", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("destination file should not exist after failed Restore (stat err: %v)", statErr)
	}
}
