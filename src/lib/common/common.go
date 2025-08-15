// Package common contains utilities common to all libraries and services.
package common

import (
	"flag"
	"path/filepath"
	"sync"
)

func init() {
	flag.Parse()
}

// BackupStatus represents the status of backups for a certain server.
type BackupStatus struct {
	// Enabled indicates whether to backup the server on the next cycle.
	Enabled bool `json:"enabled"`
	// enabledMu is the Mutex for Enabled. This must be held when accessing or changing
	// the value of Enabled.
	enabledMu sync.Mutex `json:"-"`
}

type ServerStatus struct {
	// Name is the name of the server/modpack.
	Name string
	// ShouldRun indicates if the server is expected to be running.
	ShouldRun bool
	// Port is the port that the server is using.
	Port int
}

var (
	// ModpackLocation is the location of the modpack files.
	ModpackLocation = flag.String("modpackdir", "/etc/minecraft/modpacks", "Location to find minecraft modpack installations")
	// BackupStatuses stores the status of all the backups.
	BackupStatuses   map[string]*BackupStatus
	BackupStatusesMu sync.Mutex
	// ServerStatuses keeps track of server status.
	ServerStatuses   = make(map[string]*ServerStatus)
	ServerStatusesMu sync.Mutex
)

// ServerDirectory returns the location of the server's files.
func ServerDirectory(server string) string {
	return filepath.Join(*ModpackLocation, server)
}

// BackupLockFile is the location of the backup lock.
func BackupLockFile() string {
	return filepath.Join(*ModpackLocation, "backup.lock")
}
