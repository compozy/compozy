package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/spf13/cobra"
)

// ExtensionInputsRecord is the setup view of one extension instance, without values or Vault references.
type ExtensionInputsRecord = contract.ExtensionInputsPayload

type extensionInputsClient interface {
	ExtensionInputs(context.Context, string, extensionCLIScope) (ExtensionInputsRecord, error)
	SetExtensionInputs(
		context.Context, string, extensionCLIScope, contract.SetExtensionInputsRequest,
	) (ExtensionInputsRecord, error)
}

func (c *daemonClient) ExtensionInputs(
	ctx context.Context, name string, selected extensionCLIScope,
) (ExtensionInputsRecord, error) {
	var response ExtensionInputsRecord
	err := c.doJSON(ctx, http.MethodGet, extensionPath(name, "inputs"), extensionInputsQuery(selected), nil, &response)
	return response, err
}

func (c *daemonClient) SetExtensionInputs(
	ctx context.Context, name string, selected extensionCLIScope, request contract.SetExtensionInputsRequest,
) (ExtensionInputsRecord, error) {
	var response ExtensionInputsRecord
	err := c.doJSON(
		ctx, http.MethodPut, extensionPath(name, "inputs"), extensionInputsQuery(selected), request, &response,
	)
	return response, err
}

func extensionInputsQuery(selected extensionCLIScope) url.Values {
	query := make(url.Values)
	if selected.scope != "" {
		query.Set("scope", selected.scope)
	}
	if selected.workspaceID != "" {
		query.Set(extensionWorkspaceQueryKey, selected.workspaceID)
	}
	if selected.profile != "" {
		query.Set(profileFlagName, selected.profile)
	}
	return query
}

type extensionSetupFlags struct {
	scope        string
	workspaceRef string
	inputs       extensionInputFlags
	all          bool
}

func newExtensionSetupCommand(deps commandDeps) *cobra.Command {
	flags := &extensionSetupFlags{}
	cmd := &cobra.Command{
		Use:   "setup <name>",
		Short: "Complete or change the inputs of one installed extension",
		Long: "Prompt for required inputs that are missing or invalid, and for inputs that only the daemon " +
			"environment satisfies, then write them and restart the instance when it is enabled. --all revisits " +
			"every input. --input and --input-file write without prompting. Structured output never prompts.",
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionSetup(cmd, deps, args[0], flags)
		},
	}
	cmd.Flags().StringVar(&flags.scope, "scope", "", "Select the global or workspace installation: global, workspace")
	cmd.Flags().StringVar(&flags.workspaceRef, workspaceFlagName, "", "Override workspace context")
	cmd.Flags().BoolVar(&flags.all, "all", false, "Prompt for every declared input, including optional ones")
	flags.inputs.registerValues(cmd)
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func runExtensionSetup(cmd *cobra.Command, deps commandDeps, name string, f *extensionSetupFlags) error {
	explicit := len(f.inputs.values) > 0 || strings.TrimSpace(f.inputs.file) != ""
	if explicit && f.all {
		return errors.New("cli: --all prompts for inputs and cannot be combined with --input or --input-file")
	}
	selected, err := resolveExtensionCLIScope(cmd, deps, f.scope, f.workspaceRef)
	if err != nil {
		return err
	}
	client, err := requireExtensionInputsClient(cmd.Context(), deps)
	if err != nil {
		return err
	}
	state, err := client.ExtensionInputs(cmd.Context(), name, selected)
	if err != nil {
		return err
	}
	values, err := extensionSetupValues(cmd, deps, state, selected, f, explicit)
	if err != nil {
		return err
	}
	result, wrote := state, len(values) > 0
	if wrote {
		result, err = client.SetExtensionInputs(
			cmd.Context(), name, selected, contract.SetExtensionInputsRequest{Inputs: values},
		)
		if err != nil {
			return err
		}
	}
	if err := writeCommandOutput(cmd, extensionSetupBundle(result, wrote, selected.profile)); err != nil {
		return err
	}
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if pending := extensionSetupPending(result); mode == OutputHuman && len(pending) > 0 {
		return withCommandExitCode(2, fmt.Errorf(
			"extension inputs required: %s. Pass --input id=value or --input-file inputs.json",
			strings.Join(pending, ", "),
		))
	}
	return nil
}

func extensionSetupValues(
	cmd *cobra.Command, deps commandDeps, state ExtensionInputsRecord, selected extensionCLIScope,
	f *extensionSetupFlags, explicit bool,
) (map[string]extensioninput.Value, error) {
	interactive, err := extensionInputsInteractive(cmd, deps)
	if err != nil {
		return nil, err
	}
	if (explicit || interactive) && state.WorkspaceID == "" && selected.workspaceID != "" {
		// The daemon refuses a workspace write to a global installation; fail before collecting values.
		return nil, fmt.Errorf(
			"cli: %s is installed globally; set up its inputs with: compozy extension setup %s --scope global",
			state.Extension, state.Extension,
		)
	}
	if explicit {
		values, err := readExtensionInputFile(f.inputs.file)
		if err != nil {
			return nil, err
		}
		types := make(map[string]string, len(state.Inputs))
		for _, input := range state.Inputs {
			types[input.ID] = input.Type
		}
		return mergeExtensionInputFlags(values, f.inputs.values, types)
	}
	if !interactive {
		return nil, nil
	}
	return newExtensionInputPrompter(cmd).promptAll(cmd.Context(), extensionSetupPromptInputs(state, f.all))
}

// extensionSetupPromptInputs selects what setup asks for: every input with --all; otherwise required
// missing or invalid inputs and every daemon_env input, required or optional.
func extensionSetupPromptInputs(state ExtensionInputsRecord, all bool) []extensionPromptInput {
	inputs := make([]extensionPromptInput, 0, len(state.Inputs))
	for _, input := range state.Inputs {
		needed := input.State == contract.ExtensionInputStateDaemonEnv ||
			(input.Required && (input.State == contract.ExtensionInputStateMissing ||
				input.State == contract.ExtensionInputStateInvalid))
		if !all && !needed {
			continue
		}
		inputs = append(inputs, extensionPromptInput{
			ID: input.ID, Prompt: input.Prompt, Type: input.Type, Required: input.Required,
			Default: input.Default, State: input.State, Subprocess: slices.Contains(input.Targets, "subprocess"),
		})
	}
	return inputs
}

// extensionSetupPending lists required inputs that still block the instance.
func extensionSetupPending(result ExtensionInputsRecord) []string {
	pending := slices.Clone(result.Missing)
	for _, input := range result.Inputs {
		if input.Required && input.State == contract.ExtensionInputStateInvalid && !slices.Contains(pending, input.ID) {
			pending = append(pending, input.ID)
		}
	}
	return pending
}

// extensionSetupAuthServers lists OAuth MCP servers that at least one input feeds.
func extensionSetupAuthServers(result ExtensionInputsRecord) []string {
	servers := make([]string, 0, len(result.MCPAuthServers))
	for _, server := range result.MCPAuthServers {
		fed := slices.ContainsFunc(result.Inputs, func(input contract.ExtensionInputStatusPayload) bool {
			return slices.Contains(input.Targets, "mcp:"+server)
		})
		if fed {
			servers = append(servers, server)
		}
	}
	return servers
}

func extensionSetupBundle(result ExtensionInputsRecord, wrote bool, profile string) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			return renderExtensionSetupHuman(result, wrote, profile), nil
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(result.Inputs))
			for _, input := range result.Inputs {
				rows = append(rows, []string{
					input.ID, input.Type, formatBool(input.Required), input.State, strings.Join(input.Targets, " "),
				})
			}
			return renderToonArray(
				"extension_inputs", []string{"id", "type", "required", "state", "targets"}, rows,
			), nil
		},
	}
}

// renderExtensionSetupHuman prints the setup outcome. Its secrets set hint targets the same instance:
// the instance's workspace (absent for a global installation) and the explicitly selected profile.
func renderExtensionSetupHuman(result ExtensionInputsRecord, wrote bool, profile string) string {
	var lines []string
	name := result.Extension
	pending := extensionSetupPending(result)
	switch {
	case wrote && len(result.Changed) == 0:
		lines = append(lines, "No inputs of "+name+" changed.")
	case wrote:
		line := "Updated " + strconv.Itoa(len(result.Changed)) + " input"
		if len(result.Changed) != 1 {
			line += "s"
		}
		line += " of " + name + "."
		if result.Reloaded {
			line += " Restarted " + name + "."
		}
		lines = append(lines, line)
	}
	if len(pending) == 0 {
		lines = append(lines, "All required inputs of "+name+" are set.")
	} else {
		rows := make([][]string, 0, len(result.Inputs))
		for _, input := range result.Inputs {
			rows = append(rows, []string{
				input.ID, input.Type, formatBool(input.Required), input.State, strings.Join(input.Targets, " "),
			})
		}
		lines = append(lines, strings.TrimRight(renderHumanTable(
			"Extension Inputs", []string{"ID", "TYPE", "REQUIRED", "STATE", "TARGETS"}, rows,
		), "\n"))
	}
	if len(result.MissingEnv) > 0 {
		selectors := ""
		if result.WorkspaceID != "" {
			selectors += " --" + workspaceFlagName + " " + result.WorkspaceID
		}
		if profile != "" {
			selectors += " --" + profileFlagName + " " + profile
		}
		lines = append(lines, fmt.Sprintf(
			"Environment keys without an input: %s. Set them with: compozy extension secrets set %s%s --env <KEY>",
			strings.Join(result.MissingEnv, ", "), name, selectors,
		))
	}
	for _, server := range extensionSetupAuthServers(result) {
		lines = append(lines, "Sign in: compozy mcp auth login "+server)
	}
	return strings.Join(lines, "\n") + "\n"
}

func requireExtensionInputsClient(ctx context.Context, deps commandDeps) (extensionInputsClient, error) {
	base, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	client, ok := base.(extensionInputsClient)
	if !ok {
		return nil, errors.New("cli: extension input setup transport is unavailable")
	}
	return client, nil
}
