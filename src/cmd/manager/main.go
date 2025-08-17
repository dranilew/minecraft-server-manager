//go:build linux

// Package main is the service that manages all the minecraft servers.
package main

import (
	"context"
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

func main() {
	// Set up logger for syslog.
	if err := logger.Init("manager"); err != nil {
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

// recoverServers attempts to recover any servers that aren't running, but should
// be running.
func recoverServers() {
	ticker := time.NewTicker(time.Second)
	done := make(chan bool)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			handleCrash()
		}
	}
}

// writeStatus constantly polls the status of the servers and adjusts the backup
// locks as needed.
func writeStatus() {
	ticker := time.NewTicker(time.Second)
	done := make(chan bool)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			handleStatus()
		}
	}
}

// handleStatus only tries to unlock backups for a server if any players are detected online on a server.
// This checks once every second.
func handleStatus() error {
	ctx := context.Background()
	runningServers, err := server.GetRunningServers(ctx)
	if err != nil {
		return err
	}

	var changed bool
	for _, srv := range runningServers {
		common.ServerStatusesMu.Lock()
		s, ok := common.ServerStatuses[srv]
		if !ok {
			common.ServerStatusesMu.Unlock()
			continue
		}
		online, err := status.Online(ctx, uint16(s.Port))
		if err != nil {
			if time.Since(s.StartTime) < time.Minute {
				common.ServerStatusesMu.Unlock()
				continue
			}
			common.ServerStatusesMu.Unlock()
			logger.Printf("Error fetching %q server status: %v", srv, err)
			continue
		}
		common.ServerStatusesMu.Unlock()
		if online > 0 {
			common.BackupStatusesMu.Lock()
			changed = !common.BackupStatuses[srv] // This would only change if previous value is false.
			common.BackupStatuses[srv] = true
			common.BackupStatusesMu.Unlock()
		}
	}

	// Only update if something has changed.
	if changed {
		if err := common.UpdateBackupStatus(); err != nil {
			return fmt.Errorf("Failed to update backup status: %v", err)
		}
	}
	return nil
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
			return fmt.Errorf("Failed to recovery server %q: %v", k, err)
		}
	}
	return server.Start(ctx, startServers...)
}
