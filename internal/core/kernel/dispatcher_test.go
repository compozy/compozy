package kernel

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/kernel/commands"
)

type runStartStubHandler struct {
	result commands.RunStartResult
	err    error
}

func (h runStartStubHandler) Handle(
	context.Context,
	commands.RunStartCommand,
) (commands.RunStartResult, error) {
	return h.result, h.err
}

type loopCommand struct {
	Value int
}

type loopResult struct {
	Value int
}

type loopHandler struct{}

func (loopHandler) Handle(_ context.Context, cmd loopCommand) (loopResult, error) {
	return loopResult(cmd), nil
}

func TestDispatchRoutesRegisteredHandler(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher()
	Register(dispatcher, runStartStubHandler{
		result: commands.RunStartResult{
			RunID:        "run-123",
			ArtifactsDir: "/tmp/run-123",
			Status:       runStartStatusSucceeded,
		},
	})

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{},
	)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if result.RunID != "run-123" {
		t.Fatalf("unexpected run id: %q", result.RunID)
	}
	if result.Status != runStartStatusSucceeded {
		t.Fatalf("unexpected status: %q", result.Status)
	}
}

func TestDispatchReturnsTypedErrorForUnregisteredCommand(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher()
	_, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		context.Background(),
		dispatcher,
		commands.ReviewsFetchCommand{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	var typedErr *UnregisteredHandlerError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected UnregisteredHandlerError, got %T", err)
	}
	if got := formatType(typedErr.CommandType); !strings.Contains(got, "commands.ReviewsFetchCommand") {
		t.Fatalf("unexpected command type: %q", got)
	}
}

func TestDispatchWithNilDispatcherReturnsTypedError(t *testing.T) {
	t.Parallel()

	_, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		context.Background(),
		nil,
		commands.ReviewsFetchCommand{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	var typedErr *UnregisteredHandlerError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected UnregisteredHandlerError, got %T", err)
	}
}

func TestDispatchReturnsTypedErrorForHandlerTypeMismatch(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher()
	Register(dispatcher, runStartStubHandler{
		result: commands.RunStartResult{Status: runStartStatusSucceeded},
	})

	_, err := Dispatch[commands.RunStartCommand, commands.WorkflowPrepareResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	var typedErr *HandlerTypeMismatchError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected HandlerTypeMismatchError, got %T", err)
	}
	if got := formatType(typedErr.CommandType); !strings.Contains(got, "commands.RunStartCommand") {
		t.Fatalf("unexpected command type: %q", got)
	}
	if got := formatType(typedErr.ResultType); !strings.Contains(got, "commands.WorkflowPrepareResult") {
		t.Fatalf("unexpected result type: %q", got)
	}
}

func TestRegisterIgnoresNilDispatcher(t *testing.T) {
	t.Parallel()

	Register[commands.RunStartCommand, commands.RunStartResult](nil, runStartStubHandler{})
}

func TestRegisterKeepsFirstHandlerForCommandType(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher()
	first := runStartStubHandler{
		result: commands.RunStartResult{RunID: "first", Status: runStartStatusSucceeded},
	}
	second := runStartStubHandler{
		result: commands.RunStartResult{RunID: "second", Status: runStartStatusSucceeded},
	}
	Register(dispatcher, first)
	Register(dispatcher, second)

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{},
	)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if result.RunID != "first" {
		t.Fatalf("expected first handler to remain registered, got %q", result.RunID)
	}
}

func TestTypedErrorMessagesNameCommandType(t *testing.T) {
	t.Parallel()

	unregistered := (&UnregisteredHandlerError{
		CommandType: reflect.TypeFor[commands.RunStartCommand](),
	}).Error()
	if !strings.Contains(unregistered, "commands.RunStartCommand") {
		t.Fatalf("unexpected unregistered error: %q", unregistered)
	}

	mismatch := (&HandlerTypeMismatchError{
		CommandType:       reflect.TypeFor[commands.RunStartCommand](),
		ResultType:        reflect.TypeFor[commands.WorkflowPrepareResult](),
		ActualHandlerType: reflect.TypeOf(runStartStubHandler{}),
	}).Error()
	if !strings.Contains(mismatch, "commands.RunStartCommand") {
		t.Fatalf("unexpected mismatch error: %q", mismatch)
	}
}

func TestDispatcherConcurrentRegisterAndDispatchIsRaceFree(t *testing.T) {
	dispatcher := NewDispatcher()
	Register(dispatcher, loopHandler{})

	var wg sync.WaitGroup
	for idx := 0; idx < 100; idx++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()

			if value%2 == 0 {
				Register(dispatcher, loopHandler{})
				return
			}

			result, err := Dispatch[loopCommand, loopResult](
				context.Background(),
				dispatcher,
				loopCommand{Value: value},
			)
			if err != nil {
				t.Errorf("dispatch %d: %v", value, err)
				return
			}
			if result.Value != value {
				t.Errorf("dispatch %d returned %d", value, result.Value)
			}
		}(idx)
	}
	wg.Wait()
}

func TestFormatTypeHandlesNil(t *testing.T) {
	t.Parallel()

	if got := formatType(nil); got != "<nil>" {
		t.Fatalf("unexpected nil type rendering: %q", got)
	}
}
