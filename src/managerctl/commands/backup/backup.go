// Package backup is the command for managing backups.
package backup

import (
	"github.com/dranilew/minecraft-server-manager/src/lib/backup"
	"github.com/spf13/cobra"
)

var (
	// gcsBucket is the destination bucket to which to upload backups. The backups
	// will use the destination gs://[gcsBucket]/SERVERNAME
	gcsBucket string
)

// New returns a new command for creating backups.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manages backups",
		Long:  "Creates backups and uploads to a specified GCS URL",
		RunE:  createBackup,
	}
	cmd.PersistentFlags().StringVar()
}

// createBackup creates a backup
func createBackup(cmd *cobra.Command, args []string) error {
	return backup.Create()
}
