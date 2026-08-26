package acp

import (
	"context"
	"errors"
	"fmt"

	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	shellquote "github.com/kballard/go-shellquote"
)

func terminalArgv(request acpsdk.CreateTerminalRequest) ([]string, error) {
	command := strings.TrimSpace(request.Command)
	if command == "" {
		return nil, errors.New("acp: terminal command is required")
	}

	argv, err := shellquote.Split(command)
	if err != nil {
		return nil, fmt.Errorf("acp: parse terminal command %q: %w", request.Command, err)
	}
	if len(argv) == 0 {
		return nil, errors.New("acp: terminal command is required")
	}
	return append(argv, request.Args...), nil
}

func isAllowedNetworkTerminalArgv(argv []string) bool {
	if len(argv) < 3 || argv[0] != defaultClientName || argv[1] != networkCommandName {
		return false
	}

	switch argv[2] {
	case "send", "peers", "channels", terminalStatusKey, "inbox", "threads", "directs", "work":
		return true
	default:
		return false
	}
}

func (p *AgentProcess) ensureNetworkTurnTerminalAccess(id string, requireSameTurn bool) error {
	if !p.isNetworkTurn() {
		return nil
	}

	ownership, err := p.lookupTerminalOwnership(id)
	if err != nil {
		return err
	}
	if !ownership.networkOwned {
		return ErrToolBlockedForNetworkTurn
	}
	if requireSameTurn && strings.TrimSpace(ownership.ownerTurnID) != p.activeTurnID() {
		return ErrToolBlockedForNetworkTurn
	}
	return nil
}

func (p *AgentProcess) lookupTerminalOwnership(id string) (terminalOwnership, error) {
	host, err := p.toolHostOrDefault()
	if err != nil {
		return terminalOwnership{}, err
	}
	if localHost, ok := host.(*localToolHost); ok {
		return localHost.terminalOwnership(id)
	}

	p.terminalOwnershipMu.RLock()
	ownership, ok := p.terminalOwnership[id]
	p.terminalOwnershipMu.RUnlock()
	if !ok {
		return terminalOwnership{}, ErrToolBlockedForNetworkTurn
	}
	return ownership, nil
}

func cloneNonEmptyStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneNonEmptyEnvSlice(values []acpsdk.EnvVariable) []acpsdk.EnvVariable {
	if len(values) == 0 {
		return nil
	}
	return append([]acpsdk.EnvVariable(nil), values...)
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func withoutCancelPreservingDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(ctx)
	deadline, ok := ctx.Deadline()
	if !ok {
		return detached, func() {}
	}
	return context.WithDeadline(detached, deadline)
}
