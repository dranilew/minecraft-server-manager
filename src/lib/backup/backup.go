// Package backup creates backups.
package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/dranilew/minecraft-server-manager/src/lib/status"
)

// Init initializes the common.BackupStatuses map.
func Init() error {
	common.BackupStatusesMu.Lock()
	defer common.BackupStatusesMu.Unlock()
	contentBytes, err := os.ReadFile(filepath.Join(*common.ModpackLocation, "backup.lock"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Failed to read backup lock file: %v", err)
		}
		return nil
	}
	if err := json.Unmarshal(contentBytes, &common.BackupStatuses); err != nil {
		return fmt.Errorf("Failed to unmarshal backup lock file: %v", err)
	}
	return nil
}

// Create creates a backup for all servers in the list.
// dest is the destination Google Cloud storage location.
func Create(ctx context.Context, force bool, dest string, servers ...string) error {
	var errs []error
	for _, srv := range servers {
		common.BackupStatusesMu.Lock()
		if force || common.BackupStatuses[srv].Enabled {
			common.BackupStatusesMu.Unlock()
			if err := createBackup(ctx, srv, dest); err != nil {
				errs = append(errs, err)
			}

			// If no one is online, then stop doing backups. We assume that the server is also
			// not running when Online returns an error.
			online, _ := status.Online(ctx, uint16(common.ServerStatuses[srv].Port))
			if online == 0 {
				common.BackupStatusesMu.Lock()
				common.BackupStatuses[srv].Enabled = false
				common.BackupStatusesMu.Unlock()
			}
		} else {
			common.BackupStatusesMu.Unlock()
		}
	}
	return errors.Join(errs...)
}

// backupName is the name of the backup.
func backupName(server string) string {
	return fmt.Sprintf("%s-backup.zip", server)
}

// createBackup creates a backup for the specific server.
func createBackup(ctx context.Context, srv, dest string) error {
	worldDir := filepath.Join(common.ServerDirectory(srv), "world")
	currTime := time.Now().Format(time.RFC3339)

	// Create a temporary directory for storing the zip file.
	server.Notify(ctx, srv, "Creating backup...")
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", srv, currTime)) // Temporary directory to store the zip file.
	backupFile := filepath.Join(tempDir, backupName(srv))

	// Create the zip file.
	zipFile, err := os.Create(backupFile)
	if err != nil {
		return fmt.Errorf("failed to create zip file %q: %v", backupFile, err)
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Copy all files in the world directory into the zip file.
	if err := copyToZip(zipWriter, worldDir, ""); err != nil {
		return fmt.Errorf("failed to copy world files to zip folder: %v", err)
	}

	// Upload to the storage bucket if a URL is provided.
	if err := exec.Command("gcloud", "storage", "cp", backupFile, fmt.Sprintf("gs://%s/%s/%s", dest, srv, backupName(srv))).Run(); err != nil {
		return fmt.Errorf("failed to upload %q to %q: %v", backupFile, dest, err)
	}
	os.RemoveAll(tempDir) // Only remove if the file has been successfully uploaded.

	// Notify that the backup has been created.
	server.Notify(ctx, srv, fmt.Sprintf("Backup created at %s", currTime))
	return nil
}

// copyToZip recurses through all files from baseDir and adds them to the zip file.
func copyToZip(zipWriter *zip.Writer, baseDir, relativeDir string) error {
	var errs []error
	var errsMu sync.Mutex
	var addError = func(err error) {
		if err != nil {
			errsMu.Lock()
			errs = append(errs, err)
			errsMu.Unlock()
		}
	}

	files, err := os.ReadDir(filepath.Join(baseDir, relativeDir))
	if err != nil {
		addError(err)
	}

	// Add each file in their own goroutines.
	for _, file := range files {
		// Recurse if it's a directory. Run these in a separate
		if file.IsDir() {
			go addError(copyToZip(zipWriter, baseDir, filepath.Join(relativeDir, file.Name())))
		}

		go func() {
			// Copy all non-directory files.
			zipLoc := filepath.Join(relativeDir, file.Name())
			zipFile, err := zipWriter.Create(zipLoc)
			if err != nil {
				addError(err)
			}
			copyFile, err := os.Open(filepath.Join(baseDir, relativeDir, file.Name()))
			if err != nil {
				addError(err)
			}
			if _, err := io.Copy(zipFile, copyFile); err != nil {
				addError(err)
			}
			copyFile.Close()
		}()
	}
	return errors.Join(errs...)
}

// Writecommon.BackupStatus writes the backup common.BackupStatus.
func WriteBackupStatus() error {
	common.BackupStatusesMu.Lock()
	defer common.BackupStatusesMu.Unlock()

	// Don't write anything if the map is empty.
	if common.BackupStatuses == nil || len(common.BackupStatuses) == 0 {
		return nil
	}

	// Marshal and write the JSON.
	b, err := json.Marshal(common.BackupStatuses)
	if err != nil {
		return err
	}
	if err := os.WriteFile(common.BackupLockFile(), b, 0644); err != nil {
		return fmt.Errorf("failed to write backup lock file: %v", err)
	}
	return nil
}
