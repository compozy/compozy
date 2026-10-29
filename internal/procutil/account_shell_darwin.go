//go:build darwin

package procutil

import (
	"context"
	"fmt"
	"strconv"

	"golang.org/x/sys/execabs"
)

const directoryServicesCacheTool = "/usr/bin/dscacheutil"

// lookupAccountShell asks directory services, which also covers network and managed accounts.
func lookupAccountShell(ctx context.Context, uid int) (string, error) {
	//nolint:gosec // Fixed absolute tool path; the only argument is this process's own numeric uid.
	output, err := execabs.CommandContext(
		ctx, directoryServicesCacheTool, "-q", "user", "-a", "uid", strconv.Itoa(uid),
	).Output()
	if err != nil {
		return "", fmt.Errorf("%w: query directory services: %w", ErrAccountShellUnavailable, err)
	}
	return parseDirectoryServicesShell(output, uid)
}
