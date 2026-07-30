package backup

// Tests for S3Adapter using a mock S3APIClient.
//
// The mock stores objects in memory and records all API calls, allowing
// full behavioral testing without a real S3 endpoint or testcontainers.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// -----------------------------------------------------------------------------
// mockS3Client
// -----------------------------------------------------------------------------

// mockS3Client implements S3APIClient in-memory.  Every method records
// its input in a calls slice and returns values driven by struct fields.
// When a mockObj field is nil a zero-value output is returned so callers
// only need to set the fields they care about.
type mockS3Client struct {
	store map[string][]byte // objects stored by key

	// Recorded calls (appended on every invocation).
	putCalls        []*s3.PutObjectInput
	getCalls        []*s3.GetObjectInput
	deleteCalls     []*s3.DeleteObjectInput
	headBucketCalls []*s3.HeadBucketInput
	listCalls       []*s3.ListObjectsV2Input

	// Per-method error overrides (nil = success).
	headBucketErr    error
	putObjectErr     error
	getObjectErr     error
	deleteObjectErr  error
	listObjectsV2Err error

	// Override ListObjectsV2 output (nil = empty output).
	listObjectsV2Out *s3.ListObjectsV2Output

	// Override GetObject output (nil = store lookup / empty body).
	getObjectOut *s3.GetObjectOutput
}

func (m *mockS3Client) HeadBucket(_ context.Context, params *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	m.headBucketCalls = append(m.headBucketCalls, params)
	if m.headBucketErr != nil {
		return nil, m.headBucketErr
	}
	return &s3.HeadBucketOutput{}, nil
}

func (m *mockS3Client) PutObject(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putCalls = append(m.putCalls, params)
	if m.putObjectErr != nil {
		return nil, m.putObjectErr
	}
	if m.store != nil && params.Key != nil && params.Body != nil {
		b, err := io.ReadAll(params.Body)
		if err == nil {
			m.store[*params.Key] = b
		}
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getCalls = append(m.getCalls, params)
	if m.getObjectErr != nil {
		return nil, m.getObjectErr
	}
	if m.getObjectOut != nil {
		return m.getObjectOut, nil
	}
	if m.store != nil && params.Key != nil {
		b, ok := m.store[*params.Key]
		if ok {
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}, nil
		}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, params *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteCalls = append(m.deleteCalls, params)
	if m.deleteObjectErr != nil {
		return nil, m.deleteObjectErr
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) ListObjectsV2(_ context.Context, params *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.listCalls = append(m.listCalls, params)
	if m.listObjectsV2Err != nil {
		return nil, m.listObjectsV2Err
	}
	if m.listObjectsV2Out != nil {
		return m.listObjectsV2Out, nil
	}
	return &s3.ListObjectsV2Output{}, nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newS3AdapterWithMock returns an S3Adapter wired to a mock client.
// The adapter is constructed directly (not via NewS3Adapter) so tests
// don't need real AWS credentials.
func newS3AdapterWithMock(t *testing.T, mock *mockS3Client) *S3Adapter {
	t.Helper()
	return &S3Adapter{
		cfg: S3Config{
			Bucket:     "test-bucket",
			Region:     "us-east-1",
			PathPrefix: "little_timer/",
		},
		client: mock,
	}
}

// writeTempFile creates a temp file with the given content and returns
// its path.  The caller is responsible for cleanup.
func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// readTempFile reads the contents of a file and fails the test on error.
func readTempFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return b
}

// -----------------------------------------------------------------------------
// TestS3BackupRestoreRoundTrip
// -----------------------------------------------------------------------------

// TestS3BackupRestoreRoundTrip verifies that bytes written via Backup are
// returned identically by Restore — the mock stores PutObject bytes and
// returns them from GetObject.
func TestS3BackupRestoreRoundTrip(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	original := []byte("hello S3 backup round-trip")
	srcPath := writeTempFile(t, original)
	destPath := filepath.Join(t.TempDir(), "restored.db")

	if err := adapter.Backup(srcPath, "presets_backup_1234567890.db"); err != nil {
		t.Fatalf("Backup: unexpected error: %v", err)
	}

	if err := adapter.Restore("presets_backup_1234567890.db", destPath); err != nil {
		t.Fatalf("Restore: unexpected error: %v", err)
	}

	if got := readTempFile(t, destPath); !bytes.Equal(got, original) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, original)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_WriteProbe
// -----------------------------------------------------------------------------

// TestS3TestConnection_WriteProbe verifies that TestConnection invokes
// HeadBucket, PutObject, GetObject, and DeleteObject with the probe filename.
func TestS3TestConnection_WriteProbe(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	if err := adapter.TestConnection(); err != nil {
		t.Fatalf("TestConnection: unexpected error: %v", err)
	}

	// HeadBucket must be called once.
	if n := len(mock.headBucketCalls); n != 1 {
		t.Fatalf("HeadBucket: want 1 call, got %d", n)
	}

	// PutObject must be called with a probe key.
	if n := len(mock.putCalls); n != 1 {
		t.Fatalf("PutObject: want 1 call, got %d", n)
	}
	key := aws.ToString(mock.putCalls[0].Key)
	if !strings.Contains(key, "lt_probe_") {
		t.Fatalf("PutObject key %q does not contain lt_probe_", key)
	}

	// GetObject must be called with the same key.
	if n := len(mock.getCalls); n != 1 {
		t.Fatalf("GetObject: want 1 call, got %d", n)
	}
	if aws.ToString(mock.getCalls[0].Key) != key {
		t.Fatalf("GetObject key mismatch: got %q, want %q",
			aws.ToString(mock.getCalls[0].Key), key)
	}

	// DeleteObject must be called with the same key.
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject: want 1 call, got %d", n)
	}
	if aws.ToString(mock.deleteCalls[0].Key) != key {
		t.Fatalf("DeleteObject key mismatch: got %q, want %q",
			aws.ToString(mock.deleteCalls[0].Key), key)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_NoSuchBucket
// -----------------------------------------------------------------------------

// TestS3TestConnection_NoSuchBucket verifies that a HeadBucket
// NoSuchBucket error is classified as ErrFileNotFound.
func TestS3TestConnection_NoSuchBucket(t *testing.T) {
	mock := &mockS3Client{
		store:         make(map[string][]byte),
		headBucketErr: &types.NoSuchBucket{Message: aws.String("test")},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_AccessDenied
// -----------------------------------------------------------------------------

// TestS3TestConnection_AccessDenied verifies that a PutObject
// AccessDenied error is classified as ErrPermissionDenied.
func TestS3TestConnection_AccessDenied(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		putObjectErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "test"},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_InvalidAccessKeyId
// -----------------------------------------------------------------------------

// TestS3TestConnection_InvalidAccessKeyId verifies that a
// GenericAPIError with code "InvalidAccessKeyId" is classified as
// ErrAuthenticationFail.
func TestS3TestConnection_InvalidAccessKeyId(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &smithy.GenericAPIError{
			Code:    "InvalidAccessKeyId",
			Message: "test",
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Fatalf("expected ErrAuthenticationFail, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_SignatureDoesNotMatch
// -----------------------------------------------------------------------------

// TestS3TestConnection_SignatureDoesNotMatch verifies that a
// GenericAPIError with code "SignatureDoesNotMatch" is classified as
// ErrAuthenticationFail.
func TestS3TestConnection_SignatureDoesNotMatch(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &smithy.GenericAPIError{
			Code:    "SignatureDoesNotMatch",
			Message: "test",
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrAuthenticationFail) {
		t.Fatalf("expected ErrAuthenticationFail, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_NetworkError
// -----------------------------------------------------------------------------

// TestS3TestConnection_NetworkError verifies that a *url.Error is
// classified as ErrNetworkError.
func TestS3TestConnection_NetworkError(t *testing.T) {
	mock := &mockS3Client{
		store: make(map[string][]byte),
		headBucketErr: &url.Error{
			Op:  "Get",
			URL: "https://s3.example.com",
			Err: errors.New("dial tcp: connection refused"),
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if !errors.Is(err, ErrNetworkError) {
		t.Fatalf("expected ErrNetworkError, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3TestConnection_NilBody
// -----------------------------------------------------------------------------

// TestS3TestConnection_NilBody verifies that a GetObject response with a
// nil Body does not panic in io.ReadAll: TestConnection must return
// ErrConnectionFailed and still attempt the DeleteObject probe cleanup.
func TestS3TestConnection_NilBody(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		getObjectOut: &s3.GetObjectOutput{},
	}
	adapter := newS3AdapterWithMock(t, mock)

	err := adapter.TestConnection()
	if err == nil {
		t.Fatal("expected error for nil GetObject body, got nil")
	}
	if !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ErrConnectionFailed, got %v", err)
	}
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject probe cleanup: want 1 call, got %d", n)
	}
	if key := aws.ToString(mock.deleteCalls[0].Key); !strings.Contains(key, "lt_probe_") {
		t.Errorf("DeleteObject key %q does not contain lt_probe_", key)
	}
}

// -----------------------------------------------------------------------------
// TestS3Backup_AccessDenied
// -----------------------------------------------------------------------------

// TestS3Backup_AccessDenied verifies that a PutObject AccessDenied
// error during Backup is classified as ErrPermissionDenied (not
// generic ErrBackupFailed).
func TestS3Backup_AccessDenied(t *testing.T) {
	mock := &mockS3Client{
		store:        make(map[string][]byte),
		putObjectErr: &smithy.GenericAPIError{Code: "AccessDenied", Message: "test"},
	}
	adapter := newS3AdapterWithMock(t, mock)
	srcPath := writeTempFile(t, []byte("test"))

	err := adapter.Backup(srcPath, "presets_backup_1234567890.db")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// TestS3List
// -----------------------------------------------------------------------------

// TestS3List verifies that ListObjectsV2 results are parsed into
// BackupInfo entries with the prefix stripped.
func TestS3List(t *testing.T) {
	now := time.Now()
	mock := &mockS3Client{
		store: make(map[string][]byte),
		listObjectsV2Out: &s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key:          aws.String("little_timer/presets_backup_1000.db"),
					LastModified: aws.Time(now),
					Size:         aws.Int64(2048),
				},
				{
					Key:          aws.String("little_timer/presets_backup_2000.db"),
					LastModified: aws.Time(now.Add(time.Hour)),
					Size:         aws.Int64(4096),
				},
			},
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	results, err := adapter.List()
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Name != "presets_backup_1000.db" {
		t.Fatalf("first name: got %q, want presets_backup_1000.db", results[0].Name)
	}
	if results[0].SizeBytes != 2048 {
		t.Fatalf("first size: got %d, want 2048", results[0].SizeBytes)
	}
	if results[1].Name != "presets_backup_2000.db" {
		t.Fatalf("second name: got %q, want presets_backup_2000.db", results[1].Name)
	}
	if results[1].SizeBytes != 4096 {
		t.Fatalf("second size: got %d, want 4096", results[1].SizeBytes)
	}

	// Verify the ListObjectsV2 call used the correct prefix.
	if n := len(mock.listCalls); n != 1 {
		t.Fatalf("ListObjectsV2: want 1 call, got %d", n)
	}
	if aws.ToString(mock.listCalls[0].Prefix) != "little_timer/" {
		t.Fatalf("prefix: got %q, want little_timer/", aws.ToString(mock.listCalls[0].Prefix))
	}
}

// -----------------------------------------------------------------------------
// TestS3List_TimestampFromName
// -----------------------------------------------------------------------------

// TestS3List_TimestampFromName verifies that the backup timestamp is
// parsed from the key name (matching Local/WebDAV) and that LastModified
// is ignored when the name parses.
func TestS3List_TimestampFromName(t *testing.T) {
	lastModified := time.Unix(1800000000, 0)
	mock := &mockS3Client{
		store: make(map[string][]byte),
		listObjectsV2Out: &s3.ListObjectsV2Output{
			Contents: []types.Object{
				{
					Key:          aws.String("little_timer/presets_backup_1700000000.db"),
					LastModified: aws.Time(lastModified),
					Size:         aws.Int64(100),
				},
				{
					Key:          aws.String("little_timer/presets_backup_1700000100.db"),
					LastModified: aws.Time(lastModified),
					Size:         aws.Int64(200),
				},
			},
		},
	}
	adapter := newS3AdapterWithMock(t, mock)

	results, err := adapter.List()
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}

	// Timestamp must come from the filename, not LastModified.
	if got := results[0].Timestamp; got != 1700000000 {
		t.Errorf("first timestamp: got %d, want 1700000000 (name-parsed, LastModified=%d)", got, lastModified.Unix())
	}
	if got := results[1].Timestamp; got != 1700000100 {
		t.Errorf("second timestamp: got %d, want 1700000100 (name-parsed, LastModified=%d)", got, lastModified.Unix())
	}
}

// -----------------------------------------------------------------------------
// TestS3Delete
// -----------------------------------------------------------------------------

// TestS3Delete verifies that Delete calls DeleteObject with the correct
// key (PathPrefix + "/" + backupName) and returns no error.
func TestS3Delete(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	if err := adapter.Delete("presets_backup_1234567890.db"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}
	if n := len(mock.deleteCalls); n != 1 {
		t.Fatalf("DeleteObject: want 1 call, got %d", n)
	}
	wantKey := "little_timer/presets_backup_1234567890.db"
	if got := aws.ToString(mock.deleteCalls[0].Key); got != wantKey {
		t.Fatalf("DeleteObject key: got %q, want %q", got, wantKey)
	}
}

// -----------------------------------------------------------------------------
// TestS3WriteManifest
// -----------------------------------------------------------------------------

// TestS3WriteManifest verifies that WriteManifest calls PutObject with
// a manifest.json key and the Content-Type header set to application/json.
func TestS3WriteManifest(t *testing.T) {
	mock := &mockS3Client{store: make(map[string][]byte)}
	adapter := newS3AdapterWithMock(t, mock)

	manifestData := `{"version":1,"backups":[]}`
	if err := adapter.WriteManifest(manifestData); err != nil {
		t.Fatalf("WriteManifest: unexpected error: %v", err)
	}
	if n := len(mock.putCalls); n != 1 {
		t.Fatalf("PutObject: want 1 call, got %d", n)
	}
	call := mock.putCalls[0]
	if aws.ToString(call.Key) != "little_timer/manifest.json" {
		t.Fatalf("key: got %q, want little_timer/manifest.json", aws.ToString(call.Key))
	}
	if aws.ToString(call.ContentType) != "application/json" {
		t.Fatalf("ContentType: got %q, want application/json", aws.ToString(call.ContentType))
	}

	// Verify the body was stored in the mock.
	b, ok := mock.store["little_timer/manifest.json"]
	if !ok {
		t.Fatal("manifest.json not stored in mock")
	}
	if string(b) != manifestData {
		t.Fatalf("body: got %q, want %q", string(b), manifestData)
	}
}
