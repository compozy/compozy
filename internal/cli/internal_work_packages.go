package cli

import (
	"context"
	"encoding/json"
	"fmt"

	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/spf13/cobra"
)

type internalWorkPackageCompletionCommandState struct {
	verificationPassed bool
	complete           completionCommand
}

type completionCommand func(
	context.Context,
	corepkg.WorkPackageCompletionRequest,
) (corepkg.WorkPackageCompletionResult, error)

func newInternalCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "internal",
		Short:        "Internal automation primitives",
		SilenceUsage: true,
		Hidden:       true,
	}
	command.AddCommand(newInternalWorkPackagesCommand())
	return command
}

func newInternalWorkPackagesCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "work-packages",
		Short:        "Internal Work Package operations",
		SilenceUsage: true,
		Hidden:       true,
	}
	command.AddCommand(newInternalWorkPackageCompleteCommand())
	return command
}

func newInternalWorkPackageCompleteCommand() *cobra.Command {
	state := &internalWorkPackageCompletionCommandState{complete: corepkg.CompleteWorkPackage}
	command := &cobra.Command{
		Use:          "complete <initiative>/WP-NNN",
		Short:        "Record a clean package final-review result",
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

func (s *internalWorkPackageCompletionCommandState) run(cmd *cobra.Command, reference string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()
	workspaceRoot, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return withExitCode(2, fmt.Errorf("discover completion workspace: %w", err))
	}
	complete := s.complete
	if complete == nil {
		complete = corepkg.CompleteWorkPackage
	}
	result, completeErr := complete(ctx, corepkg.WorkPackageCompletionRequest{
		WorkspaceRoot:      workspaceRoot,
		Reference:          reference,
		VerificationPassed: s.verificationPassed,
	})
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
		return withExitCode(2, fmt.Errorf("write work package completion result: %w", err))
	}
	if completeErr != nil {
		return withExitCode(1, completeErr)
	}
	return nil
}
