//go:build linux

// Package main is the service that manages all the minecraft servers.
package main

import (
	"context"
	"fmt"
	"log"
	"log/syslog"
	"slices"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/common"
	"github.com/dranilew/minecraft-server-manager/src/lib/monitor"
	"github.com/dranilew/minecraft-server-manager/src/lib/run"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
	"github.com/dranilew/minecraft-server-manager/src/lib/status"
)

func main() {
	writer, err := syslog.New(syslog.LOG_INFO, "minecraft-server-manager")
	if err != nil {
		log.Fatalf(err.Error())
	}
	log.SetOutput(writer)
	if err := monitor.Setup(context.Background()); err != nil {
		log.Fatalf("Failed to setup command pipeline: %v", err)
	}

	if err := common.InitStatuses(); err != nil {
		log.Fatalf("Failed to initialize status maps: %v", err)
	}

	go recoverServers()
	go writeStatus()

	// Notify systemd that this is ready.
	opts := run.Options{
		Name:       "systemd-notify",
		Args:       []string{"--ready", "--status='Running Service...'"},
		OutputType: run.OutputNone,
	}

	if _, err := run.WithContext(context.Background(), opts); err != nil {
		log.Fatalf("Failed to notify systemd manager is ready: %v", err)
	}

	select {}
}

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
	for _, srv := range runningServers {
		common.ServerStatusesMu.Lock()
		s, ok := common.ServerStatuses[srv]
		if !ok {
			continue
		}
		online, err := status.Online(ctx, uint16(s.Port))
		common.ServerStatusesMu.Unlock()
		if err != nil {
			log.Printf("Error fetching %q server status: %v", srv, err)
		}
		if online > 0 {
			common.BackupStatusesMu.Lock()
			common.BackupStatuses[srv] = true
			common.BackupStatusesMu.Unlock()
		}
	}
	if err := common.UpdateBackupStatus(); err != nil {
		return fmt.Errorf("Failed to update backup status: %v", err)
	}
	return nil
}

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
