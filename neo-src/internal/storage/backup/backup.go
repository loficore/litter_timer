// Package backup — BackupManager coordinating adapters.
//
// Port of `src/storage/storage_backup.zig` (little_timer).  The Zig
// source owns its own zqlite connection and uses it to flush the WAL
// before copying the file.  In Go we delegate the "close / reopen"
// dance to the *storage.SqliteManager so the SQLite file is in a
// quiescent state when the adapter reads it — SQLite is single-writer
// and concurrent read+copy while writes are in flight is unsafe.
//
// The BackupManager dispatches every operation against a single
// BackupAdapter (constructed from a BackupConfig), which keeps the
// surface area tight.  Switching targets means constructing a new
// manager — there is no SetTarget mutator in the Zig source either.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// MaxBackups is the default retention cap.  Matches the Zig
// `max_backups: u32 = 10` field.
const MaxBackups = 10

// BackupManager is the Go port of `pub const BackupManager`.
type BackupManager struct {
	sqlite     *storage.SqliteManager
	dbPath     string
	backupDir  string
	maxBackups int
	adapter    BackupAdapter
}

// NewLocal returns a BackupManager wired to a LocalAdapter rooted at
// backupDir.  Mirrors `BackupManager.init`.
func NewLocal(sqliteMgr *storage.SqliteManager, dbPath, backupDir string) (*BackupManager, error) {
	if backupDir == "" {
		return nil, errors.New("backup: backupDir is required for local target")
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("backup: mkdir %s: %w", backupDir, err)
	}
	return &BackupManager{
		sqlite:     sqliteMgr,
		dbPath:     dbPath,
		backupDir:  backupDir,
		maxBackups: MaxBackups,
		adapter:    NewLocalAdapter(backupDir),
	}, nil
}

// NewFromConfig picks an adapter based on cfg.TargetType and wires it
// into a fresh BackupManager.  Mirrors `BackupManager.initWithConfig`.
func NewFromConfig(ctx context.Context, sqliteMgr *storage.SqliteManager, dbPath, backupDir string, cfg domain.BackupConfig) (*BackupManager, error) {
	mgr, err := NewLocal(sqliteMgr, dbPath, backupDir)
	if err != nil {
		return nil, err
	}
	adapter, err := buildAdapter(ctx, cfg, backupDir)
	if err != nil {
		return nil, err
	}
	mgr.adapter = adapter
	return mgr, nil
}

// buildAdapter picks the right adapter for the configured target type.
// webdav / s3 always need their full config; local falls back to
// backupDir when no path was supplied.
func buildAdapter(ctx context.Context, cfg domain.BackupConfig, backupDir string) (BackupAdapter, error) {
	switch cfg.TargetType {
	case domain.BackupTargetWebDAV:
		return NewWebDAVAdapter(WebDAVConfig{
			URL:      cfg.WebDAVURL,
			Username: cfg.WebDAVUsername,
			Password: cfg.WebDAVPassword,
			BasePath: cfg.WebDAVPathPrefix,
		}), nil
	case domain.BackupTargetS3:
		return NewS3Adapter(ctx, S3Config{
			Endpoint:   cfg.S3Endpoint,
			Bucket:     cfg.S3Bucket,
			Region:     cfg.S3Region,
			AccessKey:  cfg.S3AccessKey,
			SecretKey:  cfg.S3SecretKey,
			PathPrefix: cfg.S3PathPrefix,
		})
	default:
		path := cfg.LocalPath
		if path == "" {
			path = backupDir
		}
		return NewLocalAdapter(path), nil
	}
}

// Adapter returns the underlying adapter (handy for tests).
func (m *BackupManager) Adapter() BackupAdapter { return m.adapter }

// MaxBackups returns the retention cap.  Mirrors `pub max_backups`.
func (m *BackupManager) MaxBackups() int { return m.maxBackups }

// SetMaxBackups adjusts the retention cap.
func (m *BackupManager) SetMaxBackups(n int) {
	if n > 0 {
		m.maxBackups = n
	}
}

// -----------------------------------------------------------------------------
// Backup / restore.
// -----------------------------------------------------------------------------

// CreateBackup generates a new backup file and uploads it via the
// configured adapter.  Mirrors `pub fn createBackup`.
//
// Uses VACUUM INTO for a hot snapshot (requires SQLite >= 3.27.0)
// instead of the old wal_checkpoint + direct copy.  SHA-256 is computed
// from the snapshot before upload, then verified by downloading the
// uploaded copy and comparing digests.
//
// Manifest writes are best-effort: the uploaded .db file is the source
// of truth (already SHA-256-verified), so a manifest build/write
// failure is logged as a warning and does not fail the backup.
func (m *BackupManager) CreateBackup() (string, error) {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return "", fmt.Errorf("%w: sqlite not open", ErrBackupFailed)
	}
	ts := time.Now().Unix()
	name := fmt.Sprintf("%s%d%s", filenamePrefix, ts, filenameSuffix)

	// Create temp file for VACUUM INTO hot backup.
	tmp, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_backup_*.db")
	if err != nil {
		return "", fmt.Errorf("%w: tempfile: %v", ErrBackupFailed, err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		log.Warn("CreateBackup: temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpPath)

	// Hot backup: VACUUM INTO writes a consistent snapshot to the temp
	// file.  Requires a brief exclusive lock; acceptable for infrequent
	// backups.  SQLite does not support bind parameters for VACUUM INTO,
	// so single quotes in the path must be escaped by doubling them.
	escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
	if _, err := m.sqlite.DB().Exec(fmt.Sprintf("VACUUM INTO '%s'", escapedPath)); err != nil {
		log.Error("CreateBackup: vacuum failed", "error", err.Error())
		return "", fmt.Errorf("%w: vacuum into: %v", ErrBackupFailed, err)
	}

	// Compute SHA-256 from the temp file before upload.
	hash, err := sha256File(tmpPath)
	if err != nil {
		log.Error("CreateBackup: sha256 failed", "error", err.Error())
		return "", fmt.Errorf("%w: sha256: %v", ErrBackupFailed, err)
	}

	// Get file size for the manifest.
	fi, err := os.Stat(tmpPath)
	if err != nil {
		log.Error("CreateBackup: stat failed", "error", err.Error())
		return "", fmt.Errorf("%w: stat: %v", ErrBackupFailed, err)
	}
	sizeBytes := uint64(fi.Size())

	// Upload the temp file.
	if err := m.adapter.Backup(tmpPath, name); err != nil {
		log.Error("CreateBackup: upload failed", "error", err.Error())
		return "", err
	}

	// GET-back verify: download the uploaded backup and compare SHA-256.
	tmpVerify, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_verify_*.db")
	if err != nil {
		log.Error("CreateBackup: verify tempfile failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify temp: %v", ErrBackupFailed, err)
	}
	tmpVerifyPath := tmpVerify.Name()
	if err := tmpVerify.Close(); err != nil {
		log.Warn("CreateBackup: verify temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpVerifyPath)

	if err := m.adapter.Restore(name, tmpVerifyPath); err != nil {
		log.Error("CreateBackup: verify restore failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify restore: %v", ErrBackupFailed, err)
	}

	verifyHash, err := sha256File(tmpVerifyPath)
	if err != nil {
		log.Error("CreateBackup: verify sha256 failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify sha256: %v", ErrBackupFailed, err)
	}

	if hash != verifyHash {
		log.Error("CreateBackup: sha256 mismatch",
			"expected", hash, "got", verifyHash)
		// Best-effort delete the corrupted backup.
		if delErr := m.adapter.Delete(name); delErr != nil {
			log.Error("CreateBackup: delete after mismatch failed", "error", delErr.Error())
		}
		return "", fmt.Errorf("%w: sha256 mismatch", ErrBackupFailed)
	}

	// Write manifest for ALL targets.  Best-effort: the uploaded .db is
	// already SHA-256-verified, so a manifest failure only degrades the
	// convenience index — log a warning, do not fail the backup.
	manifest, err := m.buildManifest(name, ts, hash, sizeBytes)
	if err != nil {
		log.Warn("CreateBackup: build manifest failed", "error", err.Error())
	} else if err := m.adapter.WriteManifest(manifest); err != nil {
		log.Warn("CreateBackup: write manifest failed", "error", err.Error())
	}

	if err := m.cleanupOldBackups(); err != nil {
		// Retention is best-effort; log but don't fail the backup.
		log.Error("CreateBackup: cleanup failed", "error", err.Error())
	}
	log.Info("CreateBackup: success", "name", name, "sha256", hash)
	return name, nil
}

// RestoreFromBackup fetches a backup by name and overwrites the live DB.
// Mirrors `pub fn restoreFromBackup`.  The DB connection is closed
// during the swap and reopened afterward so the running app picks up
// the restored schema.
func (m *BackupManager) RestoreFromBackup(name string) error {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return fmt.Errorf("%w: sqlite not open", ErrRestoreFailed)
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_restore_*.db")
	if err != nil {
		return fmt.Errorf("%w: tempfile: %v", ErrRestoreFailed, err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		log.Warn("RestoreFromBackup: temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpPath)

	if err := m.adapter.Restore(name, tmpPath); err != nil {
		return err
	}
	if err := m.swapDatabase(tmpPath); err != nil {
		return err
	}
	return nil
}

// buildManifest creates the manifest JSON string for WriteManifest.
// sha256 and sizeBytes are populated from the just-created backup file
// so every entry carries integrity metadata.
func (m *BackupManager) buildManifest(backupName string, timestamp int64, sha256hash string, sizeBytes uint64) (string, error) {
	backups, err := m.adapter.List()
	if err != nil {
		return "", fmt.Errorf("list backups: %w", err)
	}

	type manifestBackup struct {
		Name      string `json:"name"`
		Timestamp int64  `json:"timestamp"`
		SizeBytes uint64 `json:"size_bytes"`
		SHA256    string `json:"sha256"`
	}

	manifestBackups := make([]manifestBackup, 0, len(backups)+1)
	for _, b := range backups {
		if b.Name == backupName {
			continue // skip the just-created backup; it's appended below with SHA256
		}
		manifestBackups = append(manifestBackups, manifestBackup{
			Name:      b.Name,
			Timestamp: b.Timestamp,
			SizeBytes: b.SizeBytes,
		})
	}
	manifestBackups = append(manifestBackups, manifestBackup{
		Name:      backupName,
		Timestamp: timestamp,
		SizeBytes: sizeBytes,
		SHA256:    sha256hash,
	})

	manifest := struct {
		Version   int              `json:"version"`
		Backups   []manifestBackup `json:"backups"`
		DBVersion string           `json:"db_version"`
	}{
		Version:   1,
		Backups:   manifestBackups,
		DBVersion: "1.0",
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}

	return string(data), nil
}

// swapDatabase closes the SQLite connection, replaces the file, then
// reopens.  Mirrors the close/reopen dance in storage_backup.zig.
func (m *BackupManager) swapDatabase(src string) error {
	if err := m.sqlite.Close(); err != nil {
		return fmt.Errorf("%w: close before swap: %v", ErrRestoreFailed, err)
	}
	if err := os.Rename(src, m.dbPath); err != nil {
		return fmt.Errorf("%w: rename: %v", ErrRestoreFailed, err)
	}
	if err := m.sqlite.Open(); err != nil {
		return fmt.Errorf("%w: reopen after swap: %v", ErrRestoreFailed, err)
	}
	if err := m.sqlite.Migrate(); err != nil {
		return fmt.Errorf("%w: migrate after swap: %v", ErrRestoreFailed, err)
	}
	return nil
}

// DeleteBackup removes a single backup.  Mirrors `pub fn deleteBackup`.
func (m *BackupManager) DeleteBackup(name string) error {
	return m.adapter.Delete(name)
}

// ListBackups returns every backup known to the adapter.  Mirrors
// `pub fn listBackups`.
func (m *BackupManager) ListBackups() ([]BackupInfo, error) {
	return m.adapter.List()
}

// TestConnection validates the configured adapter is reachable.
func (m *BackupManager) TestConnection() error {
	return m.adapter.TestConnection()
}

// -----------------------------------------------------------------------------
// BackupInfo helpers — `getBackupInfo` / `freeBackupInfo` analogues.
// -----------------------------------------------------------------------------

// BackupSummary is the analogue of `getBackupInfo`'s anonymous struct.
type BackupSummary struct {
	TotalBackups   int    `json:"total_backups"`
	TotalSizeBytes uint64 `json:"total_size_bytes"`
	OldestBackup   string `json:"oldest_backup,omitempty"`
	NewestBackup   string `json:"newest_backup,omitempty"`
}

// Summary aggregates counts and size of the adapter's stored backups.
func (m *BackupManager) Summary() (BackupSummary, error) {
	items, err := m.adapter.List()
	if err != nil {
		return BackupSummary{}, err
	}
	if len(items) == 0 {
		return BackupSummary{}, nil
	}
	sorted := append([]BackupInfo(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })

	var totalSize uint64
	for _, it := range sorted {
		totalSize += it.SizeBytes
	}
	return BackupSummary{
		TotalBackups:   len(sorted),
		TotalSizeBytes: totalSize,
		OldestBackup:   sorted[0].Name,
		NewestBackup:   sorted[len(sorted)-1].Name,
	}, nil
}

// sha256File computes the SHA-256 hex digest of a file via streaming.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cleanupOldBackups trims the adapter down to m.maxBackups entries by
// deleting the oldest.  Local + WebDAV + S3 all support Delete so this
// is uniform across adapters.
func (m *BackupManager) cleanupOldBackups() error {
	items, err := m.adapter.List()
	if err != nil {
		return err
	}
	if len(items) <= m.maxBackups {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp < items[j].Timestamp })
	toDelete := items[:len(items)-m.maxBackups]
	for _, it := range toDelete {
		if err := m.adapter.Delete(it.Name); err != nil {
			fmt.Fprintf(os.Stderr, "backup: delete %s: %v\n", it.Name, err)
		}
	}
	return nil
}
