package cli

import (
	"errors"
	"fmt"

	"strings"

	"github.com/spf13/cobra"
)

func newTaskNextCommand(deps commandDeps) *cobra.Command {
	var (
		runID                string
		workspaceID          string
		requiredCapabilities []string
		priorityMin          int
		leaseSeconds         int64
		wait                 bool
		idempotencyKey       string
	)

	cmd := &cobra.Command{
		Use:   taskNextKey,
		Short: "Claim the next task run for the current agent session",
		Args:  cobra.NoArgs,
		Example: `  # Claim the next available run for this session
  agh task next

  # Wait until matching work is claimable and request a five-minute lease
  agh task next --wait --lease-seconds 300 -o json

  # Claim one exact queued run through the canonical lease path
  agh task next --run-id run-123 -o json

  # Filter by required caller capability
  agh task next --capability go.test --priority-min 10`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateAgentTaskLeaseSeconds(leaseSeconds); err != nil {
				return err
			}
			if cmd.Flags().Changed("run-id") && strings.TrimSpace(runID) == "" {
				return errors.New("cli: --run-id cannot be blank")
			}
			if priorityMin < 0 {
				return fmt.Errorf("cli: --priority-min must be zero or positive: %d", priorityMin)
			}
			request := AgentTaskClaimNextRequest{
				RunID:                strings.TrimSpace(runID),
				WorkspaceID:          strings.TrimSpace(workspaceID),
				RequiredCapabilities: trimAgentTaskCapabilities(requiredCapabilities),
				PriorityMin:          priorityMin,
				LeaseSeconds:         leaseSeconds,
				Wait:                 wait,
				IdempotencyKey:       strings.TrimSpace(idempotencyKey),
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(
				cmd.Context(),
				deps,
				client,
				agentActionCLI("task.next"),
			)
			if err != nil {
				return err
			}
			record, err := client.AgentTaskClaimNext(cmd.Context(), request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentTaskNextBundle(record))
		},
	}
	cmd.Flags().StringVar(&runID, "run-id", "", "Claim exactly this queued run")
	cmd.Flags().
		StringVar(&workspaceID, "workspace-id", "", "Workspace ID override; defaults to caller workspace")
	cmd.Flags().
		StringArrayVar(&requiredCapabilities, "capability", nil, "Caller capability filter (repeatable)")
	cmd.Flags().IntVar(&priorityMin, "priority-min", 0, "Minimum task priority")
	cmd.Flags().Int64Var(&leaseSeconds, "lease-seconds", 0, "Lease duration in seconds")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait until work is claimable")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newTaskHeartbeatCommand(deps commandDeps) *cobra.Command {
	var leaseSeconds int64

	cmd := &cobra.Command{
		Use:   "heartbeat <run-id>",
		Short: "Extend a claimed task run lease for the current agent session",
		Args:  cobra.ExactArgs(1),
		Example: `  # Extend the active session-bound lease
  agh task heartbeat run-123

	  # Request a specific lease duration
  agh task heartbeat run-123 --lease-seconds 300`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runID, err := requiredAgentTaskRunID(args[0])
			if err != nil {
				return err
			}
			if err := validateAgentTaskLeaseSeconds(leaseSeconds); err != nil {
				return err
			}
			request := AgentTaskHeartbeatRequest{
				LeaseSeconds: leaseSeconds,
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(
				cmd.Context(),
				deps,
				client,
				agentActionCLI("task.heartbeat"),
			)
			if err != nil {
				return err
			}
			record, err := client.AgentTaskHeartbeat(cmd.Context(), runID, request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentTaskLeaseBundle(record))
		},
	}
	cmd.Flags().Int64Var(&leaseSeconds, "lease-seconds", 0, "Lease duration in seconds")
	return cmd
}

func newTaskCompleteCommand(deps commandDeps) *cobra.Command {
	var resultRaw string

	cmd := &cobra.Command{
		Use:   "complete <run-id>",
		Short: "Complete a claimed task run for the current agent session",
		Args:  cobra.ExactArgs(1),
		Example: `  # Complete a claimed run
  agh task complete run-123

	  # Complete with structured result data
  agh task complete run-123 --result '{"summary":"tests passed"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runID, err := requiredAgentTaskRunID(args[0])
			if err != nil {
				return err
			}
			request := AgentTaskCompleteRequest{}
			if cmd.Flags().Changed("result") {
				request.Result, err = parseAgentTaskJSONFlag("result", resultRaw)
				if err != nil {
					return err
				}
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(
				cmd.Context(),
				deps,
				client,
				agentActionCLI("task.complete"),
			)
			if err != nil {
				return err
			}
			record, err := client.AgentTaskComplete(cmd.Context(), runID, request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentTaskLeaseBundle(record))
		},
	}
	cmd.Flags().StringVar(&resultRaw, "result", "", "Optional result JSON")
	return cmd
}

func newTaskFailCommand(deps commandDeps) *cobra.Command {
	flags := taskFailCommandFlags{}

	cmd := &cobra.Command{
		Use:   "fail <run-id> [run-id...]",
		Short: "Fail task runs through a session-bound lease or operator override",
		Args:  cobra.MinimumNArgs(1),
		Example: `  # Fail the current session's claimed run
  agh task fail run-123 --error "provider returned invalid JSON"

  # Force fail one run
  agh task fail run-123 --reason "operator recovery"

  # Force fail multiple runs with shared audit evidence
  agh task fail run-123 run-456 \
    --reason "provider credentials revoked" \
    --metadata '{"incident":"INC-42"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskFailCommand(cmd, args, deps, flags)
		},
	}
	cmd.Flags().StringVar(&flags.reason, "reason", "", "Forced-failure reason")
	cmd.Flags().StringVar(&flags.errorMessage, taskErrorKey, "", "Session-bound failure message")
	cmd.Flags().StringVar(&flags.metadataRaw, "metadata", "", "Optional failure metadata JSON")
	return cmd
}

type taskFailCommandFlags struct {
	reason       string
	errorMessage string
	metadataRaw  string
}

func runTaskFailCommand(
	cmd *cobra.Command,
	args []string,
	deps commandDeps,
	flags taskFailCommandFlags,
) error {
	runIDs, err := requiredTaskRunIDs(args)
	if err != nil {
		return err
	}
	if cmd.Flags().Changed(taskErrorKey) {
		return runSessionTaskFailCommand(cmd, deps, runIDs, flags)
	}
	return runForceTaskFailCommand(cmd, deps, runIDs, flags)
}

func runSessionTaskFailCommand(
	cmd *cobra.Command,
	deps commandDeps,
	runIDs []string,
	flags taskFailCommandFlags,
) error {
	if cmd.Flags().Changed("reason") {
		return errors.New(
			"cli: choose either --error for session-bound failure or --reason for force failure",
		)
	}
	if len(runIDs) != 1 {
		return errors.New("cli: --error supports exactly one run id")
	}
	request := AgentTaskFailRequest{Error: strings.TrimSpace(flags.errorMessage)}
	if request.Error == "" {
		return errors.New("cli: --error is required")
	}
	if cmd.Flags().Changed("metadata") {
		metadata, err := parseAgentTaskJSONFlag("metadata", flags.metadataRaw)
		if err != nil {
			return err
		}
		request.Metadata = metadata
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	credentials, err := requireAgentCommandIdentity(
		cmd.Context(),
		deps,
		client,
		agentActionCLI("task.fail"),
	)
	if err != nil {
		return err
	}
	record, err := client.AgentTaskFail(cmd.Context(), runIDs[0], request, credentials)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, agentTaskLeaseBundle(record))
}

func runForceTaskFailCommand(
	cmd *cobra.Command,
	deps commandDeps,
	runIDs []string,
	flags taskFailCommandFlags,
) error {
	request := ForceFailTaskRunRequest{
		Reason: strings.TrimSpace(flags.reason),
	}
	if request.Reason == "" {
		return errors.New("cli: --reason is required")
	}
	if cmd.Flags().Changed("metadata") {
		metadata, err := parseAgentTaskJSONFlag("metadata", flags.metadataRaw)
		if err != nil {
			return err
		}
		request.Metadata = metadata
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	if len(runIDs) == 1 {
		record, err := client.ForceFailTaskRun(cmd.Context(), runIDs[0], request)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, taskRunBundle(record))
	}
	record, err := client.BulkForceFailTaskRuns(cmd.Context(), BulkForceTaskRunRequest{
		RunIDs:   runIDs,
		Reason:   request.Reason,
		Metadata: request.Metadata,
	})
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, bulkForceTaskRunBundle(record))
}

func newTaskReleaseCommand(deps commandDeps) *cobra.Command {
	var reason string
	var metadataRaw string

	cmd := &cobra.Command{
		Use:   "release <run-id> [run-id...]",
		Short: "Force release claimed task runs back to the queue",
		Args:  cobra.MinimumNArgs(1),
		Example: `  # Release a claim without completing the run
  agh task release run-123

  # Release multiple claims with shared audit evidence
  agh task release run-123 run-456 --reason handoff`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runIDs, err := requiredTaskRunIDs(args)
			if err != nil {
				return err
			}
			request := ForceReleaseTaskRunRequest{
				Reason: strings.TrimSpace(reason),
			}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseAgentTaskJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if len(runIDs) == 1 {
				record, err := client.ForceReleaseTaskRun(cmd.Context(), runIDs[0], request)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, taskRunBundle(record))
			}
			record, err := client.BulkForceReleaseTaskRuns(cmd.Context(), BulkForceTaskRunRequest{
				RunIDs:   runIDs,
				Reason:   request.Reason,
				Metadata: request.Metadata,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bulkForceTaskRunBundle(record))
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Optional release reason")
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional release metadata JSON")
	return cmd
}

func newTaskRetryCommand(deps commandDeps) *cobra.Command {
	var metadataRaw string

	cmd := &cobra.Command{
		Use:   "retry <run-id>",
		Short: "Retry one failed task run",
		Args:  cobra.ExactArgs(1),
		Example: `  # Re-enqueue one failed run
  agh task retry run-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runIDs, err := requiredTaskRunIDs(args)
			if err != nil {
				return err
			}
			request := RetryTaskRunRequest{}
			if cmd.Flags().Changed("metadata") {
				request.Metadata, err = parseAgentTaskJSONFlag("metadata", metadataRaw)
				if err != nil {
					return err
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.RetryTaskRun(cmd.Context(), runIDs[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, retryTaskRunBundle(&record))
		},
	}
	cmd.Flags().StringVar(&metadataRaw, "metadata", "", "Optional retry metadata JSON")
	return cmd
}
