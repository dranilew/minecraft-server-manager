//go:build windows

package logger

import (
	"fmt"
)

func initPlatformLogger(string) error {
	return fmt.Errorf("not implemented on windows")
}
