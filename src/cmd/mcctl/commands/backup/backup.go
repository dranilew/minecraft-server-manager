// Package backup is the command for managing backups.
package backup

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dranilew/minecraft-server-manager/src/lib/backup"
	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/monitor"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/spf13/cobra"
)

var (
	// gcsBucket is the destination bucket to which to upload backups. The backups
	// will use the destination [gcsBucket]/SERVERNAME
	gcsBucket string
	// force ignores any backup status locks and backs up the listed servers.
	force bool
	// skipUpload skips the upload task.
	skipUpload bool
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
	createCmd.Flags().BoolVar(&skipUpload, "skip-upload", false, "Skip uploading the backup file to GCS.")

	// Parse flags.
	createCmd.Flags().Parse([]string{"bucket", "force", "skip-upload"})

	cmd.AddCommand(createCmd)
	cmd.AddCommand(infoCmd)
	return cmd
}

// createBackup creates a backup
func createBackup(cmd *cobra.Command, args []string) error {
	var err error

	// Get the list of potential servers.
	potentialServers := args
	if slices.Contains(args, "all") {
		potentialServers, err = server.AllServers()
		if err != nil {
			return fmt.Errorf("failed to get all servers: %v", err)
		}
	}

	// Find out which servers actually need to be backed up.
	servers := potentialServers
	if !force {
		servers = nil // Reset servers list since we're not forcing.
		for _, server := range potentialServers {
			if backup, ok := common.BackupStatuses[server]; ok && backup {
				servers = append(servers, server)
			}
		}
	}

	// Log according to if we have any servers to backup.
	if len(servers) > 0 {
		logger.Printf("Creating backups for %v", servers)
		// Formulate the command monitor message.
		req := backup.CreateRequest{
			Force:      force,
			Bucket:     gcsBucket,
			SkipUpload: skipUpload,
			Servers:    servers,
		}
		reqJson, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal request %v: %v", req, err)
		}
		commandReq := strings.Join([]string{"backup", string(reqJson)}, " ")

		// Send the backup request to the manager.
		if err := monitor.SendCommand(context.Background(), []byte(commandReq)); err != nil {
			return fmt.Errorf("failed to send backup command: %v", err)
		}
	} else {
		logger.Printf("No backups to make, skipping.")
	}
	return nil
}

// backupInfo prints a pretty version of the backup.lock file.
func backupInfo(*cobra.Command, []string) error {
	w := tabwriter.NewWriter(os.Stdout, 5, 1, 2, ' ', 0)

	var result []string
	result = append(result, "NAME\tENABLED")

	// status represents an entry in the BackupStatuses map.
	type status struct {
		name    string
		enabled bool
	}
	// Marshal the map into a slice of status structs for sorting.
	var statuses []*status
	common.BackupStatusesMu.Lock()
	for k, v := range common.BackupStatuses {
		statuses = append(statuses, &status{name: k, enabled: v})
	}
	common.BackupStatusesMu.Unlock()

	// Sort the statuses by server name.
	slices.SortFunc(statuses, func(a *status, b *status) int {
		return cmp.Compare(a.name, b.name)
	})

	// Formulate the output.
	for _, v := range statuses {
		lineFields := []string{v.name, strconv.FormatBool(v.enabled)}
		line := strings.Join(lineFields, "\t")
		result = append(result, line)
	}

	// Print the output.
	fmt.Fprintln(w, strings.Join(result, "\n"))
	w.Flush()
	return nil
}

// initBackup initializes the status map.
func initStatus(*cobra.Command, []string) error {
	return common.InitStatuses()
}
