package acp

import (
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// providerAgy is the canonical Antigravity provider id.
const providerAgy = "agy"

// agyDisabledTools lists Antigravity built-in tools a CompozyOS session turns off.
// run_workflow writes its own skill file (skills/system/run-workflow/SKILL.md) into the
// session workspace, and CompozyOS owns orchestration through its native tools.
var agyDisabledTools = []string{"run_workflow"}

// ErrNativeAgyToolPolicyUnsupported identifies a host policy the native CLI cannot enforce.
var ErrNativeAgyToolPolicyUnsupported = errors.New("acp: native Antigravity tool policy unsupported")

// sessionSetupMeta returns the provider-private `_meta` for session/new and session/load.
func sessionSetupMeta(opts StartOpts) any {
	if meta := claudeCodeSessionMeta(opts); meta != nil {
		return meta
	}
	return agySessionMeta(opts)
}

// agySessionMeta returns Antigravity's session `_meta` for an AGY route; ACP agents
// ignore `_meta` keys they do not own, so other runtimes never read it.
func agySessionMeta(opts StartOpts) any {
	if compozyconfig.CanonicalProviderName(opts.ProviderName) != providerAgy &&
		!compozyconfig.IsNativeAgyACPCommand(startCommand(opts)) {
		return nil
	}
	return map[string]any{
		"antigravity": map[string]any{
			"disabledTools": append([]string(nil), agyDisabledTools...),
		},
	}
}

// validateNativeAgyToolPolicy refuses host policies the native Antigravity CLI cannot
// enforce: it executes its own tools without host permission requests, so it can neither
// ask for nor honor an approval or denial. Approve-all without deny rules keeps the runtime
// usable; allow rules are accepted there because they never narrow what runs. Promptless
// inspection or authentication stays allowed.
func validateNativeAgyToolPolicy(opts StartOpts) error {
	if opts.Inspection || !compozyconfig.IsNativeAgyACPCommand(startCommand(opts)) {
		return nil
	}
	permissions := opts.Permissions
	if permissions == "" {
		permissions = compozyconfig.PermissionModeApproveReads
	}
	if permissions != compozyconfig.PermissionModeApproveAll {
		return fmt.Errorf(
			"%w: the native Antigravity CLI executes its own tools without host permission requests, "+
				"so %q permissions cannot be enforced before a tool runs; "+
				"an operator must select approve-all or a host-mediated Antigravity ACP command",
			ErrNativeAgyToolPolicyUnsupported,
			permissions,
		)
	}
	for i, rule := range opts.PermissionRulesValue() {
		if strings.TrimSpace(rule.Decision) == "deny" {
			return fmt.Errorf(
				"%w: the native Antigravity CLI executes its own tools without host permission requests, "+
					"so permission_rules[%d] deny cannot be enforced; "+
					"an operator must remove the deny rule or select a host-mediated Antigravity ACP command",
				ErrNativeAgyToolPolicyUnsupported,
				i,
			)
		}
	}
	return nil
}

// startCommand returns the launch command, falling back to the provider's configured command.
func startCommand(opts StartOpts) string {
	command := strings.TrimSpace(opts.Command)
	if command == "" && opts.ProviderConfig != nil {
		command = strings.TrimSpace(opts.ProviderConfig.Command)
	}
	return command
}
