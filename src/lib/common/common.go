// Package common contains utilities common to all libraries and services.
package common

import (
	"flag"
	"path/filepath"
)

func init() {
	flag.Parse()
}

var (
	// ModpackLocation is the location of the modpack files.
	ModpackLocation = flag.String("modpackdir", "/etc/minecraft/modpacks", "Location to find minecraft modpack installations")
)

// ServerDirectory returns the location of the server's files.
func ServerDirectory(server string) string {
	return filepath.Join(*ModpackLocation, server)
}

// BackupLockFile is the location of the backup lock.
func BackupLockFile() string {
	return filepath.Join(*ModpackLocation, "backup.lock")
}
