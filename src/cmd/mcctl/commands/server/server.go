// Package server contains utilities for managing the server.
package server

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
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
	cmd.AddCommand(newInfoCommand())
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

func newInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Shows server information",
		Long:  "Shows server information, such as whether it should run, its start time, etc.",
		RunE:  serverInfo,
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

func serverInfo(*cobra.Command, []string) error {
	if err := common.InitStatuses(); err != nil {
		return fmt.Errorf("error initializing server status map: %v", err)
	}

	server.GetInfo(os.Stdout)
	return nil
}

// sendRequest sends a request to the command socket.
func sendRequest(cmd *cobra.Command, args []string) error {
	reqArgs := append([]string{"server", cmd.Name()}, args...)
	return monitor.SendCommand(context.Background(), []byte(strings.Join(reqArgs, " ")))
}
