package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/spf13/cobra"
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
	scope                  string
	workspaceID            string
	profile                string
	manual                 bool
	timeout                time.Duration
	approvedScopes         []string
	approveScopeEscalation bool
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

func newMCPAuthLoginCommand(deps commandDeps) *cobra.Command {
	return newMCPAuthorizationCommand(
		deps,
		"login <server>",
		"Authorize a remote MCP server through the daemon",
	)
}

func newMCPAuthorizationCommand(deps commandDeps, use string, short string) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser)}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.timeout <= 0 {
				return errors.New("cli: MCP authorization timeout must be positive")
			}
			if len(opts.approvedScopes) > 0 && !opts.approveScopeEscalation {
				return errors.New("cli: --approved-scope requires --approve-scope-escalation")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			resolvedOpts, err := opts.resolveTarget(cmd, deps, client)
			if err != nil {
				return err
			}
			target, err := resolvedOpts.target(args[0])
			if err != nil {
				return err
			}
			status, err := authorizeMCPServer(
				cmd,
				client,
				target,
				opts.manual,
				opts.timeout,
				opts.approvedScopes,
				opts.approveScopeEscalation,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, mcpAuthStatusBundle(status))
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.manual, "manual", false, "Paste the full authorization redirect URL")
	cmd.Flags().StringArrayVar(
		&opts.approvedScopes,
		"approved-scope",
		nil,
		"Approve one additional OAuth scope (repeatable)",
	)
	cmd.Flags().BoolVar(
		&opts.approveScopeEscalation,
		"approve-scope-escalation",
		false,
		"Confirm the requested OAuth scope escalation",
	)
	cmd.Flags().DurationVar(&opts.timeout, cliTimeoutKey, defaultMCPAuthLoginTimeout, "Authorization timeout")
	return cmd
}

func newMCPAuthStatusCommand(deps commandDeps) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser)}
	cmd := &cobra.Command{
		Use:   "status [server]",
		Short: "Show redacted remote MCP auth status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			resolvedOpts, err := opts.resolveTarget(cmd, deps, client)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				target, targetErr := resolvedOpts.target(args[0])
				if targetErr != nil {
					return targetErr
				}
				status, statusErr := client.GetSettingsMCPAuthStatus(cmd.Context(), target)
				if statusErr != nil {
					return statusErr
				}
				if status.Status == string(mcpauth.StatusUnconfigured) {
					return writeCommandOutput(cmd, mcpAuthStatusListBundle(nil))
				}
				return writeCommandOutput(cmd, mcpAuthStatusListBundle([]SettingsMCPAuthStatusRecord{status}))
			}
			scope := contract.SettingsLayeredScopeKind(strings.TrimSpace(resolvedOpts.scope))
			workspaceID := strings.TrimSpace(resolvedOpts.workspaceID)
			response, err := client.ListSettingsMCPServers(
				cmd.Context(), scope, workspaceID, resolvedOpts.profile,
			)
			if err != nil {
				return err
			}
			statuses := mcpAuthStatuses(response.MCPServers, resolvedOpts)
			return writeCommandOutput(cmd, mcpAuthStatusListBundle(statuses))
		},
	}
	addMCPAuthTargetFlags(cmd, &opts)
	return cmd
}

func newMCPAuthLogoutCommand(deps commandDeps) *cobra.Command {
	opts := mcpAuthCommandOptions{scope: string(contract.SettingsLayeredScopeUser)}
	cmd := &cobra.Command{
		Use:   "logout <server>",
		Short: "Revoke or delete remote MCP auth tokens through the daemon",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			resolvedOpts, err := opts.resolveTarget(cmd, deps, client)
			if err != nil {
				return err
			}
			target, err := resolvedOpts.target(args[0])
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
	cmd.Flags().StringVar(&opts.scope, "scope", opts.scope, "MCP server scope: user, profile, or workspace")
	cmd.Flags().
		StringVar(&opts.workspaceID, "workspace", "", "Override workspace (ID, name, or path)")
}

func (o mcpAuthCommandOptions) resolveTarget(
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
) (mcpAuthCommandOptions, error) {
	scope := contract.SettingsLayeredScopeKind(strings.TrimSpace(o.scope))
	if scope == contract.SettingsLayeredScopeWorkspace {
		resolution, err := resolveCommandWorkspace(
			cmd.Context(),
			cmd,
			deps,
			client,
			workspaceResolutionRequest{FlagRef: o.workspaceID},
		)
		if err != nil {
			return mcpAuthCommandOptions{}, err
		}
		o.workspaceID = resolution.ID
	}
	if scope == contract.SettingsLayeredScopeProfile {
		profiles, _, err := profileClientFromDeps(deps)
		if err != nil {
			return mcpAuthCommandOptions{}, err
		}
		resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
		if err != nil {
			return mcpAuthCommandOptions{}, err
		}
		o.profile = strings.TrimSpace(resolution.Profile.Name)
	}
	if err := o.validateScope(); err != nil {
		return mcpAuthCommandOptions{}, err
	}
	return o, nil
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
		Scope:       contract.SettingsLayeredScopeKind(strings.TrimSpace(o.scope)),
		WorkspaceID: strings.TrimSpace(o.workspaceID),
		Profile:     strings.TrimSpace(o.profile),
	}, nil
}

func (o mcpAuthCommandOptions) validateScope() error {
	switch contract.SettingsLayeredScopeKind(strings.TrimSpace(o.scope)) {
	case contract.SettingsLayeredScopeUser:
		if strings.TrimSpace(o.workspaceID) != "" || strings.TrimSpace(o.profile) != "" {
			return errors.New("cli: --workspace requires --scope workspace")
		}
	case contract.SettingsLayeredScopeProfile:
		if strings.TrimSpace(o.workspaceID) != "" {
			return errors.New("cli: --workspace requires --scope workspace")
		}
		if strings.TrimSpace(o.profile) == "" || strings.TrimSpace(o.profile) == "default" {
			return errors.New("cli: --scope profile requires an active non-default profile")
		}
	case contract.SettingsLayeredScopeWorkspace:
		if strings.TrimSpace(o.workspaceID) == "" {
			return errors.New("cli: --scope workspace requires --workspace")
		}
		if strings.TrimSpace(o.profile) != "" {
			return errors.New("cli: profile identity requires --scope profile")
		}
	default:
		return fmt.Errorf("cli: unsupported MCP auth scope %q", o.scope)
	}
	return nil
}
