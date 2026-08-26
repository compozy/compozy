package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type callCreateFlags struct {
	workspace       string
	expect          string
	idleTTL         time.Duration
	deadline        time.Duration
	strict          bool
	resultBudget    string
	resultOverflow  string
	idempotencyKey  string
	runtime         string
	tools           []string
	skills          []string
	workspacePaths  []string
	networkChannels []string
}

func newCallCommand(deps commandDeps) *cobra.Command {
	flags := &callCreateFlags{}
	cmd := &cobra.Command{
		Use:   "call <agent-or-session-id> <prompt>",
		Short: "Call an agent and receive a durable result",
		Args:  cobra.ExactArgs(2),
		RunE:  runCallCreate(deps, flags),
	}
	addCallCreateFlags(cmd, flags)
	configureProfileMutationCommand(cmd, deps)
	cmd.AddCommand(
		newCallListCommand(deps),
		newCallShowCommand(deps),
		newCallAwaitCommand(deps),
		newCallCancelCommand(deps),
		newCallResultCommand(deps),
		newCallPublishCommand(deps),
	)
	return cmd
}

func addCallCreateFlags(cmd *cobra.Command, flags *callCreateFlags) {
	cmd.Flags().StringVar(&flags.workspace, "workspace", "", "Override workspace name or id; omit for global scope")
	cmd.Flags().StringVar(&flags.expect, "expect", "", "Result contract as inline JSON or @file")
	cmd.Flags().DurationVar(&flags.idleTTL, "idle-ttl", 0, "Parked child idle ceiling")
	cmd.Flags().DurationVar(&flags.deadline, "deadline", 0, "Optional call deadline")
	cmd.Flags().BoolVar(&flags.strict, "strict", false, "Require an explicit structured return")
	cmd.Flags().StringVar(&flags.resultBudget, "result-budget", "", "Per-call result budget, for example 512KiB")
	cmd.Flags().StringVar(&flags.resultOverflow, "result-overflow", "", "Over-budget policy: store or reject")
	cmd.Flags().StringVar(&flags.idempotencyKey, "idempotency-key", "", "Stable retry identity")
	cmd.Flags().StringVar(&flags.runtime, "runtime", "", "Runtime override: provider/model/reasoning/speed")
	cmd.Flags().StringSliceVar(&flags.tools, "tools", nil, "Allowed tool atoms")
	cmd.Flags().StringSliceVar(&flags.skills, "skills", nil, "Allowed skill atoms")
	cmd.Flags().StringSliceVar(&flags.workspacePaths, "workspace-paths", nil, "Allowed workspace paths")
	cmd.Flags().StringSliceVar(&flags.networkChannels, "network-channels", nil, "Allowed Network channels")
}

func runCallCreate(deps commandDeps, flags *callCreateFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		request, err := flags.request(args[0], args[1])
		if err != nil {
			return withCommandExitCode(2, err)
		}
		calls, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
		if err != nil {
			return err
		}
		record, err := calls.CreateCall(cmd.Context(), workspaceID, request)
		if err != nil {
			return withCallCommandExit(err)
		}
		return writeCommandOutput(cmd, callCreateBundle(record))
	}
}

func withCallCommandExit(err error) error {
	return withCommandExitCode(2, err)
}

func (flags *callCreateFlags) request(target, prompt string) (contract.CreateCallRequest, error) {
	item := contract.CreateCallItemRequest{
		Prompt: prompt, Strict: flags.strict, ResultBudget: strings.TrimSpace(flags.resultBudget),
		ResultOverflow: strings.TrimSpace(flags.resultOverflow), IdempotencyKey: strings.TrimSpace(flags.idempotencyKey),
		Narrow: contract.CallPermissionNarrowingRequest{
			Tools: cleanCallValues(flags.tools), Skills: cleanCallValues(flags.skills),
			WorkspacePaths: cleanCallValues(flags.workspacePaths), NetworkChannels: cleanCallValues(flags.networkChannels),
		},
	}
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "ses_") || strings.HasPrefix(target, "sess-") {
		item.Target.SessionID = target
	} else {
		item.Target.Agent = target
	}
	if strings.TrimSpace(prompt) == "" {
		return contract.CreateCallRequest{}, errors.New("cli: prompt is required")
	}
	if flags.idleTTL < 0 || flags.deadline < 0 {
		return contract.CreateCallRequest{}, errors.New("cli: idle-ttl and deadline must not be negative")
	}
	if flags.idleTTL > 0 {
		seconds := durationSecondsCeil(flags.idleTTL)
		item.IdleTTLSeconds = &seconds
	}
	if flags.deadline > 0 {
		seconds := durationSecondsCeil(flags.deadline)
		item.DeadlineSeconds = &seconds
	}
	expect, err := readCallExpect(flags.expect)
	if err != nil {
		return contract.CreateCallRequest{}, err
	}
	item.Expect = expect
	if raw := strings.TrimSpace(flags.runtime); raw != "" {
		parts := strings.Split(raw, "/")
		if len(parts) > 4 {
			return contract.CreateCallRequest{}, errors.New("cli: --runtime accepts provider/model/reasoning/speed")
		}
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		item.Runtime = &contract.CallRuntimeRequest{
			Provider: parts[0], Model: parts[1], ReasoningEffort: parts[2], Speed: parts[3],
		}
	}
	return contract.CreateCallRequest{CreateCallItemRequest: item}, nil
}

func readCallExpect(raw string) (json.RawMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	bytes := []byte(raw)
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(raw, "@"))
		if path == "" {
			return nil, errors.New("cli: --expect @file path is required")
		}
		var err error
		bytes, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cli: read --expect file %q: %w", path, err)
		}
	}
	if !json.Valid(bytes) {
		return nil, errors.New("cli: --expect must contain valid JSON")
	}
	return append(json.RawMessage(nil), bytes...), nil
}

func durationSecondsCeil(value time.Duration) int64 {
	return int64((value + time.Second - 1) / time.Second)
}

func cleanCallValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func resolveCallClient(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (callAPIClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	calls, ok := client.(callAPIClient)
	if !ok {
		return nil, "", errors.New("cli: daemon client does not support calls")
	}
	workspaceID := ""
	if strings.TrimSpace(workspaceRef) != "" {
		workspaceID, err = resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
		if err != nil {
			return nil, "", err
		}
	}
	return calls, workspaceID, nil
}
