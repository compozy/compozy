package cli

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/spf13/cobra"
)

const (
	sessionContinueRouteFlag = "route"
	errContinueRouteRuntime  = "cli: --route cannot be combined with runtime flags " +
		"(--provider, --model, --reasoning-effort, --speed, --acp-option)"
	errDeriveFencesTogether = "cli: set all transcript fence flags together or omit all three"
)

var sessionContinueRuntimeFlags = []string{"provider", "model", "reasoning-effort", agentSpeedField, "acp-option"}

// SessionContinueRequest is the daemon continue request.
type SessionContinueRequest = contract.ContinueSessionRequest

// SessionDeriveRecord is the daemon continue/fork response.
type SessionDeriveRecord = contract.SessionDeriveResponse

// cliUsageError is a command-line usage error; the process exits with status 2.
type cliUsageError struct {
	message string
}

func (e *cliUsageError) Error() string { return e.message }

func (e *cliUsageError) cliExitCode() int { return 2 }

func newCLIUsageError(message string) error {
	return &cliUsageError{message: message}
}

type sessionContinueFlags struct {
	agent, provider, model, reasoningEffort, speed string
	acpOptions                                     []string
	route                                          int
	name, message, idempotencyKey                  string
	epoch, generation, maxSequence                 int64
}

func newSessionContinueCommand(deps commandDeps) *cobra.Command {
	var flags sessionContinueFlags
	cmd := &cobra.Command{
		Use:   "continue <session-id>",
		Short: "Continue a session with another agent, runtime, or declared route",
		Long: "Start a new session for another agent with the source conversation carried over as " +
			"historical context. The source session is never changed. Re-running with the same " +
			"--idempotency-key and arguments prints the recorded result instead of creating a second session.",
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildSessionContinueRequest(cmd, &flags)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if request.ExpectedEpoch == nil {
				if err := fillDeriveFencesFromTranscript(cmd.Context(), client, args[0],
					&request.ExpectedEpoch, &request.ExpectedGeneration, &request.ExpectedMaxSequence); err != nil {
					return err
				}
			}
			record, err := client.ContinueSession(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionDeriveBundle(&record))
		},
	}
	cmd.Flags().StringVar(&flags.agent, "agent", "", "Agent that continues the session (required)")
	cmd.Flags().StringVar(&flags.provider, "provider", "", "Provider for the new session's runtime")
	cmd.Flags().StringVar(&flags.model, "model", "", "Model for the new session's runtime")
	cmd.Flags().StringVar(&flags.reasoningEffort, "reasoning-effort", "", "Reasoning effort for the new session")
	cmd.Flags().StringVar(&flags.speed, agentSpeedField, "", "Speed for the new session (normal|fast)")
	cmd.Flags().StringArrayVar(&flags.acpOptions, "acp-option", nil, "ACP config option as id=value (repeatable)")
	cmd.Flags().IntVar(&flags.route, sessionContinueRouteFlag, 0,
		"1-based declared route of the agent's fallback_chain (excludes every runtime flag)")
	cmd.Flags().StringVar(&flags.name, "name", "", "Name of the new session (defaults to the source title)")
	cmd.Flags().StringVar(&flags.message, "message", "", "First message for the new session")
	addDeriveFenceFlags(cmd, &flags.epoch, &flags.generation, &flags.maxSequence)
	cmd.Flags().StringVar(&flags.idempotencyKey, "idempotency-key", "", "Stable retry key (generated when omitted)")
	return cmd
}

func addDeriveFenceFlags(cmd *cobra.Command, epoch, generation, maxSequence *int64) {
	cmd.Flags().Int64Var(epoch, "expected-epoch", 0, "Transcript epoch the source must still have")
	cmd.Flags().Int64Var(generation, "expected-generation", 0, "Transcript generation the source must still have")
	cmd.Flags().Int64Var(maxSequence, "expected-max-sequence", 0, "Maximum event sequence the source must still have")
}

func buildSessionContinueRequest(cmd *cobra.Command, flags *sessionContinueFlags) (SessionContinueRequest, error) {
	agent := strings.TrimSpace(flags.agent)
	if agent == "" {
		return SessionContinueRequest{}, newCLIUsageError("cli: --agent is required")
	}
	runtimeChanged := false
	for _, name := range sessionContinueRuntimeFlags {
		if cmd.Flags().Changed(name) {
			runtimeChanged = true
		}
	}
	if cmd.Flags().Changed(sessionContinueRouteFlag) && runtimeChanged {
		return SessionContinueRequest{}, newCLIUsageError(errContinueRouteRuntime)
	}
	if cmd.Flags().Changed(sessionContinueRouteFlag) && flags.route <= 0 {
		return SessionContinueRequest{}, newCLIUsageError("cli: --route must be a positive 1-based index")
	}
	request := SessionContinueRequest{
		AgentName: agent, Route: flags.route, Name: strings.TrimSpace(flags.name), Message: flags.message,
	}
	if runtimeChanged {
		runtime, err := sessionContinueRuntime(flags)
		if err != nil {
			return SessionContinueRequest{}, err
		}
		request.Runtime = runtime
	}
	if err := applyDeriveFenceFlags(cmd, flags.epoch, flags.generation, flags.maxSequence,
		&request.ExpectedEpoch, &request.ExpectedGeneration, &request.ExpectedMaxSequence); err != nil {
		return SessionContinueRequest{}, err
	}
	key, err := deriveIdempotencyKey(flags.idempotencyKey)
	if err != nil {
		return SessionContinueRequest{}, err
	}
	request.IdempotencyKey = key
	return request, nil
}

func sessionContinueRuntime(flags *sessionContinueFlags) (*contract.PromptRuntimeSelectionPayload, error) {
	runtime := &contract.PromptRuntimeSelectionPayload{
		Provider:        strings.TrimSpace(flags.provider),
		Model:           strings.TrimSpace(flags.model),
		ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(flags.reasoningEffort)),
		Speed:           contract.Speed(strings.TrimSpace(flags.speed)),
	}
	for _, raw := range flags.acpOptions {
		id, value, ok := strings.Cut(raw, "=")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(value) == "" {
			return nil, newCLIUsageError(fmt.Sprintf("cli: --acp-option must be id=value, got %q", raw))
		}
		runtime.ACPOptions = append(runtime.ACPOptions, contract.AgentACPOptionSelection{
			ID: strings.TrimSpace(id), ValueID: strings.TrimSpace(value),
		})
	}
	return runtime, nil
}

func applyDeriveFenceFlags(
	cmd *cobra.Command,
	epoch, generation, maxSequence int64,
	epochOut, generationOut, maxSequenceOut **int64,
) error {
	changed := 0
	for _, name := range []string{"expected-epoch", "expected-generation", "expected-max-sequence"} {
		if cmd.Flags().Changed(name) {
			changed++
		}
	}
	switch changed {
	case 0:
		return nil
	case 3:
		if epoch < 0 || generation < 0 || maxSequence < 0 {
			return newCLIUsageError("cli: transcript fences cannot be negative")
		}
		*epochOut, *generationOut, *maxSequenceOut = &epoch, &generation, &maxSequence
		return nil
	default:
		return newCLIUsageError(errDeriveFencesTogether)
	}
}

func deriveIdempotencyKey(value string) (string, error) {
	if key := strings.TrimSpace(value); key != "" {
		return key, nil
	}
	return store.NewID("idem")
}

// fillDeriveFencesFromTranscript reads the source transcript fences when the operator
// set none, so the derive refuses a transcript that moved in between.
func fillDeriveFencesFromTranscript(
	ctx context.Context,
	client sessionClientAPI,
	sessionID string,
	epochOut, generationOut, maxSequenceOut **int64,
) error {
	transcript, err := client.GetSessionTranscript(ctx, sessionID)
	if err != nil {
		return err
	}
	epoch, generation, maxSequence := transcript.Epoch, transcript.Generation, transcript.MaxSequence
	*epochOut, *generationOut, *maxSequenceOut = &epoch, &generation, &maxSequence
	return nil
}

func (c *daemonClient) ContinueSession(
	ctx context.Context,
	id string,
	request SessionContinueRequest,
) (SessionDeriveRecord, error) {
	var response SessionDeriveRecord
	path, err := c.sessionScopedPath(ctx, id, "/continue")
	if err != nil {
		return SessionDeriveRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return SessionDeriveRecord{}, err
	}
	return response, nil
}

func sessionDeriveBundle(record *SessionDeriveRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human:     func() (string, error) { return renderSessionDeriveHuman(record), nil },
		toon: func() (string, error) {
			derived := record.Derived
			return renderToonObject("session_derived", []string{
				"child_session_id", "kind", "source_session_id", "seed", "first_prompt", "replayed",
			}, []string{
				derived.ChildSessionID, derived.Kind, derived.SourceSessionID, derived.Seed,
				derived.FirstPrompt, strconv.FormatBool(derived.Replayed),
			}), nil
		},
	}
}

func renderSessionDeriveHuman(record *SessionDeriveRecord) string {
	derived := record.Derived
	childAgent := "deleted"
	if record.Session != nil {
		childAgent = stringOrDash(record.Session.AgentName)
	}
	headline := fmt.Sprintf("Continued %s (%s) into %s (%s)", derived.SourceSessionID,
		stringOrDash(derived.OriginAgentName), derived.ChildSessionID, childAgent)
	if derived.Kind == string(store.LineageKindFork) {
		headline = fmt.Sprintf("Forked %s into %s (%s)", derived.SourceSessionID, derived.ChildSessionID, childAgent)
	}
	lines := []keyValue{
		{Label: taskOriginValue, Value: deriveOriginValue(derived)},
		{Label: "Context", Value: deriveContextValue(derived)},
		{Label: "Seed", Value: deriveSeedValue(derived)},
		{Label: "First prompt", Value: stringOrDash(derived.FirstPrompt)},
	}
	if derived.Replayed {
		lines = append(lines, keyValue{Label: "Replayed", Value: "yes"})
	}
	if derived.ChildDeleted {
		lines = append(lines, keyValue{Label: "Child deleted", Value: "yes"})
	}
	var builder strings.Builder
	builder.WriteString(headline)
	for _, line := range lines {
		fmt.Fprintf(&builder, "\n  %-13s %s", line.Label, line.Value)
	}
	return builder.String()
}

func deriveOriginValue(derived contract.SessionDerivedPayload) string {
	parts := []string{derived.Kind, "from " + derived.SourceSessionID}
	if derived.OriginMessageID != "" {
		parts = append(parts, "through "+derived.OriginMessageID+" (turn "+derived.ThroughTurnID+")")
		return strings.Join(parts, " · ")
	}
	parts = append(parts, stringOrDash(derived.OriginAgentName))
	if derived.ThroughTurnID != "" {
		parts = append(parts, "through turn "+derived.ThroughTurnID)
	}
	return strings.Join(parts, " · ")
}

func deriveContextValue(derived contract.SessionDerivedPayload) string {
	count, size := 0, 0
	if derived.ReplayMessageCount != nil {
		count = *derived.ReplayMessageCount
	}
	if derived.ReplayBytes != nil {
		size = *derived.ReplayBytes
	}
	omitted := "nothing omitted"
	if derived.OmittedCount > 0 {
		omitted = strconv.Itoa(derived.OmittedCount) + " earlier messages omitted"
	}
	return fmt.Sprintf("%d messages · %.1f KiB · %s", count, float64(size)/1024, omitted)
}

func deriveSeedValue(derived contract.SessionDerivedPayload) string {
	value := stringOrDash(derived.Seed)
	if derived.NativeState != "" {
		value += " · " + derived.NativeState
	}
	if derived.NativeForkError != "" {
		value += " · " + derived.NativeForkError
	}
	return value
}

func sessionDerivationLines(info *SessionRecord) []keyValue {
	if info == nil {
		return nil
	}
	return derivationLines(info.Lineage, info.Derivation)
}

// derivationLines renders the Origin and Derivation lines of a continued or forked session.
func derivationLines(
	lineage *contract.SessionLineagePayload,
	derivation *contract.SessionDerivationPayload,
) []keyValue {
	if derivation == nil {
		return nil
	}
	origin := string(derivation.Kind) + " · from " + derivation.SourceSessionID
	if lineage != nil && lineage.OriginAgentName != "" {
		origin += " · " + lineage.OriginAgentName
	}
	detail := "seed " + stringOrDash(derivation.Seed)
	switch {
	case derivation.NativeState == "failed" && derivation.NativeForkError != "":
		detail += " · failed: " + derivation.NativeForkError + " (replayed the carried context)"
	case derivation.NativeState != "":
		detail += " · " + derivation.NativeState
	default:
		detail += " · first prompt " + stringOrDash(derivation.FirstPrompt)
	}
	return []keyValue{{Label: taskOriginValue, Value: origin}, {Label: "Derivation", Value: detail}}
}
