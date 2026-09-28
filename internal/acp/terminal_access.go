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
	if ok {
		return context.WithDeadline(detached, deadline)
	}
	return context.WithTimeout(detached, defaultStopTimeout)
}
