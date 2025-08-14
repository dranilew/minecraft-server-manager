//go:build linux

package monitor

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"syscall"
)

func listen(ctx context.Context, pipe string) (net.Listener, error) {
	runtime.LockOSThread()
	oldMask := syscall.Umask(777 - pipeFileMode)
	var lc net.ListenConfig
	srv, err := lc.Listen(ctx, "unix", pipe)
	syscall.Umask(oldMask)
	runtime.UnlockOSThread()
	if err != nil {
		return nil, fmt.Errorf("failed to start listener on %q: %w", pipe, err)
	}
	return srv, nil
}
