package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/backup"
	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
)

var (
	// enableTLS indicates whether to enable the TLS listener.
	enableTLS = flag.Bool("enable-tls", false, "Enable TLS listener for external command monitoring.")
	// tlsTimeout is the timeout for TLS connections.
	tlsTimeout = flag.String("tls-timeout", "5m", "Timeout for TLS connections")
	// tlsPort is the port to use for TLS connections.
	tlsPort = flag.String("tls-port", "4040", "Port to use for TLS connections.")
	// tlsCert is the location of the certificate PEM file.
	tlsCert = flag.String("tls-cert", "cert.pem", "Location of certificate PEM file. This is relative to <modpack directory>/certificates.")
	// keyFile is the location of the key PEM file.
	tlsKey = flag.String("tls-key", "key.pem", "Location of key PEM file. This is relative to <modpack directory>/certificates.")
	// tlsBucket is the bucket to which a backup is uploaded when the command is triggered.
	tlsBucket = flag.String("tls-bucket", "", "GCloud storage location to store backups when backup command is sent.")
	// tlsMonitor is the currently active TLS socket monitoring.
	tlsMonitor = &TLSMonitor{}
)

// TLSMonitor is a TLS server hosted on the VM.
type TLSMonitor struct {
	srv *TLSServer
}

// TLSServer is the TLS server
type TLSServer struct {
	port    uint16
	timeout time.Duration
	lc      net.Listener
	monitor *TLSMonitor
}

func SetupTLS(ctx context.Context) error {
	if !*enableTLS {
		logger.Printf("Skipping TLS setup, TLS is disabled")
		return nil
	}

	t, err := time.ParseDuration(*tlsTimeout)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(*tlsPort)
	if err != nil {
		return err
	}

	tlsMonitor.srv = &TLSServer{
		port:    uint16(port),
		timeout: t,
		monitor: tlsMonitor,
	}
	if err := tlsMonitor.srv.start(ctx); err != nil {
		return err
	}
	return nil
}

// start starts a TLS server.
func (srv *TLSServer) start(ctx context.Context) error {
	if srv.lc != nil {
		return fmt.Errorf("already listening for TLS on port %d", srv.port)
	}

	cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
	if err != nil {
		return fmt.Errorf("failed to load x509 key pair: %w", err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	logger.Debugf("Listening on port %d\n", srv.port)
	lc, err := tls.Listen("tcp", fmt.Sprintf(":%d", srv.port), config)
	if err != nil {
		return fmt.Errorf("failed to listen for TPC on port %d: %v", srv.port, err)
	}
	srv.lc = lc

	go func() {
		defer lc.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := lc.Accept()
				if err != nil {
					logger.Printf("Failed to accept TLS connection: %v", err)
				}

				message, ok := readFromConn(conn)
				if !ok {
					return
				}
				logger.Printf("Received command request: %s", string(message))
				exeErr := NewExecutionError(handleTLSMessage(message))
				b, err := json.Marshal(exeErr)
				if err != nil {
					logger.Printf("Failed to marshal execution error: %v", err)
				}
				if n, err := conn.Write(b); err != nil || n != len(b) {
					logger.Printf("Failed to write to connection on TLS: %v", err)
				}
			}
		}
	}()
	return nil
}

// handleTLSMessage handles TLS messages. This is different from the internal
// command monitor's handler because we want to limit the information that
// TLS connections have to the servers.
func handleTLSMessage(req []byte) (string, error) {
	ctx := context.Background()
	reqString := string(req)
	fields := strings.Fields(reqString)
	switch fields[0] {
	case "backup":
		switch fields[1] {
		case "create":
			servers := fields[2:]
			return fmt.Sprintf("Successfully created backups for %v", servers), backup.Create(ctx, true, *tlsBucket, servers...)
		default:
			return "", fmt.Errorf("unknown server request: %v", fields[1])
		}
	case "server":
		switch fields[1] {
		case "info":
			var buf bytes.Buffer
			server.GetInfo(&buf)
			return buf.String(), nil
		}
	default:
		return "", fmt.Errorf("unknown request: %v", fields[0])
	}
	return "", nil
}
