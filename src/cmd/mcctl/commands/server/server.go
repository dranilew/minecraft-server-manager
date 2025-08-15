// Package server contains utilities for managing the server.
package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/dranilew/minecraft-server-manager/src/lib/monitor"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manages the servers",
		Long:  "Starts, restarts, or stops the servers.",
		RunE:  listServers,
	}

	cmd.AddCommand(newStartCommand())
	cmd.AddCommand(newRestartCommand())
	cmd.AddCommand(newStopCommand())
	return cmd
}

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start <servers>",
		Short: "Starts a server",
		Long:  "Starts all listed servers.",
		RunE:  sendRequest,
	}
}

func newRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <servers>",
		Short: "Restarts a server",
		Long:  "Restarts all listed servers.",
		RunE:  sendRequest,
	}
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <servers>",
		Short: "Stops a sever",
		Long:  "Stops all listed servers.",
		RunE:  sendRequest,
	}
}

func listServers(*cobra.Command, []string) error {
	srvs, err := server.GetRunningServers(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(srvs)
	return nil
}

func sendRequest(cmd *cobra.Command, args []string) error {
	return monitor.SendCommand(context.Background(), []byte(strings.Join(args, " ")))
}
