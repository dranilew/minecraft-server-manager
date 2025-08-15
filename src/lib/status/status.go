// Package status gets the status of the server.
package status

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/dranilew/minecraft-server-manager/src/lib/run"
	"github.com/mcstatus-io/mcutil/v4/status"
)

var (
	// ServerIP is the IP of the server.
	ServerIP string
	// ipRegex is the regex for the IP.
	ipRegex = regexp.MustCompile("[0-9]+.[0-9]+.[0-9]+.[0-9]+")
)

func init() {
	// Initialize the server IP.
	opts := run.Options{
		Name: "dig",
		Args: []string{
			"TXT",
			"+short",
			"o-o.myaddr.l.google.com",
			"@ns1.google.com",
		},
	}
	out, err := run.WithContext(context.Background(), opts)
	if err != nil {
		log.Fatalf("Failed to get server IP: %v", err)
	}
	ServerIP = ipRegex.FindString(out.Output)
}

// Online gets the number of players online on the server.
func Online(ctx context.Context, port uint16) (int, error) {
	resp, err := status.Modern(ctx, ServerIP, port)
	if err != nil {
		return 0, fmt.Errorf("failed to get server status: %v", err)
	}
	return int(*resp.Players.Online), nil
}
