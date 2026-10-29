//go:build !darwin && !windows

package procutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/execabs"
)

const passwdDatabasePath = "/etc/passwd"

// lookupAccountShell prefers NSS through getent and falls back to the local passwd file.
func lookupAccountShell(ctx context.Context, uid int) (string, error) {
	var getentErr error
	if getent, err := execabs.LookPath("getent"); err == nil {
		output, runErr := execabs.CommandContext(ctx, getent, "passwd", strconv.Itoa(uid)).Output()
		if runErr == nil {
			return parsePasswdShell(output, uid)
		}
		getentErr = fmt.Errorf("query getent passwd: %w", runErr)
	}
	passwd, err := os.ReadFile(passwdDatabasePath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrAccountShellUnavailable, errors.Join(getentErr, err))
	}
	return parsePasswdShell(passwd, uid)
}
