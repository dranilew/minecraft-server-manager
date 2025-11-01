//go:build linux

// Package main is the service that manages all the minecraft servers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/monitor"
	"github.com/dranilew/minecraft-server-manager/src/lib/run"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/dranilew/minecraft-server-manager/src/lib/status"
)

var (
	// recoveryInterval is the interval at which the manager tries to recover
	// servers that crashed.
	recoveryInterval = flag.String("recovery_interval", "1s", "Interval at which the manager tries to recover servers that have been stopped or crashed unexpectedly.")
	// statusInterval is the interval at which the manager polls the status of
	// all servers it manages.
	statusInterval = flag.String("status_interval", "1s", "Interval at which the manager polls the status of all managed servers.")
	// extraScriptsInterval is the minimum interval at which all extra scripts
	// for all running servers are executed.
	extraScriptsInterval = flag.String("min_script_interval", "1m", "Interval at which the manager executes configured extra scripts for all running servers.")
)

func init() {
	flag.Parse()
}

func main() {
	if err := logger.Init("minecraft-server-manager"); err != nil {
		fmt.Printf("Failed to initialize loggers: %v\n", err)
		os.Exit(1)
	}

	// Set up command monitoring pipeline for use with mcctl.
	if err := monitor.Setup(context.Background()); err != nil {
		logger.Fatalf("Failed to setup command pipeline: %v", err)
	}

	// Initialize and parse the previously stored server and backup statuses.
	if err := common.InitStatuses(); err != nil {
		logger.Fatalf("Failed to initialize status maps: %v", err)
	}
	logger.Printf("ServerStatus: %+v", common.ServerStatuses)
	logger.Printf("BackupStatus: %+v", common.BackupStatuses)

	// Start to recover and monitor servers.
	go recoverServers()
	go writeStatus()
	go runExtraScripts()

	// Notify systemd that this is ready.
	opts := run.Options{
		Name:       "systemd-notify",
		Args:       []string{"--ready", "--status='Running Service...'"},
		OutputType: run.OutputNone,
	}

	if _, err := run.WithContext(context.Background(), opts); err != nil {
		logger.Fatalf("Failed to notify systemd manager is ready: %v", err)
	}

	select {}
}

// runExtraScripts attempts to run all extra scripts for all running servers
// every minute.
func runExtraScripts() {
	interval, err := time.ParseDuration(*extraScriptsInterval)
	if err != nil {
		logger.Fatalf("Failed to parse extra scripts interval duration: %v", err)
	}

	ticker := time.NewTicker(interval)
	done := make(chan bool)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := handleExtraScripts(); err != nil {
				logger.Printf("Failed to run extra scripts: %v", err)
			}
		}
	}
}

// recoverServers attempts to recover any servers that aren't running, but
// should be running.
func recoverServers() {
	interval, err := time.ParseDuration(*recoveryInterval)
	if err != nil {
		logger.Fatalf("Failed to parse recovery interval duration: %v", err)
	}

	ticker := time.NewTicker(interval)
	done := make(chan bool)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := handleCrash(); err != nil {
				logger.Printf("Failed to handle crash: %v", err)
			}
		}
	}
}

// writeStatus constantly polls the status of the servers and adjusts the backup
// locks as needed.
func writeStatus() {
	interval, err := time.ParseDuration(*statusInterval)
	if err != nil {
		logger.Fatalf("Failed to parse status interval duration: %v", err)
	}

	ticker := time.NewTicker(interval)
	done := make(chan bool)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := handleStatus(); err != nil {
				logger.Printf("Failed to handle server statuses: %v", err)
			}
		}
	}
}

// handleStatus only tries to unlock backups for a server if any players are
// detected online on a server.
func handleStatus() error {
	ctx := context.Background()
	runningServers, err := server.GetRunningServers(ctx)
	if err != nil {
		return err
	}

	var errs []error
	var changed bool
	for _, srv := range runningServers {
		// Ignore if the server shouldn't be running.
		common.ServerStatusesMu.Lock()
		if !common.ServerStatuses[srv].ShouldRun {
			common.ServerStatusesMu.Unlock()
			continue
		}
		common.ServerStatusesMu.Unlock()

		// Get the server's previous status. If it doesn't exist, then it isn't
		// registered, so ignore it.
		common.ServerStatusesMu.Lock()
		s, ok := common.ServerStatuses[srv]
		if !ok {
			common.ServerStatusesMu.Unlock()
			continue
		}
		// Get the server's current status.
		online, err := status.Online(ctx, uint16(s.Port))
		if err != nil {
			if time.Since(s.StartTime) < time.Minute {
				common.ServerStatusesMu.Unlock()
				continue
			}
			common.ServerStatusesMu.Unlock()
			errs = append(errs, fmt.Errorf("Error fetching %q server status: %v", srv, err))
			continue
		}
		common.ServerStatusesMu.Unlock()

		// Unlock backups if a player is online.
		if online > 0 {
			logger.Debugf("Players found online for %v, enabling backups", srv)
			common.BackupStatusesMu.Lock()
			// This would only change if previous value is false.
			changed = !common.BackupStatuses[srv]
			common.BackupStatuses[srv] = true
			common.BackupStatusesMu.Unlock()
		}
	}

	// Only update if something has changed.
	if changed {
		if err := common.UpdateBackupStatus(); err != nil {
			return fmt.Errorf("failed to update backup status: %v", err)
		}
	}
	return errors.Join(errs...)
}

// handleCrash attempts to bring back servers that are crashed or stopped.
func handleCrash() error {
	ctx := context.Background()
	runningServers, err := server.GetRunningServers(ctx)
	if err != nil {
		return err
	}
	var startServers []string
	for k, v := range common.ServerStatuses {
		// If server should run but isn't, we start it again.
		if v.ShouldRun && !slices.Contains(runningServers, k) {
			startServers = append(startServers, k)
		}
		// Sometimes server is still running despite having crashed.
		// We need to check crash reports and recover if needed.
		if err := server.Recover(ctx, k); err != nil {
			return fmt.Errorf("failed to recover server %q: %v", k, err)
		}
	}
	// Start all the servers that have been deemed to have crashed or have stopped unexpectedly.
	return server.Start(ctx, startServers...)
}

// handleExtraScripts runs extra scripts for every single server as specified in their configuration files.
func handleExtraScripts() error {
	ctx := context.Background()
	runningServers, err := server.GetRunningServers(ctx)
	if err != nil {
		return err
	}

	// Execute extra scripts for all running servers.
	var errs []error
	for _, srv := range runningServers {
		if err := server.ExtraScripts(ctx, srv); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
