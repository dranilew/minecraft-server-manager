// Package server contains utilities for managing the server.
package server

import (
	"context"
	"fmt"
	"strconv"
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

	var result []string
	result = append(result, "NAME\tPORT\tSHOULDRUN\tSTARTTIME")

	common.ServerStatusesMu.Lock()
	for _, v := range common.ServerStatuses {
		lineFields := []string{v.Name, strconv.Itoa(v.Port), strconv.FormatBool(v.ShouldRun), v.StartTime.String()}
		line := strings.Join(lineFields, "\t")
		result = append(result, line)
	}
	common.ServerStatusesMu.Unlock()
	fmt.Println(strings.Join(result, "\n"))
	return nil
}

func sendRequest(cmd *cobra.Command, args []string) error {
	reqArgs := append([]string{cmd.Parent().Name(), cmd.Name()}, args...)
	return monitor.SendCommand(context.Background(), []byte(strings.Join(reqArgs, " ")))
}
