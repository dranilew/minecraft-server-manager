// Package backup is the command for managing backups.
package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dranilew/minecraft-server-manager/src/lib/backup"
	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
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
		Use:               "backup",
		Short:             "Manages backups",
		Long:              "Provides commands for managing backups for all servers.",
		PersistentPreRunE: initStatus,
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Creates a backup",
		Long:  "Create a backup, uploaded to the specified bucket. Specifying 'all' creates a backup for all servers.",
		RunE:  createBackup,
	}

	infoCmd := &cobra.Command{
		Use:   "info",
		Short: "Gets backup lock information",
		Long:  "Gets backup lock information",
		RunE:  backupInfo,
	}
	createCmd.Flags().StringVar(&gcsBucket, "bucket", "", "The GCS bucket and location to which to store backups. This should contain gs://. The backups will use the destination [gcsBucket]/SERVERNAME")
	createCmd.MarkFlagRequired("bucket")
	createCmd.Flags().BoolVar(&force, "force", false, "Force a backup regardless of the current backup status.")

	// Parse flags.
	createCmd.Flags().Parse([]string{"bucket", "force"})

	cmd.AddCommand(createCmd)
	cmd.AddCommand(infoCmd)
	return cmd
}

// createBackup creates a backup
func createBackup(cmd *cobra.Command, args []string) error {
	var err error
	servers := args
	if slices.Contains(args, "all") {
		servers, err = server.AllServers(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get all servers: %v", err)
		}
	}
	log.Printf("Creating backups for %v", servers)
	return backup.Create(context.Background(), force, gcsBucket, servers...)
}

// backupInfo prints a pretty version of the backup.lock file.
func backupInfo(*cobra.Command, []string) error {
	w := tabwriter.NewWriter(os.Stdout, 5, 1, 2, ' ', 0)

	var result []string
	result = append(result, "NAME\tENABLED")

	common.BackupStatusesMu.Lock()
	for k, v := range common.BackupStatuses {
		lineFields := []string{k, strconv.FormatBool(v)}
		line := strings.Join(lineFields, "\t")
		result = append(result, line)
	}
	common.BackupStatusesMu.Unlock()
	fmt.Fprintln(w, strings.Join(result, "\n"))
	w.Flush()
	return nil
}

// initBackup initializes the status map.
func initStatus(*cobra.Command, []string) error {
	return common.InitStatuses()
}
