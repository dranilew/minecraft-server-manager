// Package backup is the command for managing backups.
package backup

import (
	"context"

	"github.com/dranilew/minecraft-server-manager/src/lib/backup"
	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/spf13/cobra"
)

var (
	// gcsBucket is the destination bucket to which to upload backups. The backups
	// will use the destination [gcsBucket]/SERVERNAME
	gcsBucket string
	// force ignores any backup status locks and backs up the listed servers.
	force bool
)

// New returns a new command for creating backups.
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "backup",
		Short:   "Manages backups",
		Long:    "Creates backups and uploads to a specified GCS URL",
		PreRunE: initStatus,
		RunE:    createBackup,
	}
	cmd.Flags().StringVar(&gcsBucket, "bucket", "", "The GCS bucket and location to which to store backups. This should contain gs://. The backups will use the destination [gcsBucket]/SERVERNAME")
	cmd.MarkFlagRequired("bucket")
	cmd.Flags().BoolVar(&force, "force", false, "Force a backup regardless of the current backup status.")

	// Parse flags.
	cmd.Flags().Parse([]string{"bucket", "force"})
	return cmd
}

// createBackup creates a backup
func createBackup(cmd *cobra.Command, args []string) error {
	return backup.Create(context.Background(), force, gcsBucket, args...)
}

// initBackup initializes the status map.
func initStatus(*cobra.Command, []string) error {
	return common.InitStatuses()
}
