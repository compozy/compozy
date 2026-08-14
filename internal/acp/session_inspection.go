package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// SessionInspectionRequest describes a short-lived ACP session used only to read advertised options.
type SessionInspectionRequest struct {
	AgentName string
	Command   string
	Cwd       string
	Env       []string
}

// InspectSessionConfigOptions creates a short-lived ACP session and returns its advertised config options.
func InspectSessionConfigOptions(
	ctx context.Context,
	req SessionInspectionRequest,
) (_ []SessionConfigOption, err error) {
	if ctx == nil {
		return nil, errors.New("acp: session inspection context is required")
	}
	driver := New()
	proc, err := driver.Start(ctx, StartOpts{
		AgentName:  strings.TrimSpace(req.AgentName),
		Command:    strings.TrimSpace(req.Command),
		Cwd:        strings.TrimSpace(req.Cwd),
		Env:        append([]string(nil), req.Env...),
		Inspection: true,
	})
	if err != nil {
		return nil, fmt.Errorf("acp: inspect session config options: %w", err)
	}
	defer func() {
		// Stop uses defaultStopTimeout for cooperative shutdown before signal escalation.
		// Give that full lifecycle a separate, bounded budget instead of reporting the
		// normal escalation path as a failed inspection.
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*defaultStopTimeout)
		defer cancel()
		if stopErr := driver.stop(stopCtx, proc, true); stopErr != nil {
			err = errors.Join(err, fmt.Errorf("acp: stop inspected session: %w", stopErr))
		}
	}()

	return CloneSessionConfigOptions(proc.CapsSnapshot().ConfigOptions), nil
}
