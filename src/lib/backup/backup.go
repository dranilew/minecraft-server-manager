// Package backup creates backups.
package backup

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/dranilew/minecraft-server-manager/src/lib/status"
)

// Create creates a backup for all servers in the list.
// dest is the destination Google Cloud storage location.
func Create(ctx context.Context, force bool, dest string, servers ...string) error {
	var errs []error
	var errsMu sync.Mutex
	var wg sync.WaitGroup
	var backupMade atomic.Bool

	for _, srv := range servers {
		wg.Add(1)
		go func() {
			errsMu.Lock()
			defer errsMu.Unlock()
			defer wg.Done()
			backedUp, err := createBackup(ctx, force, srv, dest)
			errs = append(errs, err)
			if backedUp {
				backupMade.Store(true)
			}
		}()
	}
	wg.Wait()
	if backupMade.Load() {
		errs = append(errs, common.UpdateBackupStatus())
	}
	return errors.Join(errs...)
}

// backupName is the name of the backup.
func backupName(server string) string {
	return fmt.Sprintf("%s-backup.zip", server)
}

// shouldBackup indicates whether the given server should be backed up.
func shouldBackup(force bool, srv string) bool {
	common.BackupStatusesMu.Lock()
	defer common.BackupStatusesMu.Unlock()

	// Backup status might not exist if it's a new server.
	// Automatically assume a backup should be made if it doesn't exist.
	status, ok := common.BackupStatuses[srv]
	if !ok {
		common.BackupStatuses[srv] = true
		return true
	}
	return force || status
}

// createBackup creates a backup for the specific server.
func createBackup(ctx context.Context, force bool, srv, dest string) (bool, error) {
	if !shouldBackup(force, srv) {
		return false, nil
	}
	worldDir := filepath.Join(common.ServerDirectory(srv), "world")
	currTime := time.Now().Format(time.RFC3339)

	// Force save the server, and notify about the backup.
	server.Notify(ctx, srv, "Creating backup...")
	server.ForceSave(ctx, srv)

	// Create a temporary file for zipping
	zipFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.zip", srv)) // Temporary directory to store the zip file.
	if err != nil {
		return false, fmt.Errorf("failed to create zip file %q: %v", zipFile.Name(), err)
	}
	backupFile := zipFile.Name()
	defer zipFile.Close()

	// Let the zipfile be readable by others.
	if err := zipFile.Chmod(0644); err != nil {
		return false, fmt.Errorf("failed to chmod zipfile: %v", err)
	}

	// Create the zip file.
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Copy all files in the world directory into the zip file.
	if err := copyToZip(zipWriter, worldDir, ""); err != nil {
		return false, fmt.Errorf("failed to copy world files to zip folder: %v", err)
	}

	fullDestination := fmt.Sprintf("%s/%s/%s", dest, srv, backupName(srv))
	fmt.Printf("Zip file created at %s, uploading to %s\n", backupFile, fullDestination)
	// Upload to the storage bucket if a URL is provided.
	if err := exec.Command("gcloud", "storage", "cp", backupFile, fullDestination).Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false, fmt.Errorf("failed to upload %q to %q: %v", backupFile, fullDestination, err)
		}
		return false, fmt.Errorf("failed to upload %q to %q: %v", backupFile, fullDestination, string(exitErr.Stderr))
	}
	if err := os.Remove(backupFile); err != nil {
		log.Printf("Failed to remove temporary zip file: %v", err)
	}

	// Notify that the backup has been created.
	server.Notify(ctx, srv, fmt.Sprintf("Backup created at %s", currTime))

	// If no one is online, then stop doing backups. We assume that the server is also
	// not running when Online returns an error.
	common.BackupStatusesMu.Lock()
	online, _ := status.Online(ctx, uint16(common.ServerStatuses[srv].Port))
	if online == 0 {
		common.BackupStatuses[srv] = false
	}
	common.BackupStatusesMu.Unlock()
	return true, nil
}

// copyToZip recurses through all files from baseDir and adds them to the zip file.
func copyToZip(zipWriter *zip.Writer, baseDir, relativeDir string) error {
	var errs []error

	// Read all files in the directory.
	files, err := os.ReadDir(filepath.Join(baseDir, relativeDir))
	if err != nil {
		errs = append(errs, err)
	}

	// Zip files cannot be created concurrently.
	for _, file := range files {
		// Recurse if it's a directory.
		if file.IsDir() {
			errs = append(errs, copyToZip(zipWriter, baseDir, filepath.Join(relativeDir, file.Name())))
			continue // Don't add directories to the zip file.
		}

		// Copy all non-directory files.
		zipLoc := filepath.Join(relativeDir, file.Name())
		zipFile, err := zipWriter.Create(zipLoc)
		if err != nil {
			errs = append(errs, err)
		}
		copyFile, err := os.Open(filepath.Join(baseDir, relativeDir, file.Name()))
		if err != nil {
			errs = append(errs, err)
		}
		if _, err := io.Copy(zipFile, copyFile); err != nil {
			errs = append(errs, err)
		}
		copyFile.Close()
	}
	return errors.Join(errs...)
}
