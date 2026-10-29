//go:build windows

package procutil

import (
	"context"
	"fmt"
)

// Windows accounts record no POSIX login shell; callers keep their native shell order.
func lookupAccountShell(context.Context, int) (string, error) {
	return "", fmt.Errorf("%w: windows accounts have no login shell", ErrAccountShellUnavailable)
}
