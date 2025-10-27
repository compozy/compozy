package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestNewTestContextProvidesLoggerAndConfig(t *testing.T) {
	t.Parallel()
	ctx := NewTestContext(t)
	if ctx.Done() == nil {
		t.Fatalf("expected context with cancellation support")
	}
	if logger.FromContext(ctx) == nil {
		t.Fatalf("expected logger in context")
	}
	if config.FromContext(ctx) == nil {
		t.Fatalf("expected configuration in context")
	}
}

func TestRequireNoError(t *testing.T) {
	RequireNoError(t, nil)
	prev := reportFailure
	called := false
	var message string
	reportFailure = func(tt *testing.T, format string, args ...any) {
		called = true
		message = fmt.Sprintf(format, args...)
	}
	t.Cleanup(func() {
		reportFailure = prev
	})
	RequireNoError(t, fmt.Errorf("boom"))
	if !called {
		t.Fatalf("expected failure handler to be invoked")
	}
	if !strings.Contains(message, "unexpected error") {
		t.Fatalf("expected unexpected error message, got %s", message)
	}
}

func TestRequireValidationError(t *testing.T) {
	inner := fmt.Errorf("invalid value for field")
	be := &sdkerrors.BuildError{Errors: []error{inner}}
	RequireValidationError(t, be, "field")
	prev := reportFailure
	called := false
	var message string
	reportFailure = func(tt *testing.T, format string, args ...any) {
		called = true
		message = fmt.Sprintf(format, args...)
	}
	t.Cleanup(func() {
		reportFailure = prev
	})
	RequireValidationError(t, nil, "")
	if !called {
		t.Fatalf("expected validation failure handler to run")
	}
	if !strings.Contains(message, "expected validation error") {
		t.Fatalf("expected validation error message, got %s", message)
	}
}

func TestAssertBuildError(t *testing.T) {
	t.Parallel()
	be := &sdkerrors.BuildError{Errors: []error{fmt.Errorf("missing id"), fmt.Errorf("invalid name")}}
	AssertBuildError(t, be, []string{"missing", "invalid"})
}

func TestNewTestModelDefaults(t *testing.T) {
	t.Parallel()
	model := NewTestModel("", "")
	if model.Provider != enginecore.ProviderOpenAI {
		t.Fatalf("expected provider openai, got %s", model.Provider)
	}
	if model.Model == "" {
		t.Fatalf("expected non-empty model id")
	}
	if !strings.Contains(model.APIKey, "TEST_API_KEY") {
		t.Fatalf("expected api key placeholder, got %s", model.APIKey)
	}
}

func TestNewTestAgent(t *testing.T) {
	t.Parallel()
	agent := NewTestAgent("example-agent")
	if agent.ID != "example-agent" {
		t.Fatalf("expected id example-agent, got %s", agent.ID)
	}
	if strings.TrimSpace(agent.Instructions) == "" {
		t.Fatalf("expected default instructions")
	}
	if agent.Model.Config.Model == "" {
		t.Fatalf("expected inline model configuration")
	}
}

func TestNewTestWorkflow(t *testing.T) {
	t.Parallel()
	wf := NewTestWorkflow("workflow")
	if wf.ID != "workflow" {
		t.Fatalf("expected workflow id, got %s", wf.ID)
	}
	if len(wf.Agents) != 1 {
		t.Fatalf("expected single agent")
	}
	if len(wf.Tasks) != 1 {
		t.Fatalf("expected single task")
	}
	if wf.Tasks[0].Agent == nil {
		t.Fatalf("expected task to reference agent")
	}
	if wf.Tasks[0].Agent.ID == "" {
		t.Fatalf("expected agent id on task")
	}
	ctx := NewTestContext(t)
	if err := wf.Validate(ctx); err != nil {
		t.Fatalf("expected workflow validation to pass: %v", err)
	}
	if wf.Tasks[0].Agent.ID != wf.Agents[0].ID {
		t.Fatalf("expected task agent id to match workflow agent id")
	}
}

func TestRunTableTests(t *testing.T) {
	t.Parallel()
	executions := make([]string, 0, 2)
	table := []TableTest{
		{
			Name: "ok",
			BuildFunc: func(ctx context.Context) (any, error) {
				if logger.FromContext(ctx) == nil {
					return nil, errors.New("logger missing")
				}
				executions = append(executions, "ok")
				return "value", nil
			},
			Validate: func(t *testing.T, v any) {
				t.Helper()
				str, ok := v.(string)
				if !ok {
					t.Fatalf("expected string value, got %T", v)
				}
				if str != "value" {
					t.Fatalf("unexpected result %s", str)
				}
			},
		},
		{
			Name:        "err",
			WantErr:     true,
			ErrContains: "boom",
			BuildFunc: func(ctx context.Context) (any, error) {
				executions = append(executions, "err")
				return nil, fmt.Errorf("boom failure")
			},
		},
	}
	RunTableTests(t, table)
	if len(executions) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(executions))
	}
}

func TestAssertConfigEqual(t *testing.T) {
	t.Parallel()
	want := map[string]any{"k": "v"}
	got := map[string]any{"k": "v"}
	AssertConfigEqual(t, want, got)
}

func TestTestDataHelpers(t *testing.T) {
	t.Parallel()
	name := fmt.Sprintf("temp-%d.txt", time.Now().UnixNano())
	path := TestDataPath(t, name)
	data := []byte("sample")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write testdata: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path)
	})
	read := ReadTestData(t, name)
	if string(read) != "sample" {
		t.Fatalf("expected to read sample, got %s", string(read))
	}
}
