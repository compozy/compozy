package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	extensionExecAgentArg         = "--agent"
	extensionExecApprovalTokenArg = "--approval-token"
	extensionExecCommandArg       = "--cmd"
	extensionExecInputArg         = "--input"
	extensionExecSessionArg       = "--session"
	extensionExecWorkspaceArg     = "--workspace"
)

type extensionExecHostFlags struct {
	command       string
	input         string
	output        string
	workspace     string
	session       string
	agent         string
	approvalToken string
	inputSet      bool
	outputSet     bool
	json          bool
	jsonSet       bool
	help          bool
}

func newExtensionExecCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "exec <extension> --cmd <path> [flags]",
		Short:              "Invoke an extension command through daemon tool policy",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			extensionName, host, projectedArgv, err := parseExtensionExecArgs(args)
			if err != nil {
				return err
			}
			if err := applyExtensionExecOutputFlags(cmd, host); err != nil {
				return err
			}
			client, err := requireExtensionDaemonClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveCLIWorkspaceRouteRef(cmd, deps, client, host.workspace)
			if err != nil {
				return err
			}
			catalog, err := client.ListExtensionCommands(cmd.Context(), extensionName, workspaceID)
			if err != nil {
				return err
			}
			descriptor, err := selectExtensionCommand(catalog, extensionName, host.command)
			if err != nil {
				return err
			}
			projectedFlags := newProjectedCommandFlagSet(descriptor)
			if host.help {
				return writeExtensionExecHelp(cmd, descriptor)
			}
			if err := projectedFlags.Parse(projectedArgv); err != nil {
				return fmt.Errorf("cli: parse command %q flags: %w", descriptor.Verb, err)
			}
			if projectedFlags.NArg() != 0 {
				return fmt.Errorf(
					"cli: command %q received unexpected arguments: %s",
					descriptor.Verb,
					strings.Join(projectedFlags.Args(), " "),
				)
			}
			rawInput := ""
			if host.inputSet {
				if strings.TrimSpace(host.input) == "" {
					return errors.New("cli: --input requires a non-empty JSON document")
				}
				rawInput = host.input
			}
			input, err := buildCommandInput(descriptor, projectedFlags, rawInput)
			if err != nil {
				return err
			}
			toolID, err := parseToolIDArg(descriptor.ToolID)
			if err != nil {
				return err
			}
			response, err := client.InvokeTool(cmd.Context(), toolID.String(), ToolInvokeRequest{
				WorkspaceID:   workspaceID,
				SessionID:     strings.TrimSpace(host.session),
				AgentName:     strings.TrimSpace(host.agent),
				ApprovalToken: strings.TrimSpace(host.approvalToken),
				Input:         input,
			})
			if err != nil {
				return writeToolCommandError(cmd, err)
			}
			return writeCommandOutput(cmd, toolInvokeBundle(sanitizeToolInvokeResponse(response)))
		},
	}
	cmd.Flags().String("cmd", "", "Extension command path")
	cmd.Flags().String("input", "", "Inline JSON input instead of projected flags")
	return cmd
}

func parseExtensionExecArgs(args []string) (string, extensionExecHostFlags, []string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
		return "", extensionExecHostFlags{}, nil, errors.New("cli: exec requires an extension name before flags")
	}
	extensionName := strings.TrimSpace(args[0])
	host := extensionExecHostFlags{}
	projected := make([]string, 0, len(args)-1)
	for index := 1; index < len(args); index++ {
		token := args[index]
		name, inline, hasInline := strings.Cut(token, "=")
		switch name {
		case extensionExecCommandArg,
			extensionExecInputArg,
			outputFlagArg,
			"-o",
			extensionExecWorkspaceArg,
			extensionExecSessionArg,
			extensionExecAgentArg,
			extensionExecApprovalTokenArg:
			value := inline
			if !hasInline {
				index++
				if index >= len(args) {
					return "", extensionExecHostFlags{}, nil, fmt.Errorf("cli: %s requires a value", name)
				}
				value = args[index]
			}
			if err := setExtensionExecHostValue(&host, name, value); err != nil {
				return "", extensionExecHostFlags{}, nil, err
			}
		case jsonFlagArg:
			host.jsonSet = true
			host.json = true
			if hasInline {
				parsed, parseErr := parseCommandBoolean(inline)
				if parseErr != nil {
					return "", extensionExecHostFlags{}, nil, fmt.Errorf("cli: %s: %w", jsonFlagArg, parseErr)
				}
				host.json = parsed
			}
		case "--help", "-h":
			host.help = true
		default:
			if strings.HasPrefix(token, "-o") && token != "-o" {
				if err := setExtensionExecHostValue(&host, "-o", strings.TrimPrefix(token, "-o")); err != nil {
					return "", extensionExecHostFlags{}, nil, err
				}
				continue
			}
			projected = append(projected, token)
		}
	}
	if strings.TrimSpace(host.command) == "" {
		return "", extensionExecHostFlags{}, nil, errors.New("cli: exec requires --cmd <path>")
	}
	return extensionName, host, projected, nil
}

func setExtensionExecHostValue(host *extensionExecHostFlags, name, value string) error {
	switch name {
	case extensionExecCommandArg:
		host.command = strings.TrimSpace(value)
	case extensionExecInputArg:
		host.input = value
		host.inputSet = true
	case outputFlagArg, "-o":
		host.output = strings.TrimSpace(value)
		host.outputSet = true
	case extensionExecWorkspaceArg:
		host.workspace = strings.TrimSpace(value)
	case extensionExecSessionArg:
		host.session = strings.TrimSpace(value)
	case extensionExecAgentArg:
		host.agent = strings.TrimSpace(value)
	case extensionExecApprovalTokenArg:
		host.approvalToken = strings.TrimSpace(value)
	default:
		return fmt.Errorf("cli: unknown exec host flag %q", name)
	}
	return nil
}

func applyExtensionExecOutputFlags(cmd *cobra.Command, host extensionExecHostFlags) error {
	if host.outputSet {
		if err := cmd.Flags().Set(outputFlagName, host.output); err != nil {
			return fmt.Errorf("cli: set output format: %w", err)
		}
	}
	if host.jsonSet {
		if err := cmd.Flags().Set(jsonFlagName, fmt.Sprintf("%t", host.json)); err != nil {
			return fmt.Errorf("cli: set json output: %w", err)
		}
	}
	return nil
}

func selectExtensionCommand(
	catalog ExtensionCommandsRecord,
	extensionName string,
	path string,
) (contract.ExtensionCommandPayload, error) {
	path = strings.TrimSpace(path)
	for _, command := range catalog.Commands {
		if command.Extension == extensionName && command.Verb == path {
			return command, nil
		}
	}
	for _, group := range catalog.Groups {
		if group.Extension != extensionName || group.Path != path {
			continue
		}
		leaves := commandPathsWithPrefix(catalog.Commands, extensionName, path+"/")
		return contract.ExtensionCommandPayload{}, fmt.Errorf(
			"cli: command path %q is a group and is not executable; available leaves: %s",
			path,
			strings.Join(leaves, ", "),
		)
	}
	available := commandPathsWithPrefix(catalog.Commands, extensionName, "")
	message := fmt.Sprintf("cli: unknown command path %q; available commands: %s", path, strings.Join(available, ", "))
	if suggestion := closestCommandPath(path, available); suggestion != "" {
		message += fmt.Sprintf("; did you mean %q?", suggestion)
	}
	return contract.ExtensionCommandPayload{}, errors.New(message)
}

func commandPathsWithPrefix(
	commands []contract.ExtensionCommandPayload,
	extensionName string,
	prefix string,
) []string {
	paths := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Extension == extensionName && strings.HasPrefix(command.Verb, prefix) {
			paths = append(paths, command.Verb)
		}
	}
	slices.Sort(paths)
	return paths
}

func closestCommandPath(input string, candidates []string) string {
	best := ""
	bestScore := 0
	for _, candidate := range candidates {
		score := commonCommandPathPrefix(input, candidate)*2 - absInt(len(input)-len(candidate))
		if best == "" || score > bestScore {
			best, bestScore = candidate, score
		}
	}
	return best
}

func commonCommandPathPrefix(left, right string) int {
	limit := min(len(left), len(right))
	for index := range limit {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func writeExtensionExecHelp(cmd *cobra.Command, descriptor contract.ExtensionCommandPayload) error {
	var help strings.Builder
	fmt.Fprintf(
		&help,
		"Usage: compozy extension exec %s --cmd %s [flags]\n\n%s\n",
		descriptor.Extension,
		descriptor.Verb,
		descriptor.Summary,
	)
	if len(descriptor.Flags) > 0 {
		help.WriteString("\nProjected flags:\n")
		for _, flag := range descriptor.Flags {
			fmt.Fprintf(&help, "  --%s  %s\n", flag.Name, commandFlagHelp(flag))
		}
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), help.String())
	return err
}
