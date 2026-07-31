package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authorizeMCPServer(
	cmd *cobra.Command,
	client MCPSettingsClient,
	target SettingsMCPAuthTarget,
	manual bool,
	timeout time.Duration,
	approvedScopes []string,
	approveScopeEscalation bool,
) (SettingsMCPAuthStatusRecord, error) {
	authCtx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	baseline, err := loadMCPAuthStatus(authCtx, client, target)
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	mode := contract.SettingsMCPAuthBeginModeAutomatic
	if manual {
		mode = contract.SettingsMCPAuthBeginModeManual
	}
	begin, err := client.BeginSettingsMCPAuth(authCtx, target, SettingsMCPAuthBeginRequest{
		Mode:                   mode,
		ApprovedScopes:         append([]string(nil), approvedScopes...),
		ApproveScopeEscalation: approveScopeEscalation,
	})
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	if _, err := fmt.Fprintf(
		cmd.ErrOrStderr(),
		"Open this URL to authorize %s:\n%s\n",
		target.Name,
		begin.AuthorizationURL,
	); err != nil {
		return SettingsMCPAuthStatusRecord{}, fmt.Errorf("cli: write MCP authorization instructions: %w", err)
	}
	exchangeCtx := authCtx
	exchangeCancel := func() {}
	if !begin.ExpiresAt.IsZero() {
		exchangeCtx, exchangeCancel = context.WithDeadline(authCtx, begin.ExpiresAt)
	}
	defer exchangeCancel()
	if manual {
		return exchangeManualMCPAuth(exchangeCtx, cmd, client, target)
	}
	return waitForMCPAuth(authCtx, client, target, baseline, begin.ExpiresAt, timeout)
}

func exchangeManualMCPAuth(
	ctx context.Context,
	cmd *cobra.Command,
	client MCPSettingsClient,
	target SettingsMCPAuthTarget,
) (SettingsMCPAuthStatusRecord, error) {
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Paste the full authorization redirect URL:"); err != nil {
		return SettingsMCPAuthStatusRecord{}, fmt.Errorf("cli: write MCP authorization prompt: %w", err)
	}
	input, err := readManualMCPAuthInputContext(ctx, cmd.InOrStdin(), cmd.ErrOrStderr())
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	request, err := manualMCPAuthExchangeRequest(input)
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	status, err := client.ExchangeSettingsMCPAuth(ctx, target, request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return SettingsMCPAuthStatusRecord{}, mcpAuthorizationTimeoutError(context.DeadlineExceeded)
		}
		return SettingsMCPAuthStatusRecord{}, err
	}
	if !confirmedMCPAuthStatus(status) {
		return SettingsMCPAuthStatusRecord{}, errors.New(
			"cli: MCP authorization did not produce a confirmed credential",
		)
	}
	return status, nil
}

func readManualMCPAuthInputContext(ctx context.Context, input io.Reader, output io.Writer) (string, error) {
	return readManualMCPAuthInputContextWithTerminal(
		ctx,
		input,
		output,
		supportBundleInputIsTerminal,
		term.ReadPassword,
	)
}

func readManualMCPAuthInputContextWithTerminal(
	ctx context.Context,
	input io.Reader,
	output io.Writer,
	isTerminal func(io.Reader) bool,
	readPassword func(int) ([]byte, error),
) (string, error) {
	inputIsTerminal := isTerminal(input)
	readInput := func() (string, error) {
		return readManualMCPAuthInputWithTerminal(
			input,
			output,
			func(io.Reader) bool { return inputIsTerminal },
			readPassword,
		)
	}
	if inputIsTerminal {
		value, err := readInput()
		if err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", mcpAuthorizationTimeoutError(err)
		}
		return value, nil
	}

	file, ok := input.(*os.File)
	if !ok {
		value, err := readInput()
		if err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
			return "", mcpAuthorizationTimeoutError(err)
		}
		return value, nil
	}

	type readResult struct {
		value string
		err   error
	}
	results := make(chan readResult, 1)
	go func() {
		value, err := readInput()
		results <- readResult{value: value, err: err}
	}()
	select {
	case result := <-results:
		if err := ctx.Err(); err != nil {
			return "", mcpAuthorizationTimeoutError(err)
		}
		return result.value, result.err
	case <-ctx.Done():
		timeoutErr := mcpAuthorizationTimeoutError(ctx.Err())
		if err := file.Close(); err != nil {
			return "", errors.Join(timeoutErr, fmt.Errorf("cli: cancel MCP authorization input: %w", err))
		}
		<-results
		return "", timeoutErr
	}
}

func mcpAuthorizationTimeoutError(err error) error {
	return fmt.Errorf("cli: MCP authorization timed out: %w", err)
}

func readManualMCPAuthInputWithTerminal(
	input io.Reader,
	output io.Writer,
	isTerminal func(io.Reader) bool,
	readPassword func(int) ([]byte, error),
) (string, error) {
	if isTerminal(input) {
		file, ok := input.(*os.File)
		if !ok {
			return "", errors.New("cli: terminal MCP authorization input must be an operating-system file")
		}
		value, readErr := readPassword(int(file.Fd()))
		_, newlineErr := fmt.Fprintln(output)
		if readErr != nil {
			wrappedReadErr := fmt.Errorf("cli: read hidden MCP authorization response: %w", readErr)
			if newlineErr != nil {
				return "", errors.Join(
					wrappedReadErr,
					fmt.Errorf("cli: write hidden-input newline: %w", newlineErr),
				)
			}
			return "", wrappedReadErr
		}
		if newlineErr != nil {
			return "", fmt.Errorf("cli: write hidden-input newline: %w", newlineErr)
		}
		return string(value), nil
	}

	value, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("cli: read MCP authorization response: %w", err)
	}
	return value, nil
}

func manualMCPAuthExchangeRequest(input string) (SettingsMCPAuthExchangeRequest, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return SettingsMCPAuthExchangeRequest{}, errors.New("cli: authorization redirect URL is required")
	}
	if !strings.Contains(input, "://") {
		return SettingsMCPAuthExchangeRequest{}, errors.New("cli: authorization redirect URL must be absolute")
	}
	return SettingsMCPAuthExchangeRequest{RedirectURL: input}, nil
}

func waitForMCPAuth(
	ctx context.Context,
	client MCPSettingsClient,
	target SettingsMCPAuthTarget,
	baseline SettingsMCPAuthStatusRecord,
	expiresAt time.Time,
	timeout time.Duration,
) (SettingsMCPAuthStatusRecord, error) {
	deadline := time.Now().Add(timeout)
	if !expiresAt.IsZero() && expiresAt.Before(deadline) {
		deadline = expiresAt
	}
	waitCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	ticker := time.NewTicker(mcpAuthPollInterval)
	defer ticker.Stop()
	for {
		status, err := loadMCPAuthStatus(waitCtx, client, target)
		if err != nil {
			return SettingsMCPAuthStatusRecord{}, err
		}
		if completedMCPAuthStatus(baseline, status) {
			return status, nil
		}
		select {
		case <-waitCtx.Done():
			return SettingsMCPAuthStatusRecord{}, mcpAuthorizationTimeoutError(waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func loadMCPAuthStatus(
	ctx context.Context,
	client MCPSettingsClient,
	target SettingsMCPAuthTarget,
) (SettingsMCPAuthStatusRecord, error) {
	status, err := client.GetSettingsMCPAuthStatus(ctx, target)
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	if status.Status == string(mcpauth.StatusUnconfigured) {
		return SettingsMCPAuthStatusRecord{}, fmt.Errorf(
			"cli: remote MCP server %q in %s scope does not configure OAuth",
			target.Name,
			target.Scope,
		)
	}
	return status, nil
}

func mcpAuthStatuses(
	servers []contract.SettingsMCPServerItemPayload,
	opts mcpAuthCommandOptions,
) []SettingsMCPAuthStatusRecord {
	statuses := make([]SettingsMCPAuthStatusRecord, 0, len(servers))
	for _, server := range servers {
		if string(server.Scope) != strings.TrimSpace(opts.scope) ||
			server.WorkspaceID != strings.TrimSpace(opts.workspaceID) {
			continue
		}
		if server.AuthStatus == nil {
			continue
		}
		statuses = append(statuses, *server.AuthStatus)
	}
	return statuses
}

func confirmedMCPAuthStatus(status SettingsMCPAuthStatusRecord) bool {
	return status.Status == string(mcpauth.StatusAuthenticated) && status.TokenPresent
}

func completedMCPAuthStatus(
	baseline SettingsMCPAuthStatusRecord,
	status SettingsMCPAuthStatusRecord,
) bool {
	if !confirmedMCPAuthStatus(status) {
		return false
	}
	if !confirmedMCPAuthStatus(baseline) {
		return true
	}
	if status.UpdatedAt == nil {
		return false
	}
	if baseline.UpdatedAt == nil {
		return true
	}
	return status.UpdatedAt.After(*baseline.UpdatedAt)
}
