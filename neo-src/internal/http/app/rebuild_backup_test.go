package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// TestRebuildBackupSwitchesTargetType verifies that RebuildBackup
// reconstructs the BackupManager from the persisted BackupConfig,
// swapping the active adapter as target_type changes.  The webdav/s3
// configs are pure constructions — NewWebDAVAdapter / NewS3Adapter do
// no network I/O — so no httptest server is needed.
func TestRebuildBackupSwitchesTargetType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
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

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)

	assertTarget := func(want backup.BackupTarget) *backup.BackupManager {
		t.Helper()
		got := a.BackupManager()
		if got == nil {
			t.Fatalf("BackupManager() = nil, want adapter target %q", want)
		}
		if got.Adapter().Target() != want {
			t.Fatalf("adapter target = %q, want %q", got.Adapter().Target(), want)
		}
		return got
	}

	update := func(json string) {
		t.Helper()
		if err := sm.UpdateBackupConfigFromJSON(json); err != nil {
			t.Fatalf("update backup config: %v", err)
		}
		if err := a.RebuildBackup(context.Background()); err != nil {
			t.Fatalf("RebuildBackup: %v", err)
		}
	}

	// Local — the default target; adapter is rooted in the derived
	// sibling-of-DB backup dir (LocalPath is empty).
	update(`{"target_type": "local"}`)
	localMgr := assertTarget(backup.TargetLocal)

	// WebDAV — fake URL/creds are enough; construction does no I/O.
	update(`{"target_type": "webdav", "webdav_url": "https://example.com/dav", "webdav_username": "u", "webdav_password": "p"}`)
	webdavMgr := assertTarget(backup.TargetWebDAV)
	if webdavMgr == localMgr {
		t.Fatal("BackupManager() still references the pre-rebuild manager after webdav switch")
	}

	// S3 — bucket + region are required by NewS3Adapter; static creds
	// only, no network.
	update(`{"target_type": "s3", "s3_endpoint": "https://minio.example:9000", "s3_bucket": "bkt", "s3_region": "us-east-1", "s3_access_key": "ak", "s3_secret_key": "sk"}`)
	s3Mgr := assertTarget(backup.TargetS3)
	if s3Mgr == webdavMgr {
		t.Fatal("BackupManager() still references the pre-rebuild manager after s3 switch")
	}
	if s3Mgr == localMgr {
		t.Fatal("BackupManager() still references the original manager after s3 switch")
	}

	// Switch back to local — rebuild from a cloud target must land back
	// on the local adapter too.
	update(`{"target_type": "local"}`)
	backAgain := assertTarget(backup.TargetLocal)
	if backAgain == s3Mgr {
		t.Fatal("BackupManager() still references the s3 manager after switching back to local")
	}
}

// TestRebuildBackupDisablesOnCloudConfigError verifies the cloud-target
// failure contract: when NewFromConfig fails for a cloud target (here:
// s3 without bucket/region, which NewS3Adapter rejects), RebuildBackup
// logs, sets a.Backup to nil, and returns the error — it must NOT fall
// back to a local adapter, since the settings advertise a cloud target.
func TestRebuildBackupDisablesOnCloudConfigError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
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

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)

	if err := sm.UpdateBackupConfigFromJSON(`{"target_type": "s3"}`); err != nil {
		t.Fatalf("update backup config: %v", err)
	}
	if err := a.RebuildBackup(context.Background()); err == nil {
		t.Fatal("RebuildBackup with broken s3 config should return an error")
	}
	if mgr := a.BackupManager(); mgr != nil {
		t.Fatalf("BackupManager() = %v after cloud config error, want nil (disabled)", mgr)
	}
}

// Note on the local-target fallback path: a lenient-fallback test for
// local targets is not constructible.  NewFromConfig for a local target
// can only fail inside NewLocal (os.MkdirAll of the derived backupDir —
// buildAdapter's local branch never fails), and RebuildBackup retries
// NewLocal with that same derived dir, so the fallback fails identically
// and always lands on the disable path below.  The disable contract for
// local targets is therefore pinned by
// TestRebuildBackupDisablesOnDoubleFailure.

// TestRebuildBackupDisablesOnDoubleFailure verifies the terminal path:
// when both NewFromConfig and the NewLocal fallback fail, RebuildBackup
// sets a.Backup to nil and returns the error.  The backup dir is forced
// to be uncreatable by placing a plain file where its parent dir would
// live, so os.MkdirAll fails with ENOTDIR.
func TestRebuildBackupDisablesOnDoubleFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
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

	blocker := filepath.Join(t.TempDir(), "not_a_dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, filepath.Join(blocker, "test.db"))

	err = a.RebuildBackup(context.Background())
	if err == nil {
		t.Fatal("RebuildBackup with uncreatable backup dir should return an error")
	}
	if mgr := a.BackupManager(); mgr != nil {
		t.Fatalf("BackupManager() = %v after double failure, want nil", mgr)
	}
}
