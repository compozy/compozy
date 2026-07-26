package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	looppkg "github.com/compozy/agh/internal/loop"
	toolspkg "github.com/compozy/agh/internal/tools"
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
}

type safeActionFailureTestError struct{}

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
