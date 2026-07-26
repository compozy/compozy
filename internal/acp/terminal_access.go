package acp

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"unicode/utf8"

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
	host := p.toolHostOrDefault()
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

func mergeCommandEnv(base []string, variables []acpsdk.EnvVariable) []string {
	merged := make(map[string]string, len(base)+len(variables))
	order := make([]string, 0, len(base)+len(variables))

	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		if _, exists := merged[name]; !exists {
			order = append(order, name)
		}
		merged[name] = parts[1]
	}

	for _, variable := range variables {
		if _, exists := merged[variable.Name]; !exists {
			order = append(order, variable.Name)
		}
		merged[variable.Name] = variable.Value
	}

	result := make([]string, 0, len(order))
	for _, name := range order {
		result = append(result, name+"="+merged[name])
	}
	return result
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

func appendTerminalOutputWindow(dst []byte, src []byte, limit int) ([]byte, bool) {
	if limit <= 0 {
		return nil, len(dst) > 0 || len(src) > 0
	}
	if len(src) == 0 {
		if len(dst) <= limit {
			return dst, false
		}
		return trimUTF8LeadingBytes(dst[len(dst)-limit:]), true
	}
	if len(dst)+len(src) <= limit {
		return append(dst, src...), false
	}

	var out []byte
	if cap(dst) == limit {
		out = dst[:0]
	} else {
		out = make([]byte, 0, limit)
	}

	if len(src) >= limit {
		out = append(out, src[len(src)-limit:]...)
		return trimUTF8LeadingBytes(out), true
	}

	keepFromDst := limit - len(src)
	if len(dst) > keepFromDst {
		dst = dst[len(dst)-keepFromDst:]
	}
	out = append(out, dst...)
	out = append(out, src...)
	return trimUTF8LeadingBytes(out), true
}

func trimUTF8LeadingBytes(data []byte) []byte {
	trim := 0
	for trim < len(data) && !isValidUTF8LeadingByte(data[trim]) {
		trim++
	}
	if trim == 0 {
		return data
	}
	copy(data, data[trim:])
	return data[:len(data)-trim]
}

func isValidUTF8LeadingByte(b byte) bool {
	return b < utf8.RuneSelf || (b >= 0xC2 && b <= 0xF4)
}

func watchTerminalShutdown(ctx context.Context, terminalDone <-chan struct{}, onShutdown func()) <-chan struct{} {
	watcherDone := make(chan struct{})
	if ctx == nil {
		close(watcherDone)
		return watcherDone
	}

	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			if onShutdown != nil {
				onShutdown()
			}
		case <-terminalDone:
		}
	}()

	return watcherDone
}
