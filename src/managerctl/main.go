// Package managerctl is the command line tool for managing minecraft servers.
package managerctl

import (
	"context"
	"log"

	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()
	rootCmd := &cobra.Command{
		Use:   "mcctl",
		Short: "Minecraft server management CLI",
		Long:  "Minecraft server management CLI used to trigger backups.",
	}

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}
}
