//go:build darwin || linux

package procutil

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

func ReapRemoteConnectors(ctx context.Context, home, directory string) error {
	snapshot, err := ScanProcesses(ctx)
	if err != nil {
		return err
	}
	for _, p := range snapshot.Processes {
		if !p.RemoteConnector || p.Home != home || p.Zombie || !filepath.IsAbs(p.ConnectorExecutable) {
			continue
		}
		relative, err := filepath.Rel(directory, p.ConnectorExecutable)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		members := []ScannedProcess{p}
		if err := signalScannedMembers(members, syscall.SIGTERM); err != nil {
			return fmt.Errorf("stop orphaned connector %d: %w", p.PID, err)
		}
		result := awaitGracefulScannedExit(ctx, members, ownedProcessGrace)
		if result.Err != nil || result.Outcome == terminationUnproven {
			return fmt.Errorf("stop orphaned connector %d: %w", p.PID,
				errors.Join(result.Err, errors.New("could not verify connector exit")))
		}
	}
	return nil
}
