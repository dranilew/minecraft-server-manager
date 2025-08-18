// Package server contains utilities for starting and stopping servers.
package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/run"
)

const (
	// killServerTimeout is the number of seconds to wait for a server to stop before force-killing it.
	killServerTimeout = 15
	// This is the base server port. All other server ports are incremented above this.
	baseServerPort = 25565
	// crashReportsDir is the directory containing crash reports.
	crashReportsDir = "crash-reports"
	// recoveryTime is the time to wait for recovery.
	recoveryTime = 30 * time.Second
)

var (
	// crashReportsRegex is the regex for crash reports.
	crashReportsRegex = regexp.MustCompile("[0-9]+-[0-9]+-[0-9]+_[0-9]+.[0-9]+.[0-9]+")
)

// GetRunningServers gets the list of servers running on the machine.
func GetRunningServers(ctx context.Context) ([]string, error) {
	opts := run.Options{
		Name: "screen",
		Args: []string{
			"-ls",
		},
		OutputType: run.OutputCombined,
		ExecMode:   run.ExecModeSync,
	}
	res, _ := run.WithContext(ctx, opts)
	if res == nil { // Errors when nothing is found.
		return nil, nil
	}
	lines := strings.Split(res.Output, "\n")

	// Find the servers running on the machine.
	var servers []string
	for _, line := range lines {
		if !strings.Contains(line, "server") {
			continue
		}

		// First field is the PID.MODPACK.server name.
		screenName := strings.Fields(line)[0]
		// What we want is the MODPACK name.
		serverName := strings.Split(screenName, ".")[1]
		servers = append(servers, serverName)
	}
	return servers, nil
}

// AllServers returns all possible servers, located in the base server directory.
func AllServers() ([]string, error) {
	dirEntries, err := os.ReadDir(*common.ModpackLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to read modpack directory: %v", err)
	}
	var res []string
	for _, entry := range dirEntries {
		if entry.IsDir() { // Only care about directories.
			res = append(res, entry.Name())
		}
	}
	return res, nil
}

// Notify notifies the server with the given message.
func Notify(ctx context.Context, server string, message string) error {
	runningServers, err := GetRunningServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running servers: %v", err)
	}
	if !slices.Contains(runningServers, server) {
		logger.Printf("Server %q is not running, skipping notification", server)
	}
	opts := run.Options{
		Name: "screen",
		Args: []string{
			"-S",
			server,
			"-X",
			"stuff",
			fmt.Sprintf("/say %s^M", message),
		},
		OutputType: run.OutputNone,
	}
	if _, err := run.WithContext(ctx, opts); err != nil {
		return fmt.Errorf("failed to notify server %q: %v", server, message)
	}
	return nil
}

// ForceSave forces a save operation on the server.
func ForceSave(ctx context.Context, server string) error {
	runningServers, err := GetRunningServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running servers: %v", err)
	}
	if !slices.Contains(runningServers, server) {
		logger.Printf("Server %q is not running, skipping notification", server)
	}
	opts := run.Options{
		Name: "screen",
		Args: []string{
			"-S",
			server,
			"-X",
			"stuff",
			"/save-all^M",
		},
		OutputType: run.OutputNone,
	}
	if _, err := run.WithContext(ctx, opts); err != nil {
		return fmt.Errorf("failed to force-save server %q: %v", server, err)
	}
	return nil
}

// determinePort determines the port to use for the server.
// The boolean indicates whether the server is a new server.
func determinePort(server string) (int, bool) {
	// Server already has a port assigned.
	status, ok := common.ServerStatuses[server]
	if ok {
		return status.Port, false
	}
	port := baseServerPort

	// Determine the first available port.
	common.ServerStatusesMu.Lock()
	defer common.ServerStatusesMu.Unlock()
	isValid := func(port int) bool {
		for _, v := range common.ServerStatuses {
			if v.Port == port {
				return false
			}
		}
		return true
	}

	for !isValid(port) {
		port++
	}
	return port, true
}

// setPort modifies the server's server.properties file to use the new port.
func setPort(server string, port int) error {
	serverDir := common.ServerDirectory(server)
	propertiesFile := filepath.Join(serverDir, "server.properties")
	properties, err := os.ReadFile(propertiesFile)
	if err != nil {
		return fmt.Errorf("failed to read %q server.properties: %v", server, err)
	}
	lines := strings.Split(string(properties), "\n")

	// Replace the proper lines in the server.properties file.
	var resLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "query.port") {
			line = fmt.Sprintf("query.port=%d", port)
		}
		if strings.HasPrefix(line, "server-port") {
			line = fmt.Sprintf("server-port=%d", port)
		}
		resLines = append(resLines, line)
	}
	if err := os.WriteFile(propertiesFile, []byte(strings.Join(resLines, "\n")), 0755); err != nil {
		return fmt.Errorf("failed to write %q server.properties: %v", server, err)
	}
	return nil
}

// Start starts all the servers.
func Start(ctx context.Context, servers ...string) error {
	runningServers, err := GetRunningServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running servers: %v", err)
	}

	var started bool
	for _, server := range servers {
		logger.Printf("Starting server %q", server)
		if slices.Contains(runningServers, server) {
			logger.Printf("Server %q already running, skipping launch", server)
			continue
		}

		started = true
		logger.Printf("%q: Determining port for server...", server)
		port, isNew := determinePort(server)
		common.ServerStatusesMu.Lock()
		if isNew {
			logger.Printf("%q: Setting port to %d", server, port)
			if err := setPort(server, port); err != nil {
				common.ServerStatusesMu.Unlock()
				return fmt.Errorf("Failed to set port for server %q: %v", server, err)
			}
			common.ServerStatuses[server] = &common.ServerStatus{
				Name: server,
				Port: port,
			}
		} else {
			logger.Printf("Got port %d for server %q", port, server)
		}
		common.ServerStatuses[server].ShouldRun = true
		common.ServerStatuses[server].StartTime = time.Now()
		common.ServerStatusesMu.Unlock()

		// Start the server.
		entry := filepath.Join(common.ServerDirectory(server), "run.sh")
		opts := run.Options{
			Name: "screen",
			Args: []string{
				"-S",
				fmt.Sprintf("%s.server", server),
				"-d",
				"-m",
				"./run.sh",
			},
			Dir:        common.ServerDirectory(server),
			OutputType: run.OutputCombined,
			ExecMode:   run.ExecModeDetach,
		}
		if _, err := run.WithContext(ctx, opts); err != nil {
			return fmt.Errorf("Failed to start server %s: %v", server, err)
		}
		logger.Printf("Started server %q from %q", server, entry)
	}
	if started {
		// Only update status if a new server is started.
		if err := common.UpdateServerStatus(); err != nil {
			return fmt.Errorf("Failed to update server status: %v", err)
		}
	}

	return nil
}

// Stop stops all the specified servers.
func Stop(ctx context.Context, servers ...string) error {
	runningServers, err := GetRunningServers(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get currently running servers")
	}
	var stopped bool
	var wg sync.WaitGroup
	for _, server := range servers {
		// Stop/kill each specified server in their own go routines.
		wg.Go(func() {
			// If the server is already not running, we do nothing.
			if !slices.Contains(runningServers, server) {
				return
			}

			// Server shouldn't run anymore. Reset start time.
			stopped = true
			common.ServerStatusesMu.Lock()
			common.ServerStatuses[server].ShouldRun = false
			common.ServerStatuses[server].StartTime = time.Time{}
			common.ServerStatusesMu.Unlock()

			// Attempt to stop the server naturally.
			opts := run.Options{
				Name: "screen",
				Args: []string{
					"-S",
					server,
					"-X",
					"stuff",
					"stop^M",
				},
				OutputType: run.OutputCombined,
				ExecMode:   run.ExecModeDetach,
			}
			if _, err := run.WithContext(ctx, opts); err != nil {
				logger.Printf("Failed to stop server %q: %v", server, err)
				return
			}

			// Poll the list to see if it's stopped. If it's no longer there, we're good.
			// Otherwise, we wait until a specified timeout before force-killing the server.
			currentServers, err := GetRunningServers(ctx)
			if err != nil {
				logger.Printf("failed to get currently running servers: %v", err)
				return
			}
			var counter int
			for slices.Contains(currentServers, server) && counter < killServerTimeout {
				time.Sleep(time.Second)
				counter++
				currentServers, err = GetRunningServers(ctx)
				if err != nil {
					logger.Printf("failed to get currently running servers: %v", err)
				}
			}
			if counter >= killServerTimeout {
				logger.Printf("Server did not exit within timeout, force-killing...")
				Kill(ctx, false, server)
			}

			// Enable backups one last time.
			common.BackupStatusesMu.Lock()
			common.BackupStatuses[server] = true
			common.BackupStatusesMu.Unlock()
		})
	}
	wg.Wait()
	if stopped {
		// Only update if an existing server is actually stopped.
		if err := common.UpdateServerStatus(); err != nil {
			return fmt.Errorf("failed to update server status: %v", err)
		}
	}
	return nil
}

// Restart stops and starts all the specified servers.
func Restart(ctx context.Context, servers ...string) error {
	if err := Stop(ctx, servers...); err != nil {
		return fmt.Errorf("Failed to stop servers: %v", err)
	}
	if err := Start(ctx, servers...); err != nil {
		return fmt.Errorf("Failed to start servers: %v", err)
	}
	return nil
}

// Kill force-stops the server. This should be avoided unless the server
// fails to shut down the normal way.
func Kill(ctx context.Context, recover bool, server string) error {
	if !recover {
		common.ServerStatusesMu.Lock()
		common.ServerStatuses[server].ShouldRun = false
		common.ServerStatusesMu.Unlock()
	}
	killOpts := run.Options{
		Name: "screen",
		Args: []string{
			"-S",
			server,
			"-X",
			"quit",
		},
		OutputType: run.OutputNone,
		ExecMode:   run.ExecModeAsync,
	}
	if _, err := run.WithContext(ctx, killOpts); err != nil {
		return fmt.Errorf("failed to force-kill server %q: %v", server, err)
	}
	return nil
}

// Recover attempts to recover the server if it's detected to have crashed.
func Recover(ctx context.Context, server string) error {
	crashReportsLoc := filepath.Join(common.ServerDirectory(server), crashReportsDir)
	reports, err := os.ReadDir(crashReportsLoc)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	for _, report := range reports {
		if report.IsDir() {
			continue
		}

		// Parse the time from the filename.
		fileName := report.Name()
		dateTime := string(crashReportsRegex.Find([]byte(fileName)))
		crashTime, err := time.Parse("2006-01-02_15.04.05", dateTime)
		if err != nil {
			return fmt.Errorf("failed to parse crash reports time: %v", err)
		}

		// If the server crashed in the last 30 seconds, attempt to restart the server.
		common.ServerStatusesMu.Lock()
		srvRecoveryState := common.ServerStatuses[server].Recovering
		common.ServerStatusesMu.Unlock()
		if time.Since(crashTime) < recoveryTime && !srvRecoveryState {
			common.ServerStatusesMu.Lock()
			common.ServerStatuses[server].Recovering = true
			common.ServerStatusesMu.Unlock()
			logger.Printf("Crash detected for server %q", server)
			if err := Kill(ctx, true, server); err != nil {
				return fmt.Errorf("failed to kill crashed server %q: %v", server, err)
			}
			go func() {
				time.Sleep(recoveryTime)

				// Reset Recovering to false.
				common.ServerStatusesMu.Lock()
				common.ServerStatuses[server].Recovering = false
				common.ServerStatusesMu.Unlock()
			}()
			return Start(ctx, server)
		}
	}
	return nil
}
