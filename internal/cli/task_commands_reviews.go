package cli

import (
	"errors"
	"fmt"

	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/spf13/cobra"
)

func newTaskReviewCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   taskReviewKey,
		Short: "Manage task-run reviews",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTaskReviewRequestCommand(deps))
	cmd.AddCommand(newTaskReviewListCommand(deps))
	cmd.AddCommand(newTaskReviewShowCommand(deps))
	cmd.AddCommand(newTaskReviewSubmitCommand(deps))
	return cmd
}

func newTaskReviewRequestCommand(deps commandDeps) *cobra.Command {
	var (
		reasonRaw string
		policyRaw string
		round     int
		attempt   int
		parentID  string
		asAgent   bool
	)
	cmd := &cobra.Command{
		Use:   "request <run-id>",
		Short: "Request review for a task run",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildTaskRunReviewRequest(
				args[0],
				reasonRaw,
				policyRaw,
				round,
				attempt,
				parentID,
			)
			if err != nil {
				return err
			}
			var review TaskRunReviewRequestRecord
			if asAgent {
				credentials, identityErr := requireAgentCommandIdentity(
					cmd.Context(),
					deps,
					client,
					agentActionCLI("task.review.request"),
				)
				if identityErr != nil {
					return identityErr
				}
				review, err = client.RequestTaskRunReviewAsAgent(
					cmd.Context(),
					args[0],
					request,
					credentials,
				)
			} else {
				review, err = client.RequestTaskRunReview(cmd.Context(), args[0], request)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunReviewRequestBundle(&review))
		},
	}
	cmd.Flags().StringVar(&reasonRaw, "reason", "", "Reason for requesting review")
	cmd.Flags().
		StringVar(&policyRaw, "policy", "", "Review policy: always, on_success, or on_failure")
	cmd.Flags().IntVar(&round, "round", 0, "Review round number")
	cmd.Flags().IntVar(&attempt, taskAttemptKey, 0, "Review attempt number")
	cmd.Flags().
		StringVar(&parentID, "parent-review", "", "Parent review ID for continuation rounds")
	cmd.Flags().
		BoolVar(&asAgent, "as-agent", false, "Request review using the current AGH-managed agent session identity")
	return cmd
}

func newTaskReviewListCommand(deps commandDeps) *cobra.Command {
	var (
		taskID            string
		runID             string
		statusRaw         string
		reviewerSessionID string
		last              int
	)
	cmd := &cobra.Command{
		Use:   taskListKey,
		Short: "List task-run reviews",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := parseTaskRunReviewListFilters(
				taskID,
				runID,
				statusRaw,
				reviewerSessionID,
				last,
			)
			if err != nil {
				return err
			}
			reviews, err := client.ListTaskRunReviews(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunReviewListBundle(reviews))
		},
	}
	cmd.Flags().StringVar(&taskID, taskTaskKey, "", "Filter by task ID")
	cmd.Flags().StringVar(&runID, "run", "", "Filter by task run ID")
	cmd.Flags().StringVar(&statusRaw, taskStatusKey, "", "Filter by review status")
	cmd.Flags().
		StringVar(&reviewerSessionID, "reviewer-session", "", "Filter by reviewer session ID")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N reviews")
	return cmd
}

func newTaskReviewShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <review-id>",
		Short: "Show one task-run review",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			review, err := client.GetTaskRunReview(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunReviewBundle(&review))
		},
	}
}

func newTaskReviewSubmitCommand(deps commandDeps) *cobra.Command {
	input := taskReviewSubmitInput{}
	var asAgent bool
	cmd := &cobra.Command{
		Use:   "submit <review-id>",
		Short: "Submit one task-run review verdict",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildTaskRunReviewVerdictRequest(args[0], input)
			if err != nil {
				return err
			}
			var result TaskRunReviewVerdictRecord
			if asAgent {
				credentials, identityErr := requireAgentCommandIdentity(
					cmd.Context(),
					deps,
					client,
					agentActionCLI("task.review.submit"),
				)
				if identityErr != nil {
					return identityErr
				}
				result, err = client.SubmitTaskRunReviewVerdictAsAgent(
					cmd.Context(),
					args[0],
					request,
					credentials,
				)
			} else {
				result, err = client.SubmitTaskRunReviewVerdict(cmd.Context(), args[0], request)
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskRunReviewVerdictBundle(&result))
		},
	}
	cmd.Flags().StringVar(&input.RunID, "run", "", "Task run ID")
	cmd.Flags().StringVar(&input.OutcomeRaw, cliOutcomeKey, "", "Verdict outcome")
	cmd.Flags().Float64Var(&input.Confidence, "confidence", 0, "Verdict confidence from 0 to 1")
	cmd.Flags().StringVar(&input.Reason, "reason", "", "Verdict reason")
	cmd.Flags().StringVar(&input.DeliveryID, "delivery-id", "", "Idempotent delivery ID")
	cmd.Flags().
		StringArrayVar(&input.MissingWork, "missing-work", nil, "Missing work item (repeatable)")
	cmd.Flags().StringVar(&input.MissingWorkRaw, "missing-work-json", "", "Missing work JSON array")
	cmd.Flags().
		StringVar(&input.NextRoundGuidance, "next-round-guidance", "", "Guidance for continuation rounds")
	cmd.Flags().StringVar(&input.ReviewText, "review-text", "", "Full review text")
	cmd.Flags().
		BoolVar(&asAgent, "as-agent", false, "Submit review using the current AGH-managed agent session identity")
	mustMarkFlagRequired(cmd, "run")
	mustMarkFlagRequired(cmd, cliOutcomeKey)
	mustMarkFlagRequired(cmd, "confidence")
	mustMarkFlagRequired(cmd, "reason")
	mustMarkFlagRequired(cmd, "delivery-id")
	return cmd
}

func buildTaskRunReviewRequest(
	runID string,
	reasonRaw string,
	policyRaw string,
	round int,
	attempt int,
	parentID string,
) (*TaskRunReviewRequest, error) {
	if round < 0 {
		return nil, errors.New("cli: --round must be zero or positive")
	}
	if attempt < 0 {
		return nil, errors.New("cli: --attempt must be zero or positive")
	}
	policy, err := parseOptionalReviewPolicy(policyRaw)
	if err != nil {
		return nil, err
	}
	request := TaskRunReviewRequest{
		RunID:          strings.TrimSpace(runID),
		ReviewRound:    round,
		Attempt:        attempt,
		Policy:         policy,
		ParentReviewID: strings.TrimSpace(parentID),
		Reason:         strings.TrimSpace(reasonRaw),
	}
	return &request, nil
}

func buildTaskRunReviewVerdictRequest(
	reviewID string,
	input taskReviewSubmitInput,
) (*TaskRunReviewVerdictRequest, error) {
	outcome, err := parseRequiredReviewOutcome(input.OutcomeRaw)
	if err != nil {
		return nil, err
	}
	confidence := input.Confidence
	missingWork, err := missingWorkFromFlags(input.MissingWork, input.MissingWorkRaw)
	if err != nil {
		return nil, err
	}
	request := &TaskRunReviewVerdictRequest{
		RunID: strings.TrimSpace(input.RunID),
		Verdict: taskpkg.RunReviewVerdict{
			Outcome:           outcome,
			Confidence:        &confidence,
			Reason:            strings.TrimSpace(input.Reason),
			DeliveryID:        strings.TrimSpace(input.DeliveryID),
			MissingWork:       missingWork,
			NextRoundGuidance: strings.TrimSpace(input.NextRoundGuidance),
			ReviewText:        strings.TrimSpace(input.ReviewText),
		},
	}
	recordReq := taskpkg.RecordRunReviewRequest{
		ReviewID: strings.TrimSpace(reviewID),
		RunID:    request.RunID,
		Verdict:  request.Verdict,
	}.Normalize()
	if err := recordReq.Validate("task_run_review_verdict"); err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}
	request.Verdict = recordReq.Verdict
	return request, nil
}

func parseTaskRunReviewListFilters(
	taskID string,
	runID string,
	statusRaw string,
	reviewerSessionID string,
	last int,
) (TaskRunReviewListQuery, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	trimmedRunID := strings.TrimSpace(runID)
	trimmedReviewerSessionID := strings.TrimSpace(reviewerSessionID)
	if trimmedTaskID != "" && trimmedRunID != "" {
		return TaskRunReviewListQuery{}, errors.New("cli: choose either --task or --run")
	}
	status, err := parseOptionalReviewStatus(statusRaw)
	if err != nil {
		return TaskRunReviewListQuery{}, err
	}
	if trimmedTaskID == "" && trimmedRunID == "" && status == "" &&
		trimmedReviewerSessionID == "" &&
		last == 0 {
		return TaskRunReviewListQuery{}, errors.New(
			"cli: task review list requires at least one filter",
		)
	}
	if err := validateTaskLast(last); err != nil {
		return TaskRunReviewListQuery{}, err
	}
	return TaskRunReviewListQuery{
		TaskID:            trimmedTaskID,
		RunID:             trimmedRunID,
		Status:            status,
		ReviewerSessionID: trimmedReviewerSessionID,
		Limit:             last,
	}, nil
}
