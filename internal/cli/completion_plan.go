package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const completionWorktreeFlagName = "worktree"

var completionFlagFamilyTable = map[string]completionKind{
	workspaceFlagName:           kindWorkspace,
	automationLoopWorkspaceFlag: kindWorkspace,
	profileFlagName:             kindProfile,
	automationAgentFlag:         kindAgent,
	"default-agent":             kindAgent,
	"for-agent":                 kindAgent,
	"target-instance":           kindAgent,
	cliProviderKey:              kindProvider,
	agentModelKey:               kindModel,
	sessionSessionKey:           kindSession,
	"from-session":              kindSession,
	"origin-session":            kindSession,
	"root":                      kindSession,
	"parent":                    kindSession,
	taskTaskKey:                 kindTask,
	"depends-on":                kindTask,
	loopLoopKey:                 kindLoop,
	"fork-from-name":            kindLoop,
	"job-id":                    kindAutomation,
	completionWorktreeFlagName:  kindWorktree,
	taskWorktreeRefKey:          kindWorktree,
	sessionFolderFlagName:       kindSessionFolder,
	"extension":                 kindExtension,
	computerFlagName:            kindComputer,
	"vault-ref":                 kindVault,
	"toolset":                   kindToolset,
	outputFlagName:              kindOutput,
}

type completionFlagOverride struct {
	kind   completionKind
	optOut bool
}

var completionFlagOverrides = map[string]map[string]completionFlagOverride{
	"account login": {
		cliProviderKey: {optOut: true},
	},
	"support bundle": {
		outputFlagName: {optOut: true},
	},
	"task list": {
		"parent": {kind: kindTask},
	},
	"task profile update": {
		profileFlagName: {optOut: true},
	},
	"workspace move": {
		sessionFolderFlagName: {kind: kindWorkspaceFolder},
	},
	"workspace reorder": {
		sessionFolderFlagName: {kind: kindWorkspaceFolder},
	},
	"loop inspect": {
		loopNameKey: {kind: kindLoop},
	},
	"loop validate": {
		loopNameKey: {kind: kindLoop},
	},
	"loop run": {
		loopNameKey: {kind: kindLoop},
	},
	"loop configure": {
		loopNameKey: {kind: kindLoop},
	},
	"loop edit": {
		loopNameKey: {kind: kindLoop},
	},
	"loop delete": {
		loopNameKey: {kind: kindLoop},
	},
}

func configureDynamicCompletions(root *cobra.Command, deps commandDeps) {
	if root == nil {
		return
	}
	rt := newCompletionRuntime(deps)
	registered := make(map[*pflag.Flag]completionKind)
	configureCommandCompletions(root, rt, registered)
}

func configureCommandCompletions(
	cmd *cobra.Command,
	rt *completionRuntime,
	registered map[*pflag.Flag]completionKind,
) {
	if cmd == nil {
		return
	}
	path := completionCommandPath(cmd)
	registerFlagCompletions(cmd, path, rt, registered)
	registerPositionalCompletions(cmd, path, rt)

	for _, child := range cmd.Commands() {
		configureCommandCompletions(child, rt, registered)
	}
}

func completionCommandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	root := cmd.Root()
	if root == nil || cmd == root {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" "))
}

func completionFlagKind(commandPath string, flagName string) (completionKind, bool) {
	if overrides, exists := completionFlagOverrides[commandPath]; exists {
		if override, ok := overrides[flagName]; ok {
			if override.optOut || override.kind == kindNone {
				return kindNone, false
			}
			return override.kind, true
		}
	}
	kind, ok := completionFlagFamilyTable[flagName]
	return kind, ok && kind != kindNone
}

func registerFlagCompletions(
	cmd *cobra.Command,
	commandPath string,
	rt *completionRuntime,
	registered map[*pflag.Flag]completionKind,
) {
	visit := func(flag *pflag.Flag) {
		if flag == nil || flag.Hidden {
			return
		}
		kind, ok := completionFlagKind(commandPath, flag.Name)
		if !ok {
			return
		}
		if existing, seen := registered[flag]; seen {
			if existing == kind {
				return
			}
			return
		}
		fn := buildFlagCompletionFunc(kind, flag.Name, rt)
		if err := cmd.RegisterFlagCompletionFunc(flag.Name, fn); err != nil {
			if strings.Contains(err.Error(), "already registered") && registered[flag] == kind {
				return
			}
			return
		}
		registered[flag] = kind
	}

	localFlags := make(map[string]struct{})
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag == nil {
			return
		}
		localFlags[flag.Name] = struct{}{}
		visit(flag)
	})
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag == nil {
			return
		}
		if _, shadowed := localFlags[flag.Name]; shadowed {
			return
		}
		visit(flag)
	})
}

func buildFlagCompletionFunc(
	kind completionKind,
	flagName string,
	rt *completionRuntime,
) cobra.CompletionFunc {
	return completionAdapter(
		cobra.ShellCompDirectiveNoFileComp,
		func(cmd *cobra.Command, _ []string, _ string) []completionCandidate {
			workspaceOverride := ""
			if kind == kindLoop && flagName == automationLoopFlag {
				if flag := cmd.Flag(automationLoopWorkspaceFlag); flag != nil {
					workspaceOverride = strings.TrimSpace(flag.Value.String())
				}
			}
			providerOverride := ""
			if kind == kindModel {
				if flag := cmd.Flag(cliProviderKey); flag != nil {
					providerOverride = strings.TrimSpace(flag.Value.String())
				}
			}
			return rt.complete(cmd, kind, workspaceOverride, providerOverride)
		},
	)
}

func registerPositionalCompletions(cmd *cobra.Command, commandPath string, rt *completionRuntime) {
	if len(cmd.ValidArgs) > 0 {
		return
	}
	if _, optedOut := completionCommandOptOut[commandPath]; optedOut {
		if _, keepFiles := completionCommandFileOptOut[commandPath]; !keepFiles {
			cmd.ValidArgsFunction = cobra.NoFileCompletions
		}
		return
	}
	spec, ok := completionCommandTable[commandPath]
	if !ok || len(spec.args) == 0 || allArgsAreFiles(spec) {
		return
	}
	cmd.ValidArgsFunction = buildPositionalCompletionFunc(spec, rt)
}

func allArgsAreFiles(spec completionCommandSpec) bool {
	if len(spec.args) == 0 {
		return false
	}
	for _, arg := range spec.args {
		if !arg.file {
			return false
		}
	}
	return true
}

func buildPositionalCompletionFunc(spec completionCommandSpec, rt *completionRuntime) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		idx := len(args)
		var argSpec completionArgSpec
		switch {
		case idx < len(spec.args):
			argSpec = spec.args[idx]
		case spec.variadic && len(spec.args) > 0:
			argSpec = spec.args[len(spec.args)-1]
		default:
			return nil, cobra.ShellCompDirectiveDefault
		}

		if argSpec.file || argSpec.kind == kindNone {
			return nil, cobra.ShellCompDirectiveDefault
		}

		providerOverride := ""
		if argSpec.kind == kindModel && len(args) > 0 {
			providerOverride = strings.TrimSpace(args[0])
		}
		candidates := rt.complete(cmd, argSpec.kind, "", providerOverride)
		return renderCompletionCandidates(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}
