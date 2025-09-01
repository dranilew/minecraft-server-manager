package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/dranilew/minecraft-server-manager/src/lib/logger"
)

// Response represents a response written on the pipe.
type Response struct {
	// Status indicates the status code (like 404).
	Status int
	// Message is the error message.
	Message string
}

var (
	// ConnError is returned for errors from the underlying communicaton protocol.
	ConnError = Response{
		Status:  101,
		Message: "Connection error",
	}
	// TimeoutError is returned when the timeout period elapses before a valid JSON is received.
	TimeoutError = Response{
		Status:  102,
		Message: "Connection timeout before reading valid request",
	}
	// InternalError is returned if an unknown error occurred while reading or parsing requests.
	InternalError = Response{
		Status:  400,
		Message: "The command server encountered an internal error trying to response to your request",
	}
	// CmdNotFoundError is returned if an unknown command is passed in.
	CmdNotFoundError = Response{
		Status:  404,
		Message: "The requested command does not exist or is not supported",
	}
)

// NewExecutionError returned a new ExecutionError response.
// Msg is used in the case of a success.
func NewExecutionError(msg string, err error) Response {
	if err == nil {
		return Response{Status: 0, Message: msg}
	}
	return Response{
		Status:  103,
		Message: fmt.Sprintf("Failed to execute command: %v", err),
	}
}

func (r Response) Error() error {
	if r.Status == 0 {
		if r.Message != "" {
			logger.Printf("%s\n", r.Message)
		}
		return nil
	}
	return fmt.Errorf("exit status %d: %s", r.Status, r.Message)
}

// SendCommand sends a command to the command socket.
func SendCommand(ctx context.Context, req []byte) error {
	// Connect to the command socket.
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", *monitorPipe)
	if err != nil {
		return fmt.Errorf("failed to dial pipe: %v", err)
	}

	// Set a timeout for the connection.
	duration, err := time.ParseDuration(*timeoutString)
	if err != nil {
		return fmt.Errorf("invalid duration string %s: %v", *timeoutString, err)
	}
	if err := conn.SetDeadline(time.Now().Add(duration)); err != nil {
		return fmt.Errorf("failed to set deadline for connection: %v", err)
	}

	// Write the request to the pipe.
	i, err := conn.Write(req)
	if err != nil || i != len(req) {
		return ConnError.Error()
	}

	// Read the response.
	data, err := io.ReadAll(conn)
	if err != nil {
		return ConnError.Error()
	}
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}
	return resp.Error()
}
