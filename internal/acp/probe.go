package acp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/execabs"

	"github.com/compozy/agh/internal/diagnostics"
)

const (
	ProbeStatusOK       = "ok"
	ProbeStatusMissing  = "missing"
	ProbeStatusInvalid  = "invalid"
	ProbeStatusCanceled = "canceled"
	ProbeStatusTimeout  = "timeout"

	defaultProbeTimeout = 2 * time.Second
)

// ProbeTarget identifies one configured ACP-compatible agent/provider command
// that should be checked before operators need to start a session.
type ProbeTarget struct {
	AgentName string
	Provider  string
	Command   string
}

// ProbeResult is the structured health output for one downstream agent command.
type ProbeResult struct {
	AgentName  string    `json:"agent_name,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	Command    string    `json:"command,omitempty"`
	Executable string    `json:"executable,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	DurationMS int64     `json:"duration_ms"`
}

// ProbeLookup resolves an executable name. Tests can inject a blocking lookup
// to validate timeout/cancellation without sleeping in production code.
type ProbeLookup func(context.Context, string) (string, error)

// ProbeOptions configures command probing.
type ProbeOptions struct {
	Timeout time.Duration
	Now     func() time.Time
	Lookup  ProbeLookup
}

// ProbeTargets checks every target with bounded timeout/cancellation behavior.
func ProbeTargets(ctx context.Context, targets []ProbeTarget, opts ProbeOptions) []ProbeResult {
	if len(targets) == 0 {
		return nil
	}
	results := make([]ProbeResult, 0, len(targets))
	for _, target := range targets {
		results = append(results, ProbeTargetCommand(ctx, target, opts))
	}
	return results
}

// ProbeTargetCommand checks one target command by resolving its executable.
func ProbeTargetCommand(ctx context.Context, target ProbeTarget, opts ProbeOptions) (result ProbeResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := opts.Now
	if now == nil {
		now = timeNowUTC
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}
	lookup := opts.Lookup
	if lookup == nil {
		lookup = defaultProbeLookup
	}

	start := time.Now()
	checkedAt := now().UTC()
	rawCommand := strings.TrimSpace(target.Command)
	result = ProbeResult{
		AgentName: strings.TrimSpace(target.AgentName),
		Provider:  strings.TrimSpace(target.Provider),
		Command:   diagnostics.RedactAndBound(rawCommand, maxFailureSummaryBytes),
		CheckedAt: checkedAt,
	}
	defer func() {
		result.DurationMS = max(time.Since(start).Milliseconds(), 0)
	}()

	if err := ctx.Err(); err != nil {
		result.Status = probeStatusFromContext(err)
		result.Error = diagnostics.RedactAndBound(err.Error(), maxFailureSummaryBytes)
		return result
	}
	command, _, err := parseCommandString(rawCommand)
	if err != nil {
		result.Status = ProbeStatusInvalid
		result.Error = diagnostics.RedactAndBound(err.Error(), maxFailureSummaryBytes)
		return result
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	executable, err := lookup(probeCtx, command)
	if err != nil {
		if ctxErr := probeCtx.Err(); ctxErr != nil {
			result.Status = probeStatusFromContext(ctxErr)
			result.Error = diagnostics.RedactAndBound(ctxErr.Error(), maxFailureSummaryBytes)
			return result
		}
		result.Status = ProbeStatusMissing
		result.Error = diagnostics.RedactAndBound(
			fmt.Sprintf("resolve executable %q: %v", command, err),
			maxFailureSummaryBytes,
		)
		return result
	}
	result.Status = ProbeStatusOK
	result.Executable = strings.TrimSpace(executable)
	return result
}

func defaultProbeLookup(ctx context.Context, command string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return execabs.LookPath(command)
}

func probeStatusFromContext(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return ProbeStatusTimeout
	}
	return ProbeStatusCanceled
}
