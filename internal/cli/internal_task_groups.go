package cli

import (
	"context"
	"encoding/json"
	"fmt"

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/spf13/cobra"
)

type internalTaskGroupCompletionCommandState struct {
	verificationPassed bool
	complete           completionCommand
}

type completionCommand func(
	context.Context,
	corepkg.TaskGroupCompletionRequest,
) (corepkg.TaskGroupCompletionResult, error)

func newInternalCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "internal",
		Short:        "Internal automation primitives",
		SilenceUsage: true,
		Hidden:       true,
	}
	command.AddCommand(newInternalTaskGroupsCommand())
	return command
}

func newInternalTaskGroupsCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "task-groups",
		Short:        "Internal Task Group operations",
		SilenceUsage: true,
		Hidden:       true,
	}
	command.AddCommand(newInternalTaskGroupCompleteCommand())
	return command
}

func newInternalTaskGroupCompleteCommand() *cobra.Command {
	state := &internalTaskGroupCompletionCommandState{complete: corepkg.CompleteTaskGroup}
	command := &cobra.Command{
		Use:          "complete <initiative>/TG-NNN",
		Short:        "Record a clean task group final-review result",
		SilenceUsage: true,
		Hidden:       true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return state.run(cmd, args[0])
		},
	}
	command.Flags().BoolVar(
		&state.verificationPassed,
		"verification-passed",
		false,
		"Confirmation from the final verification gate",
	)
	return command
}

func (s *internalTaskGroupCompletionCommandState) run(cmd *cobra.Command, reference string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()
	workspaceRoot, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return withExitCode(2, fmt.Errorf("discover completion workspace: %w", err))
	}
	complete := s.complete
	if complete == nil {
		complete = corepkg.CompleteTaskGroup
	}
	result, completeErr := complete(ctx, corepkg.TaskGroupCompletionRequest{
		WorkspaceRoot:      workspaceRoot,
		Reference:          reference,
		VerificationPassed: s.verificationPassed,
	})
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
		return withExitCode(2, fmt.Errorf("write task group completion result: %w", err))
	}
	if completeErr != nil {
		return withExitCode(1, completeErr)
	}
	return nil
}
