package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/spf13/cobra"
)

type loopInputCatalogClient interface {
	ListAgents(context.Context, AgentQuery) ([]AgentRecord, error)
	ListSkills(context.Context, SkillQuery) ([]SkillRecord, error)
	ListWorktrees(context.Context, string, bool) (WorktreeListRecord, error)
	ListSessions(context.Context, SessionListQuery) (SessionListPage, error)
	ListWorkspaces(context.Context) ([]WorkspaceRecord, error)
	ListVaultSecrets(context.Context, VaultListQuery) ([]VaultRecord, error)
	ListProviderModels(context.Context, ProviderModelListQuery) (ProviderModelListRecord, error)
}

func normalizeLoopRunInputs(definition dsl.Definition, values map[string]any) (map[string]any, error) {
	if values == nil {
		values = map[string]any{}
	}
	for field, input := range definition.Inputs {
		value, present := values[field]
		if !present || input.Type != dsl.InputTypeRuntime {
			continue
		}
		expression, ok := value.(string)
		if !ok {
			continue
		}
		runtime, err := parseLoopRuntimeExpression(expression)
		if err != nil {
			return nil, fmt.Errorf("cli: parse runtime input %q: %w", field, err)
		}
		values[field] = runtimeInputMap(runtime)
	}
	return values, nil
}

func promptForMissingLoopInputs(
	cmd *cobra.Command,
	deps commandDeps,
	client loopCommandClient,
	workspaceID string,
	definition dsl.Definition,
	values map[string]any,
	noPrompt bool,
) (map[string]any, error) {
	if noPrompt {
		return values, nil
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return nil, err
	}
	input := cmd.InOrStdin()
	if mode != OutputHuman || deps.inputIsTerminal == nil || !deps.inputIsTerminal(input) {
		return values, nil
	}
	configured, err := effectiveConfiguredLoopInputs(
		cmd.Context(), client, workspaceID, definition.Meta.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("cli: resolve configured Loop input defaults: %w", err)
	}
	satisfied := make(map[string]any, len(configured)+len(values))
	for field, value := range configured {
		satisfied[field] = value
	}
	for field, value := range values {
		satisfied[field] = value
	}
	missing := missingRequiredLoopInputs(definition, satisfied)
	if len(missing) == 0 {
		return values, nil
	}
	reader := bufio.NewReader(input)
	for _, field := range missing {
		declaration := definition.Inputs[field]
		choices, choiceErr := loopInputChoices(
			cmd.Context(), client, workspaceID, definition.Meta.Name, declaration,
		)
		if choiceErr != nil {
			if _, err := fmt.Fprintf(
				cmd.ErrOrStderr(), "Catalog unavailable for %s: %v\n", field, choiceErr,
			); err != nil {
				return nil, fmt.Errorf("cli: write Loop input catalog warning: %w", err)
			}
		}
		raw, err := promptLoopInputValue(cmd.ErrOrStderr(), reader, field, declaration, choices)
		if err != nil {
			return nil, err
		}
		value, err := parsePromptedLoopInput(declaration, raw)
		if err != nil {
			return nil, fmt.Errorf("cli: input %q: %w", field, err)
		}
		if err := loop.ValidateInputLayer(
			definition, definition.Meta.Name, map[string]any{field: value}, loop.InputOriginRun,
		); err != nil {
			return nil, err
		}
		values[field] = value
	}
	return values, nil
}

func missingRequiredLoopInputs(definition dsl.Definition, values map[string]any) []string {
	fields := make([]string, 0)
	for field, input := range definition.Inputs {
		if !input.Required || input.Default != nil {
			continue
		}
		if _, present := values[field]; !present {
			fields = append(fields, field)
		}
	}
	sort.Strings(fields)
	return fields
}

func loopInputChoices(
	ctx context.Context,
	client loopCommandClient,
	workspaceID string,
	loopName string,
	input dsl.Input,
) ([]string, error) {
	if len(input.Enum) > 0 {
		return append([]string(nil), input.Enum...), nil
	}
	if input.Type == dsl.InputTypeBoolean {
		return []string{toolBoolTrue, toolBoolFalse}, nil
	}
	if input.Type == dsl.InputTypeRef && input.Ref != nil && input.Ref.Kind == dsl.InputRefKindLoop {
		return listAllLoopInputChoices(ctx, client, workspaceID, loopName)
	}
	catalog, ok := client.(loopInputCatalogClient)
	if !ok {
		return nil, nil
	}
	return loopCatalogChoices(ctx, catalog, workspaceID, input)
}

func loopCatalogChoices(
	ctx context.Context,
	client loopInputCatalogClient,
	workspaceID string,
	input dsl.Input,
) ([]string, error) {
	if input.Type == dsl.InputTypeRuntime {
		return loopRuntimeCatalogChoices(ctx, client)
	}
	return loopEntityCatalogChoices(ctx, client, workspaceID, loopInputCatalogKind(input))
}

func loopInputCatalogKind(input dsl.Input) dsl.EntityKind {
	switch input.Type {
	case dsl.InputTypeAgent:
		return dsl.EntityKindAgent
	case dsl.InputTypeRef:
		if input.Ref != nil {
			return dsl.EntityKind(input.Ref.Kind)
		}
	}
	return ""
}

func loopRuntimeCatalogChoices(ctx context.Context, client loopInputCatalogClient) ([]string, error) {
	models, err := client.ListProviderModels(ctx, ProviderModelListQuery{})
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(models.Models))
	for _, model := range models.Models {
		base := model.ProviderID + "/" + model.ModelID
		if len(model.ReasoningEfforts) == 0 {
			values = append(values, base)
			continue
		}
		for _, effort := range model.ReasoningEfforts {
			values = append(values, base+"@"+string(effort))
		}
	}
	return sortedUniqueStrings(values), nil
}

func loopEntityCatalogChoices(
	ctx context.Context,
	client loopInputCatalogClient,
	workspaceID string,
	kind dsl.EntityKind,
) ([]string, error) {
	switch kind {
	case dsl.EntityKindAgent:
		items, err := client.ListAgents(ctx, AgentQuery{Workspace: workspaceID})
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, item.Name)
		}
		return sortedUniqueStrings(values), nil
	case dsl.EntityKindSkill:
		items, err := client.ListSkills(ctx, SkillQuery{Workspace: workspaceID})
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, item.Name)
		}
		return sortedUniqueStrings(values), nil
	case dsl.EntityKindWorktree:
		items, err := client.ListWorktrees(ctx, workspaceID, false)
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(items.Worktrees))
		for _, item := range items.Worktrees {
			values = append(values, item.ID)
		}
		return sortedUniqueStrings(values), nil
	case dsl.EntityKindSession:
		return listAllSessionInputChoices(ctx, client, workspaceID)
	case dsl.EntityKindWorkspace:
		items, err := client.ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, item.ID)
		}
		return sortedUniqueStrings(values), nil
	case dsl.EntityKindSecret:
		items, err := client.ListVaultSecrets(ctx, VaultListQuery{})
		if err != nil {
			return nil, err
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, item.Ref)
		}
		return sortedUniqueStrings(values), nil
	default:
		return nil, nil
	}
}

func promptLoopInputValue(
	output io.Writer,
	reader *bufio.Reader,
	field string,
	input dsl.Input,
	choices []string,
) (string, error) {
	if _, err := fmt.Fprintf(
		output, "%s (%s)\n", safeLoopPromptText(field), safeLoopPromptText(effectivePromptInputKind(input)),
	); err != nil {
		return "", fmt.Errorf("cli: write Loop input prompt: %w", err)
	}
	for index, choice := range choices {
		if _, err := fmt.Fprintf(output, "  %d) %s\n", index+1, safeLoopPromptText(choice)); err != nil {
			return "", fmt.Errorf("cli: write Loop input choice: %w", err)
		}
	}
	if _, err := fmt.Fprint(output, "Value: "); err != nil {
		return "", fmt.Errorf("cli: write Loop input prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("cli: read Loop input: %w", err)
	}
	answer := strings.TrimSpace(line)
	if selected, err := strconv.Atoi(answer); err == nil && selected >= 1 && selected <= len(choices) {
		return choices[selected-1], nil
	}
	return answer, nil
}

func parsePromptedLoopInput(input dsl.Input, raw string) (any, error) {
	if input.Type == dsl.InputTypeRuntime {
		runtime, err := parseLoopRuntimeExpression(raw)
		if err != nil {
			return nil, err
		}
		return runtimeInputMap(runtime), nil
	}
	switch input.Type {
	case dsl.InputTypeBoolean, dsl.InputTypeNumber:
		return parseLoopValue(raw), nil
	default:
		return raw, nil
	}
}

func effectiveConfiguredLoopInputs(
	ctx context.Context,
	client loopCommandClient,
	workspaceID string,
	loopName string,
) (map[string]any, error) {
	global, err := client.GetLoopInputDefaults(
		ctx, workspaceID, loopName, contract.LoopInputDefaultsScopeGlobal,
	)
	if err != nil {
		return nil, err
	}
	workspace, err := client.GetLoopInputDefaults(
		ctx, workspaceID, loopName, contract.LoopInputDefaultsScopeWorkspace,
	)
	if err != nil {
		return nil, err
	}
	values := make(map[string]any, len(global.Values)+len(workspace.Values))
	for field, value := range global.Values {
		values[field] = value
	}
	for field, value := range workspace.Values {
		values[field] = value
	}
	return values, nil
}

func listAllLoopInputChoices(
	ctx context.Context,
	client loopCommandClient,
	workspaceID string,
	currentLoop string,
) ([]string, error) {
	var values []string
	cursor := ""
	for {
		response, err := client.ListLoops(ctx, workspaceID, LoopListQuery{Cursor: cursor, Limit: 100})
		if err != nil {
			return nil, err
		}
		for _, item := range response.Loops {
			if item.Name != currentLoop {
				values = append(values, item.Name)
			}
		}
		if !response.Page.HasMore || response.Page.NextCursor == "" {
			return sortedUniqueStrings(values), nil
		}
		cursor = response.Page.NextCursor
	}
}

func listAllSessionInputChoices(
	ctx context.Context,
	client loopInputCatalogClient,
	workspaceID string,
) ([]string, error) {
	var values []string
	cursor := ""
	for {
		page, err := client.ListSessions(ctx, SessionListQuery{Workspace: workspaceID, Cursor: cursor, Limit: 100})
		if err != nil {
			return nil, err
		}
		for _, item := range page.Sessions {
			values = append(values, item.ID)
		}
		if !page.Page.HasMore || page.Page.NextCursor == "" {
			return sortedUniqueStrings(values), nil
		}
		cursor = page.Page.NextCursor
	}
}

func safeLoopPromptText(value string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsControl(char) {
			return unicode.ReplacementChar
		}
		return char
	}, value)
}

func runtimeInputMap(runtime loop.RuntimeSpec) map[string]any {
	value := map[string]any{}
	if runtime.Provider != "" {
		value["provider"] = runtime.Provider
	}
	if runtime.Model != "" {
		value["model"] = runtime.Model
	}
	if runtime.Reasoning != "" {
		value["reasoning"] = runtime.Reasoning
	}
	return value
}

func effectivePromptInputKind(input dsl.Input) string {
	if len(input.Enum) > 0 {
		return "choice"
	}
	if input.Type == dsl.InputTypeRef && input.Ref != nil {
		return string(input.Ref.Kind)
	}
	return string(input.Type)
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
