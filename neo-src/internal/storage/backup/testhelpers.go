// Package backup — test helpers exported for cross-package test use.
//
// Export_test.go exports internal types and constructors so that tests
// in other packages (e.g., handlers) can construct BackupManagers with
// custom adapters without reaching into unexported fields.
package backup

import (
	"os"

	"little-timer/internal/storage"
)

// FakeLocalAdapter implements BackupAdapter with configurable behavior
// for testing.  Every method has an optional error field; when non-nil
// the method returns that error instead of its normal behavior.
//
// RestoreData, when non-nil, is written to the destination instead of
// whatever was recorded during Backup(), allowing tests to simulate
// SHA-256 mismatches.
type FakeLocalAdapter struct {
	backupDir   string
	backupData  []byte
	backupName  string
	backupErr   error
	restoreData []byte // if non-nil, Restore writes this instead of backupData
	restoreErr  error

	manifest     string
	manifestErr  error
	deletedNames []string
	deleteErr    error
}

// NewFakeLocalAdapter returns an adapter whose Backup() records the
// uploaded bytes and whose Restore() replays them (or restoreData if set).
func NewFakeLocalAdapter(backupDir string) *FakeLocalAdapter {
	return &FakeLocalAdapter{backupDir: backupDir}
}

// SetBackupError configures Backup() to return this error.
func (f *FakeLocalAdapter) SetBackupError(err error) { f.backupErr = err }

// SetRestoreData configures Restore() to write these bytes instead of
// what was recorded during Backup().
func (f *FakeLocalAdapter) SetRestoreData(data []byte) { f.restoreData = data }

// Target implements BackupAdapter.
func (f *FakeLocalAdapter) Target() BackupTarget { return TargetLocal }

// TestConnection implements BackupAdapter.
func (f *FakeLocalAdapter) TestConnection() error { return nil }

// Backup records the contents of srcPath.
func (f *FakeLocalAdapter) Backup(srcPath, backupName string) error {
	if f.backupErr != nil {
		return f.backupErr
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	f.backupData = data
	f.backupName = backupName
	return nil
}

// Restore writes restoreData (if set) or backupData to destPath.
func (f *FakeLocalAdapter) Restore(backupName, destPath string) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	data := f.backupData
	if f.restoreData != nil {
		data = f.restoreData
	}
	return os.WriteFile(destPath, data, 0o600)
}

// List implements BackupAdapter.
func (f *FakeLocalAdapter) List() ([]BackupInfo, error) { return nil, nil }

// Delete records the deleted name.
func (f *FakeLocalAdapter) Delete(backupName string) error {
	f.deletedNames = append(f.deletedNames, backupName)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

// WriteManifest stores the manifest JSON.
func (f *FakeLocalAdapter) WriteManifest(data string) error {
	if f.manifestErr != nil {
		return f.manifestErr
	}
	f.manifest = data
	return nil
}

// DeletedNames returns the list of names passed to Delete.
func (f *FakeLocalAdapter) DeletedNames() []string { return f.deletedNames }

// NewManagerWithAdapter constructs a BackupManager wired to the given
// sqlite, dbPath, backupDir, and adapter.  Exported so tests in other
// packages can inject fake adapters.
func NewManagerWithAdapter(
	sqlite *storage.SqliteManager,
	dbPath, backupDir string,
	adapter BackupAdapter,
) *BackupManager {
	return &BackupManager{
		sqlite:     sqlite,
		dbPath:     dbPath,
		backupDir:  backupDir,
		maxBackups: MaxBackups,
		adapter:    adapter,
	}
}
