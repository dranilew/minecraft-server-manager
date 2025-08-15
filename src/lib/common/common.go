// Package common contains utilities common to all libraries and services.
package common

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func init() {
	flag.Parse()
}

// ServerStatus represents the status of a server.
type ServerStatus struct {
	// Name is the name of the server/modpack.
	Name string `json:"name"`
	// ShouldRun indicates if the server is expected to be running.
	ShouldRun bool `json:"should-run"`
	// Port is the port that the server is using.
	Port int `json:"port"`
	// StartTime is the time the server started.
	StartTime time.Time
}

const (
	// ServerInfoFile is the file containing server information.
	ServerInfoFile = "server.info"
	// BackupLockFile is the file containing backup information.
	BackupLockFile = "backup.lock"
)

var (
	// ModpackLocation is the location of the modpack files.
	ModpackLocation = flag.String("modpackdir", "/etc/minecraft/modpacks", "Location to find minecraft modpack installations")
	// BackupStatuses stores the status of all the backups.
	BackupStatuses   = make(map[string]bool)
	BackupStatusesMu sync.Mutex
	// ServerStatuses keeps track of server status.
	ServerStatuses   = make(map[string]*ServerStatus)
	ServerStatusesMu sync.Mutex
)

// InitStatuses initializes both status maps.
func InitStatuses() error {
	if err := initStatus(&ServerStatuses, &ServerStatusesMu, ServerInfoFile); err != nil {
		return err
	}
	if err := initStatus(&BackupStatuses, &BackupStatusesMu, BackupLockFile); err != nil {
		return err
	}
	return nil
}

// Init initializes the status map.
func initStatus(statusMap any, mu *sync.Mutex, file string) error {
	mu.Lock()
	defer mu.Unlock()
	statusFile := filepath.Join(*ModpackLocation, file)
	log.Printf("Reading status from file %q", statusFile)
	contentBytes, err := os.ReadFile(statusFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to read %s file: %w", file, err)
		}
		return nil
	}
	log.Printf("Got contents %s", string(contentBytes))
	if err := json.Unmarshal(contentBytes, statusMap); err != nil {
		return fmt.Errorf("failed to unmarshal %s file: %w", file, err)
	}
	return nil
}

// UpdateServerStatus updates the status of the server to the file on disk.
func UpdateServerStatus() error {
	return updateStatus(ServerStatuses, &ServerStatusesMu, ServerInfoFile)
}

// UpdateBackupStatus updates the status of the server to the file on disk.
func UpdateBackupStatus() error {
	return updateStatus(BackupStatuses, &BackupStatusesMu, BackupLockFile)
}

// updateStatus obtains the lock, marshals the map into a JSON, and writes it
// to the given file.
func updateStatus(statusMap any, mu *sync.Mutex, file string) error {
	mu.Lock()
	defer mu.Unlock()
	b, err := json.Marshal(statusMap)
	if err != nil {
		return err
	}
	path := filepath.Join(*ModpackLocation, file)
	if err := os.WriteFile(path, b, 0644); err != nil {
		return err
	}
	log.Printf("updated status to %s at %q", string(b), path)
	return nil
}

// ServerDirectory returns the location of the server's files.
func ServerDirectory(server string) string {
	return filepath.Join(*ModpackLocation, server)
}

// BackupLockPath is the location of the backup lock.
func BackupLockPath() string {
	return filepath.Join(*ModpackLocation, "backup.lock")
}
