//go:build windows

package monitor

import (
	"context"
	"fmt"
	"net"
)

func listen(context.Context, string) (net.Listener, error) {
	return nil, fmt.Errorf("Not implemented on windows")
}
