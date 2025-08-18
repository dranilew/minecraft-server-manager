//go:build windows

package logger

import (
	"fmt"
	"io"
)

func initPlatformLogger(string, []io.Writer) error {
	return fmt.Errorf("not implemented on windows")
}
