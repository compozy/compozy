package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type sessionGoalMutationFlags struct {
	expectedRunID string
	runtime       sessionPromptRuntimeFlags
}

func newSessionGoalCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Manage a session-origin Goal", Args: cobra.NoArgs}
	cmd.AddCommand(
		newSessionGoalSetCommand(deps),
		newSessionGoalReplaceCommand(deps),
		newSessionGoalStatusCommand(deps),
		newSessionGoalPauseCommand(deps),
		newSessionGoalResumeCommand(deps),
		newSessionGoalClearCommand(deps),
	)
	return cmd
}

func newSessionGoalSetCommand(deps commandDeps) *cobra.Command {
	flags := sessionGoalMutationFlags{}
	cmd := &cobra.Command{
		Use:   "set <id> <objective>",
		Short: "Start a Goal for a session",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := flags.runtime.selection(cmd)
			if err != nil {
				return err
			}
			return runSessionGoalMutation(cmd, deps, args[0], contract.SessionGoalCommandRequest{
				Operation: contract.SessionGoalOperationSet, Objective: args[1], Runtime: runtime,
			})
		},
	}
	bindSessionPromptRuntimeFlags(cmd, &flags.runtime)
	return cmd
}

func newSessionGoalReplaceCommand(deps commandDeps) *cobra.Command {
	flags := sessionGoalMutationFlags{}
	cmd := &cobra.Command{
		Use:   "replace <id> <objective>",
		Short: "Replace the expected active Goal",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			expectedRunID, err := changedNonEmptyStringFlag(cmd, "expected-run-id", flags.expectedRunID)
			if err != nil {
				return err
			}
			runtime, err := flags.runtime.selection(cmd)
			if err != nil {
				return err
			}
			return runSessionGoalMutation(cmd, deps, args[0], contract.SessionGoalCommandRequest{
				Operation: contract.SessionGoalOperationReplace, Objective: args[1],
				ExpectedRunID: expectedRunID, Runtime: runtime,
			})
		},
	}
	cmd.Flags().StringVar(&flags.expectedRunID, "expected-run-id", "", "Run id currently owned by the Goal")
	bindSessionPromptRuntimeFlags(cmd, &flags.runtime)
	return cmd
}

func newSessionGoalStatusCommand(deps commandDeps) *cobra.Command {
	return newSessionGoalSimpleCommand(deps, "status", "Read the current Goal", contract.SessionGoalOperationStatus)
}

func newSessionGoalPauseCommand(deps commandDeps) *cobra.Command {
	return newSessionGoalSimpleCommand(deps, "pause", "Pause the current Goal", contract.SessionGoalOperationPause)
}

func newSessionGoalResumeCommand(deps commandDeps) *cobra.Command {
	return newSessionGoalSimpleCommand(deps, "resume", "Resume the current Goal", contract.SessionGoalOperationResume)
}

func newSessionGoalClearCommand(deps commandDeps) *cobra.Command {
	return newSessionGoalSimpleCommand(deps, "clear", "Clear the current Goal", contract.SessionGoalOperationClear)
}

func newSessionGoalSimpleCommand(
	deps commandDeps,
	name string,
	short string,
	operation contract.SessionGoalOperation,
) *cobra.Command {
	return &cobra.Command{
		Use: name + " <id>", Args: exactOneNonBlankArg(), Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSessionGoalMutation(cmd, deps, args[0], contract.SessionGoalCommandRequest{Operation: operation})
		},
	}
}

func runSessionGoalMutation(
	cmd *cobra.Command,
	deps commandDeps,
	sessionID string,
	request contract.SessionGoalCommandRequest,
) error {
	if cmd == nil {
		return errors.New("cli: Goal command is required")
	}
	if err := request.Validate(); err != nil {
		return fmt.Errorf("cli: validate Goal command: %w", err)
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	result, err := client.MutateSessionGoal(cmd.Context(), strings.TrimSpace(sessionID), request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, sessionGoalBundle(result))
}

func sessionGoalBundle(result contract.GoalCommandResult) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, result) },
		human: func() (string, error) {
			return renderHumanSectionResult("Goal", sessionGoalRows(result))
		},
		toon: func() (string, error) {
			return renderToonObject("goal", sessionGoalFields(), sessionGoalValues(result)), nil
		},
	}
}

func sessionGoalRows(result contract.GoalCommandResult) []keyValue {
	rows := []keyValue{{Label: taskOutcomeValue, Value: stringOrDash(string(result.Outcome))}}
	if result.ReasonCode != nil {
		rows = append(rows, keyValue{Label: cliReasonValue, Value: string(*result.ReasonCode)})
	}
	if result.ReplacedRunID != nil {
		rows = append(rows, keyValue{Label: "Replaced Run", Value: *result.ReplacedRunID})
	}
	if result.Snapshot != nil {
		rows = append(rows,
			keyValue{Label: "Run", Value: result.Snapshot.RunID},
			keyValue{Label: sessionStatusValue, Value: string(result.Snapshot.Status)},
			keyValue{Label: "Objective", Value: result.Snapshot.Objective},
			keyValue{Label: cliTurnsValue, Value: fmt.Sprintf("%d/%d", result.Snapshot.TurnsUsed, result.Snapshot.TurnLimit)},
		)
	}
	return rows
}

func sessionGoalFields() []string {
	return []string{cliOutcomeKey, "reason_code", "replaced_run_id", agentKernelRunIDKey, sessionStatusKey, "objective", "turns_used", "turn_limit", goalLiveKey}
}

func sessionGoalValues(result contract.GoalCommandResult) []string {
	values := make([]string, len(sessionGoalFields()))
	values[0] = string(result.Outcome)
	if result.ReasonCode != nil {
		values[1] = string(*result.ReasonCode)
	}
	if result.ReplacedRunID != nil {
		values[2] = *result.ReplacedRunID
	}
	if result.Snapshot != nil {
		values[3] = result.Snapshot.RunID
		values[4] = string(result.Snapshot.Status)
		values[5] = result.Snapshot.Objective
		values[6] = fmt.Sprintf("%d", result.Snapshot.TurnsUsed)
		values[7] = fmt.Sprintf("%d", result.Snapshot.TurnLimit)
		values[8] = fmt.Sprintf("%t", result.Snapshot.Live)
	}
	return values
}
