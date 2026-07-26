package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type loopCommandClient interface {
	GetWorkspace(ctx context.Context, ref string) (WorkspaceDetailRecord, error)
	ListLoops(ctx context.Context, workspaceID string, query LoopListQuery) (contract.LoopsResponse, error)
	CreateLoop(
		ctx context.Context,
		workspaceID string,
		request contract.CreateLoopRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopResponse, error)
	GetLoop(ctx context.Context, workspaceID string, name string) (contract.LoopResponse, error)
	PatchLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.PatchLoopRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopResponse, error)
	ValidateLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.ValidateLoopRequest,
	) (contract.LoopValidationResponse, error)
	DeleteLoop(ctx context.Context, workspaceID string, name string, credentials agentidentity.Credentials) error
	RunLoop(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.RunLoopRequest,
		dry bool,
		credentials agentidentity.Credentials,
	) (contract.RunLoopResponse, error)
	GetLoopConfig(ctx context.Context, workspaceID string, name string) (contract.LoopConfigResponse, error)
	PutLoopConfig(
		ctx context.Context,
		workspaceID string,
		name string,
		request contract.PutLoopConfigRequest,
		credentials agentidentity.Credentials,
	) (contract.LoopConfigResponse, error)
	ListLoopRuns(ctx context.Context, workspaceID string, query LoopRunListQuery) (contract.LoopRunsResponse, error)
	ListGoalTurns(
		ctx context.Context,
		workspaceID string,
		runID string,
		query GoalTurnListQuery,
	) (contract.GoalTurnPage, error)
	GetLoopRun(ctx context.Context, workspaceID string, runID string) (contract.LoopRunResponse, error)
	StopLoopRun(ctx context.Context, workspaceID string, runID string, credentials agentidentity.Credentials) error
	PauseLoopRun(ctx context.Context, workspaceID string, runID string, credentials agentidentity.Credentials) error
	ResumeLoopRun(ctx context.Context, workspaceID string, runID string, credentials agentidentity.Credentials) error
	ApproveLoopRun(
		ctx context.Context,
		workspaceID string,
		runID string,
		request contract.ApproveLoopRunRequest,
		credentials agentidentity.Credentials,
	) error
}

func loopClientFromDeps(deps commandDeps) (loopCommandClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	loopClient, ok := client.(loopCommandClient)
	if !ok {
		return nil, errors.New("cli: daemon client does not support loop commands")
	}
	return loopClient, nil
}

func resolveLoopWorkspaceID(ctx context.Context, client loopCommandClient, workspaceRef string) (string, error) {
	trimmed := strings.TrimSpace(workspaceRef)
	if trimmed == "" {
		return "", errors.New("cli: --workspace is required")
	}
	detail, err := client.GetWorkspace(ctx, trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: resolve workspace %q: %w", trimmed, err)
	}
	id := strings.TrimSpace(detail.Workspace.ID)
	if id == "" {
		return "", fmt.Errorf("cli: resolve workspace %q: missing workspace id", trimmed)
	}
	return id, nil
}

func requiredLoopFlag(field string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("cli: --%s is required", field)
	}
	return trimmed, nil
}

func readLoopDefinitionFile(path string) (dsl.Definition, error) {
	body, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return dsl.Definition{}, fmt.Errorf("cli: read loop definition file: %w", err)
	}
	var definition dsl.Definition
	if err := yaml.Unmarshal(body, &definition); err != nil {
		return dsl.Definition{}, fmt.Errorf("cli: parse loop definition file: %w", err)
	}
	return definition, nil
}

func loopDefinitionDocumentFromDefinition(definition dsl.Definition) (contract.LoopDefinitionDocument, error) {
	document, err := contract.NewLoopDefinitionDocument(definition)
	if err != nil {
		return contract.LoopDefinitionDocument{}, fmt.Errorf("cli: convert loop definition: %w", err)
	}
	return document, nil
}

func readLoopConfigFile(path string) (looppkg.LoopConfig, error) {
	body, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return looppkg.LoopConfig{}, fmt.Errorf("cli: read loop config file: %w", err)
	}
	var cfg looppkg.LoopConfig
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return looppkg.LoopConfig{}, fmt.Errorf("cli: parse loop config file: %w", err)
	}
	return cfg, nil
}

func loopConfigPayloadFromDomain(cfg *looppkg.LoopConfig) (*contract.LoopConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("cli: marshal loop config: %w", err)
	}
	var payload contract.LoopConfig
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("cli: unmarshal loop config payload: %w", err)
	}
	return &payload, nil
}

func parseLoopInputFlags(flags []string) (map[string]any, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	values := make(map[string]any, len(flags))
	for _, flag := range flags {
		key, value, err := splitLoopKeyValue(flag)
		if err != nil {
			return nil, err
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("cli: duplicate --input key %q", key)
		}
		values[key] = parseLoopValue(value)
	}
	return values, nil
}

func parseLoopConfigSetFlags(flags []string) (*looppkg.LoopConfig, error) {
	if len(flags) == 0 {
		return nil, nil
	}
	values := make(map[string]any, len(flags))
	for _, flag := range flags {
		key, value, err := splitLoopKeyValue(flag)
		if err != nil {
			return nil, err
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("cli: duplicate --set key %q", key)
		}
		values[key] = parseLoopValue(value)
	}
	body, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("cli: marshal loop config flags: %w", err)
	}
	var cfg looppkg.LoopConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("cli: parse loop config flags: %w", err)
	}
	return &cfg, nil
}

func splitLoopKeyValue(raw string) (string, string, error) {
	key, value, ok := strings.Cut(raw, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", "", fmt.Errorf("cli: expected key=value, got %q", raw)
	}
	return key, strings.TrimSpace(value), nil
}

func parseLoopValue(raw string) any {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err == nil {
		if err := decoder.Decode(&struct{}{}); errors.Is(err, io.EOF) {
			return value
		}
	}
	return raw
}

func loopConfigFromFlags(filePath string, setFlags []string) (looppkg.LoopConfig, error) {
	var cfg looppkg.LoopConfig
	hasFile := strings.TrimSpace(filePath) != ""
	hasSet := len(setFlags) > 0
	switch {
	case !hasFile && !hasSet:
		return looppkg.LoopConfig{}, errors.New("cli: --file or --set is required")
	case hasFile && hasSet:
		return looppkg.LoopConfig{}, errors.New("cli: --file and --set cannot be combined")
	case hasFile:
		return readLoopConfigFile(filePath)
	default:
		parsed, err := parseLoopConfigSetFlags(setFlags)
		if err != nil {
			return looppkg.LoopConfig{}, err
		}
		if parsed != nil {
			cfg = *parsed
		}
		return cfg, nil
	}
}

func parseLoopGateDecision(raw string) (looppkg.GateDecision, error) {
	decision := looppkg.GateDecision(strings.TrimSpace(raw))
	switch decision {
	case looppkg.GateDecisionApprove, looppkg.GateDecisionRequestChanges, looppkg.GateDecisionReject:
		return decision, nil
	default:
		return "", fmt.Errorf("cli: --decision must be approve, request_changes, or reject")
	}
}

func executeLoopRunAction(
	ctx context.Context,
	client loopCommandClient,
	verb string,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) error {
	switch strings.TrimSpace(verb) {
	case loopStopKey:
		return client.StopLoopRun(ctx, workspaceID, runID, credentials)
	case loopPauseKey:
		return client.PauseLoopRun(ctx, workspaceID, runID, credentials)
	case loopResumeKey:
		return client.ResumeLoopRun(ctx, workspaceID, runID, credentials)
	default:
		return fmt.Errorf("cli: unsupported loop run action %q", verb)
	}
}

func writeLoopDefinition(cmd *cobra.Command, response *contract.LoopResponse) error {
	if response == nil {
		return errors.New("cli: loop response is required")
	}
	return writeCommandOutput(
		cmd,
		loopOutputBundle(*response, fmt.Sprintf("Loop %s v%d", response.Loop.Name, response.Loop.Version)),
	)
}

func editLoopDefinition(
	cmd *cobra.Command,
	deps commandDeps,
	client loopCommandClient,
	workspaceID string,
	name string,
	editorOverride string,
) (response contract.LoopResponse, err error) {
	current, err := client.GetLoop(cmd.Context(), workspaceID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	body, err := yaml.Marshal(current.Loop.Definition)
	if err != nil {
		return contract.LoopResponse{}, fmt.Errorf("cli: marshal loop definition: %w", err)
	}
	tempFile, err := os.CreateTemp("", "agh-loop-*.yaml")
	if err != nil {
		return contract.LoopResponse{}, fmt.Errorf("cli: create temp loop definition: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
			err = fmt.Errorf("cli: remove temp loop definition: %w", removeErr)
		}
	}()
	if _, err := tempFile.Write(body); err != nil {
		closeErr := tempFile.Close()
		if closeErr != nil {
			return contract.LoopResponse{}, errors.Join(
				fmt.Errorf("cli: write temp loop definition: %w", err),
				fmt.Errorf("cli: close temp loop definition: %w", closeErr),
			)
		}
		return contract.LoopResponse{}, fmt.Errorf("cli: write temp loop definition: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return contract.LoopResponse{}, fmt.Errorf("cli: close temp loop definition: %w", err)
	}
	editor, err := loopEditorCommand(deps, editorOverride)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	editorCmd := exec.CommandContext(cmd.Context(), editor, tempPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = cmd.OutOrStdout()
	editorCmd.Stderr = cmd.ErrOrStderr()
	if err := editorCmd.Run(); err != nil {
		return contract.LoopResponse{}, fmt.Errorf("cli: run editor: %w", err)
	}
	next, err := readLoopDefinitionFile(tempPath)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	document, err := loopDefinitionDocumentFromDefinition(next)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	expected := current.Loop.Version
	return client.PatchLoop(cmd.Context(), workspaceID, name, contract.PatchLoopRequest{
		ExpectedVersion: &expected,
		Definition:      document,
	}, agentCredentialsFromEnv(deps))
}

func loopEditorCommand(deps commandDeps, override string) (string, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed, nil
	}
	if deps.getenv == nil {
		return "", errors.New("cli: EDITOR is required")
	}
	editor := strings.TrimSpace(deps.getenv("EDITOR"))
	if editor == "" {
		return "", errors.New("cli: EDITOR is required")
	}
	if deps.lookPath == nil {
		return editor, nil
	}
	resolved, err := deps.lookPath(editor)
	if err != nil {
		return "", fmt.Errorf("cli: resolve editor %q: %w", editor, err)
	}
	if strings.TrimSpace(resolved) == "" {
		return editor, nil
	}
	return resolved, nil
}

func loopOutputBundle(value any, summary string) outputBundle {
	return outputBundle{
		jsonValue: value,
		human: func() (string, error) {
			return strings.TrimSpace(summary), nil
		},
		toon: func() (string, error) {
			body, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("cli: marshal loop toon payload: %w", err)
			}
			return renderToonObject(loopLoopKey, []string{loopPayloadKey}, []string{string(body)}), nil
		},
	}
}

func writeLoopMutationOK(cmd *cobra.Command, verb string, id string) error {
	return writeCommandOutput(cmd, loopOutputBundle(
		map[string]any{"ok": true},
		fmt.Sprintf("Loop %s %s", strings.TrimSpace(id), strings.TrimSpace(verb)),
	))
}
