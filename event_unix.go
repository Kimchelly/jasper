//go:build darwin || linux || freebsd

package jasper

import (
	"context"
)

// SignalEvent is a no-op on Unix-based systems.
func SignalEvent(context.Context, string) error {
	return nil
}
