package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("window manager context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("window manager context: %w", err)
	}
	return nil
}

func validateWorkspaceID(workspaceID WorkspaceID) error {
	if strings.TrimSpace(string(workspaceID)) == "" {
		return ErrWorkspaceNotFound
	}
	return nil
}
