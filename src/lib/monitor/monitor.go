// Package monitor is for accepting and receiving command request.
package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
	"github.com/dranilew/minecraft-server-manager/src/lib/server"
)

const (
	pipeFileMode = 0770
)

var (
	monitor       = &Monitor{}
	timeoutString = flag.String("server-timeout", "5m", "The default timeout for command monitoring. This should be a Golang-parseable time duration string.")
	pipe          = flag.String("pipe", "/etc/minecraft/manager", "The pipe location for monitoring.")
)

// Server is the server to which commands are posted.
type Server struct {
	pipe    string
	timeout time.Duration
	srv     net.Listener
	monitor *Monitor
}

// Monitor is the pipe monitor, which listens for new commands and executes
// actions according to what is received through the pipe.
type Monitor struct {
	srv *Server
}

func init() {
	flag.Parse()
}

// Setup starts an internally managed command server.
func Setup(ctx context.Context) error {
	timeout, err := time.ParseDuration(*timeoutString)
	if err != nil {
		logger.Fatalf("Invalid timeout string %s", *timeoutString)
	}
	monitor.srv = &Server{
		pipe:    *pipe,
		timeout: timeout,
		monitor: monitor,
	}
	if err := monitor.srv.start(ctx); err != nil {
		return fmt.Errorf("Failed to start monitor server: %v", err)
	}
	return nil
}

// Close closes the listener.
func Close(context.Context) {
	if monitor.srv != nil {
		if err := monitor.srv.close(); err != nil {
			logger.Printf("error closing monitor: %v", err)
		}
		monitor.srv = nil
	}
}

// readFromConn reads data from a connection.
func readFromConn(conn net.Conn) ([]byte, bool) {
	b := make([]byte, 1024)
	n, err := conn.Read(b)
	if err == nil {
		return b[:n], true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		if e, err := json.Marshal(TimeoutError); err == nil {
			conn.Write(e)
			return nil, false
		}
	} else {
		if e, err := json.Marshal(ConnError); err == nil {
			conn.Write(e)
			return nil, false
		}
	}
	if e, err := json.Marshal(InternalError); err == nil {
		conn.Write(e)
	}
	return nil, false
}

// start starts a listener on the given pipe.
func (s *Server) start(ctx context.Context) error {
	if s.srv != nil {
		return fmt.Errorf("already listening on pipe %q", s.pipe)
	}

	if _, err := os.Stat(*pipe); err == nil {
		if err := os.Remove(*pipe); err != nil {
			return fmt.Errorf("error cleaning up previous pipe: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.pipe), pipeFileMode); err != nil {
		return fmt.Errorf("failed to create directories for listener: %v", err)
	}

	// Create the listener.
	srv, err := listen(ctx, *pipe)
	if err != nil {
		return err
	}
	s.srv = srv

	// Start accepting and handling connections in a separate thread.
	go func() {
		defer srv.Close()
		for {
			if ctx.Err() != nil {
				return
			}
			// Accept and wait for connections.
			conn, err := srv.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}
				logger.Printf("error on connection to pipe %s: %v", s.pipe, err)
				continue
			}
			// Handle the connection.
			go func(conn net.Conn) {
				defer conn.Close()

				deadline := time.Now().Add(s.timeout)
				if err := conn.SetDeadline(deadline); err != nil {
					logger.Printf("could not set deadline on command request: %v", err)
					return
				}

				message, ok := readFromConn(conn)
				if !ok {
					return
				}
				logger.Printf("Received command request: %s", string(message))
				exeErr := NewExecutionError(handleMessage(message))
				b, err := json.Marshal(exeErr)
				if err != nil {
					logger.Printf("Failed to marshal execution error: %v", err)
				}
				if n, err := conn.Write(b); err != nil || n != len(b) {
					logger.Printf("Failed to write to connection on pipe %q: %v", s.pipe, err)
				}
			}(conn)
		}
	}()
	return nil
}

// Close signals the server to stop listening for commands and stop waiting on listen.
func (s *Server) close() error {
	if s.srv != nil {
		return s.srv.Close()
	}
	return nil
}

// handleMessage handles the request received from the connection.
func handleMessage(req []byte) error {
	ctx := context.Background()
	reqString := string(req)
	fields := strings.Fields(reqString)
	switch fields[0] {
	case "server":
		switch fields[1] {
		case "stop":
			return server.Stop(ctx, fields[2:]...)
		case "start":
			return server.Start(ctx, fields[2:]...)
		case "restart":
			return server.Restart(ctx, fields[2:]...)
		default:
			return fmt.Errorf("unknown server request: %v", fields[1])
		}
	default:
		return fmt.Errorf("unknown request: %v", fields[0])
	}
}
