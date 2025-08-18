//go:build windows

package logger

import (
	"fmt"
	"io"
)

// Services are not supported on Windows.
func initPlatformLogger(string, []io.Writer) error {
	return fmt.Errorf("not implemented on windows")
}
