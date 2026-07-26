package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	mcpAuthAuthKey = "auth"
	mcpAuthMCPKey  = "mcp"
)

const (
	defaultMCPAuthLoginTimeout = 5 * time.Minute
	mcpAuthPollInterval        = 100 * time.Millisecond
)

type mcpAuthCommandOptions struct {
	scope       string
	workspaceID string
	manual      bool
	timeout     time.Duration
}

func newMCPAuthCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   mcpAuthAuthKey,
		Short: "Authenticate remote MCP servers through the daemon",
	}
	cmd.AddCommand(newMCPAuthLoginCommand(deps))
	cmd.AddCommand(newMCPAuthStatusCommand(deps))
	cmd.AddCommand(newMCPAuthLogoutCommand(deps))
	return cmd
}

func newMCPAuthorizeCommand(deps commandDeps) *cobra.Command {
	return newMCPAuthorizationCommand(
		deps,
		"authorize <server>",
		"Authorize a remote MCP server through the daemon",
	)
}

func newMCPAuthLoginCommand(deps commandDeps) *cobra.Command {
	return newMCPAuthorizationCommand(
		deps,
		"login <server>",
		"Authorize a remote MCP server through the daemon",
	)
}

func newMCPAuthorizationCommand(deps commandDeps, use string, short string) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsWorkspaceScopeGlobal)}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := opts.target(args[0])
			if err != nil {
				return err
			}
			if opts.timeout <= 0 {
				return errors.New("cli: MCP authorization timeout must be positive")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := authorizeMCPServer(cmd, client, target, opts.manual, opts.timeout)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, mcpAuthStatusBundle(status))
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.manual, "manual", false, "Paste an authorization code or redirect URL")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", defaultMCPAuthLoginTimeout, "Authorization timeout")
	return cmd
}

func newMCPAuthStatusCommand(deps commandDeps) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsWorkspaceScopeGlobal)}
	cmd := &cobra.Command{
		Use:   "status [server]",
		Short: "Show redacted remote MCP auth status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validateScope(); err != nil {
				return err
			}
			scope := contract.SettingsWorkspaceScopeKind(strings.TrimSpace(opts.scope))
			workspaceID := strings.TrimSpace(opts.workspaceID)
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ListSettingsMCPServers(
				cmd.Context(),
				scope,
				workspaceID,
			)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = strings.TrimSpace(args[0])
			}
			statuses, found := mcpAuthStatuses(response.MCPServers, opts, name)
			if name != "" && !found {
				return fmt.Errorf("cli: remote MCP auth server %q not found in %s scope", name, opts.scope)
			}
			return writeCommandOutput(cmd, mcpAuthStatusListBundle(statuses))
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}

func newMCPAuthLogoutCommand(deps commandDeps) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsWorkspaceScopeGlobal)}
	cmd := &cobra.Command{
		Use:   "logout <server>",
		Short: "Revoke or delete remote MCP auth tokens through the daemon",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := opts.target(args[0])
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.LogoutSettingsMCPAuth(cmd.Context(), target)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, mcpAuthStatusBundle(status))
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}

func addMCPAuthTargetFlags(cmd *cobra.Command, opts *mcpAuthCommandOptions) {
	cmd.Flags().StringVar(&opts.scope, "scope", opts.scope, "MCP server scope: global or workspace")
	cmd.Flags().StringVar(&opts.workspaceID, "workspace", "", "Workspace ID for workspace scope")
}

func (o mcpAuthCommandOptions) target(name string) (SettingsMCPAuthTarget, error) {
	if err := o.validateScope(); err != nil {
		return SettingsMCPAuthTarget{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return SettingsMCPAuthTarget{}, errors.New("cli: MCP server name is required")
	}
	if strings.Contains(name, "/") {
		return SettingsMCPAuthTarget{}, errors.New("cli: MCP server name cannot contain a slash")
	}
	return SettingsMCPAuthTarget{
		Name:        name,
		Scope:       contract.SettingsWorkspaceScopeKind(strings.TrimSpace(o.scope)),
		WorkspaceID: strings.TrimSpace(o.workspaceID),
	}, nil
}

func (o mcpAuthCommandOptions) validateScope() error {
	switch contract.SettingsWorkspaceScopeKind(strings.TrimSpace(o.scope)) {
	case contract.SettingsWorkspaceScopeGlobal:
		if strings.TrimSpace(o.workspaceID) != "" {
			return errors.New("cli: --workspace requires --scope workspace")
		}
	case contract.SettingsWorkspaceScopeWorkspace:
		if strings.TrimSpace(o.workspaceID) == "" {
			return errors.New("cli: --scope workspace requires --workspace")
		}
	default:
		return fmt.Errorf("cli: unsupported MCP auth scope %q", o.scope)
	}
	return nil
}

func authorizeMCPServer(
	cmd *cobra.Command,
	client MCPSettingsClient,
	target SettingsMCPAuthTarget,
	manual bool,
	timeout time.Duration,
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
	begin, err := client.BeginSettingsMCPAuth(authCtx, target, SettingsMCPAuthBeginRequest{Mode: mode})
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
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Paste the authorization code or full redirect URL:"); err != nil {
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
	type readResult struct {
		value string
		err   error
	}
	results := make(chan readResult, 1)
	go func() {
		value, err := readManualMCPAuthInput(input, output)
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
		if closer, ok := input.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return "", errors.Join(timeoutErr, fmt.Errorf("cli: cancel MCP authorization input: %w", err))
			}
		}
		return "", timeoutErr
	}
}

func mcpAuthorizationTimeoutError(err error) error {
	return fmt.Errorf("cli: MCP authorization timed out: %w", err)
}

func readManualMCPAuthInput(input io.Reader, output io.Writer) (string, error) {
	return readManualMCPAuthInputWithTerminal(
		input,
		output,
		supportBundleInputIsTerminal,
		term.ReadPassword,
	)
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
		return SettingsMCPAuthExchangeRequest{}, errors.New("cli: authorization code or redirect URL is required")
	}
	parsed, err := url.Parse(input)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return SettingsMCPAuthExchangeRequest{RedirectURL: input}, nil
	}
	return SettingsMCPAuthExchangeRequest{Code: input}, nil
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
	response, err := client.ListSettingsMCPServers(ctx, target.Scope, target.WorkspaceID)
	if err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	opts := mcpAuthCommandOptions{scope: string(target.Scope), workspaceID: target.WorkspaceID}
	statuses, found := mcpAuthStatuses(response.MCPServers, opts, target.Name)
	if !found {
		return SettingsMCPAuthStatusRecord{}, fmt.Errorf(
			"cli: remote MCP auth server %q not found in %s scope",
			target.Name,
			target.Scope,
		)
	}
	if len(statuses) != 1 {
		return SettingsMCPAuthStatusRecord{}, fmt.Errorf(
			"cli: remote MCP server %q in %s scope does not configure OAuth",
			target.Name,
			target.Scope,
		)
	}
	return statuses[0], nil
}

func mcpAuthStatuses(
	servers []contract.SettingsMCPServerItemPayload,
	opts mcpAuthCommandOptions,
	name string,
) ([]SettingsMCPAuthStatusRecord, bool) {
	statuses := make([]SettingsMCPAuthStatusRecord, 0, len(servers))
	found := false
	for _, server := range servers {
		if string(server.Scope) != strings.TrimSpace(opts.scope) ||
			server.WorkspaceID != strings.TrimSpace(opts.workspaceID) ||
			(name != "" && server.Name != name) {
			continue
		}
		found = true
		if server.AuthStatus == nil {
			continue
		}
		statuses = append(statuses, *server.AuthStatus)
	}
	return statuses, found
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
