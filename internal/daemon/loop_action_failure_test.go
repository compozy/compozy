package daemon

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestLoopActionFailureMetadataShouldPreserveSafeOperatorDetail(t *testing.T) {
	t.Parallel()

	t.Run("Should redact and preserve a typed tool failure", func(t *testing.T) {
		t.Parallel()

		cause := toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeInvalidInput,
			"ext__dev_cycle__import_tasks",
			"task set missing",
			toolspkg.ErrToolInvalidInput,
			"No task set matched .compozy/tasks/launch/task_*.md; api_key=secret-value.",
			"Create the matching task set, then retry the run.",
			toolspkg.ReasonDependencyMissing,
		)
		metadata, err := marshalLoopActionFailureMetadata("task_run_enqueued", cause)
		if err != nil {
			t.Fatalf("marshalLoopActionFailureMetadata() error = %v", err)
		}
		var envelope loopActionFailureMetadata
		if err := json.Unmarshal(metadata, &envelope); err != nil {
			t.Fatalf("Unmarshal(loop action failure metadata) error = %v", err)
		}
		if envelope.Failure.Code != string(toolspkg.ErrorCodeInvalidInput) ||
			envelope.Failure.Kind != "action_failure" ||
			envelope.Failure.Recovery != "Create the matching task set, then retry the run." {
			t.Fatalf("failure metadata = %#v, want typed operator detail", envelope.Failure)
		}
		if strings.Contains(envelope.Failure.Cause, "secret-value") ||
			!strings.Contains(envelope.Failure.Cause, "[REDACTED]") {
			t.Fatalf("failure cause = %q, want secret redacted", envelope.Failure.Cause)
		}
		if _, ok := looppkg.ActionFailureOutputRefFromMetadata(metadata); !ok {
			t.Fatalf("ActionFailureOutputRefFromMetadata(%s) = false, want valid payload", metadata)
		}
	})

	t.Run("Should preserve a domain-owned safe action failure", func(t *testing.T) {
		t.Parallel()

		metadata, err := marshalLoopActionFailureMetadata("goal_action", safeActionFailureTestError{})
		if err != nil {
			t.Fatalf("marshalLoopActionFailureMetadata() error = %v", err)
		}
		var envelope loopActionFailureMetadata
		if err := json.Unmarshal(metadata, &envelope); err != nil {
			t.Fatalf("Unmarshal(loop action failure metadata) error = %v", err)
		}
		if envelope.Failure.Code != "goal_control_revoked_in_flight" ||
			envelope.Failure.Cause != "Goal control revoked the in-flight turn." ||
			envelope.Failure.Recovery != "Start a new Goal to continue the objective." {
			t.Fatalf("failure metadata = %#v, want domain-owned safe detail", envelope.Failure)
		}
	})

	t.Run("Should preserve the classified attempt timeout code", func(t *testing.T) {
		t.Parallel()

		cause := &looppkg.ReasonError{
			Code: looppkg.ReasonCodeActionTimeout,
			Err:  looppkg.ErrActionTimeout,
		}
		metadata, err := marshalLoopActionFailureMetadata("loop_action", cause)
		if err != nil {
			t.Fatalf("marshalLoopActionFailureMetadata() error = %v", err)
		}
		var envelope loopActionFailureMetadata
		if err := json.Unmarshal(metadata, &envelope); err != nil {
			t.Fatalf("Unmarshal(loop action failure metadata) error = %v", err)
		}
		if envelope.ReasonCode != string(looppkg.ReasonCodeActionTimeout) ||
			envelope.Failure.Code != string(looppkg.ReasonCodeActionTimeout) {
			t.Fatalf("failure metadata = %#v, want attempt timeout classification", envelope)
		}
	})

	t.Run("Should retain safe defaults when providers omit fields", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			cause error
		}{
			{
				name:  "Should retain the default code for an empty reason code",
				cause: &looppkg.ReasonError{Err: errors.New("runtime contract failed")},
			},
			{
				name:  "Should retain default recovery for a partial safe failure",
				cause: partialSafeActionFailureTestError{},
			},
			{
				name: "Should retain default operator text for an empty tool detail",
				cause: toolspkg.NewOperatorToolError(
					toolspkg.ErrorCodeUnavailable,
					"ext__partial",
					"",
					errors.New("unavailable"),
					"",
					"",
				),
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				failure := operatorSafeActionFailure(testCase.cause)
				if strings.TrimSpace(failure.Code) == "" || strings.TrimSpace(failure.Cause) == "" ||
					strings.TrimSpace(failure.Recovery) == "" {
					t.Fatalf("operatorSafeActionFailure() = %#v, want complete safe defaults", failure)
				}
			})
		}
	})
}

type safeActionFailureTestError struct{}

type partialSafeActionFailureTestError struct{}

func (partialSafeActionFailureTestError) Error() string {
	return "partial safe failure"
}

func (partialSafeActionFailureTestError) SafeActionFailure() looppkg.ActionFailure {
	return looppkg.NewActionFailure("partial_safe", "Safe partial failure.", "")
}

func (safeActionFailureTestError) Error() string {
	return "internal state api_key=secret-value"
}

func (safeActionFailureTestError) SafeActionFailure() looppkg.ActionFailure {
	return looppkg.NewActionFailure(
		"goal_control_revoked_in_flight",
		"Goal control revoked the in-flight turn.",
		"Start a new Goal to continue the objective.",
	)
}
