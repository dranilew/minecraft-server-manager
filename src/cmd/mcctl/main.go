//go:build linux

// Package main is the command line tool for managing minecraft servers.
package main

import (
	"context"
	"log"
	"log/syslog"

	"github.com/dranilew/minecraft-server-manager/src/cmd/mcctl/commands/backup"
	"github.com/dranilew/minecraft-server-manager/src/cmd/mcctl/commands/server"
	"github.com/spf13/cobra"
)

func main() {
	writer, err := syslog.New(syslog.LOG_INFO, "mcctl")
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.SetOutput(writer)
	ctx := context.Background()
	rootCmd := &cobra.Command{
		Use:   "mcctl",
		Short: "Minecraft server management CLI",
		Long:  "Minecraft server management CLI used to trigger backups.",
	}

	rootCmd.AddCommand(backup.New())
	rootCmd.AddCommand(server.New())
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}
}
