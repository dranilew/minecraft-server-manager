//go:build linux

// Package main is the command line tool for managing minecraft servers.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dranilew/minecraft-server-manager/src/cmd/mcctl/commands/backup"
	"github.com/dranilew/minecraft-server-manager/src/cmd/mcctl/commands/server"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()
	rootCmd := &cobra.Command{
		Use:   "mcctl",
		Short: "Minecraft server management CLI",
		Long:  "Minecraft server management CLI used to trigger backups.",
	}

	rootCmd.PersistentFlags().BoolVar(logger.Debug, "v", false, "Whether to print more logs")
	rootCmd.ParseFlags([]string{"v"})

	rootCmd.AddCommand(backup.New())
	rootCmd.AddCommand(server.New())

	if err := logger.Init("mcctl", os.Stdout); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Fatalf("Failed to execute: %v", err)
	}
}
