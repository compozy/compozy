package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// NotifyExtension sends one fire-and-forget `_`-prefixed notification over the live agent connection.
func (d *Driver) NotifyExtension(
	ctx context.Context,
	proc *AgentProcess,
	method string,
	params any,
) error {
	if ctx == nil {
		return errors.New("acp: notify extension context is required")
	}
	if proc == nil {
		return errors.New("acp: agent process is required")
	}
	if proc.conn == nil {
		return errProcessConnectionUninitialized
	}
	if !strings.HasPrefix(method, "_") {
		return fmt.Errorf("acp: extension method must start with '_': %q", method)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return proc.conn.SendNotification(ctx, method, params)
}
