package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

type typedError struct {
	code string
}

func (e *typedError) Error() string {
	return fmt.Sprintf("error code: %s", e.code)
}

func TestBuildErrorErrorSingle(t *testing.T) {
	target := stderrors.New("missing workflow id")
	buildErr := &BuildError{Errors: []error{target}}

	got := buildErr.Error()
	want := "build failed: missing workflow id"

	if got != want {
		t.Fatalf("unexpected error message\nwant: %q\n got: %q", want, got)
	}
}

func TestBuildErrorErrorMultiple(t *testing.T) {
	buildErr := &BuildError{Errors: []error{
		fmt.Errorf("missing workflow id"),
		fmt.Errorf("agent must define at least one action"),
		fmt.Errorf("runtime is required"),
	}}

	got := buildErr.Error()
	want := "build failed with 3 errors:\n  1. missing workflow id\n  2. agent must define at least one action\n  3. runtime is required"

	if got != want {
		t.Fatalf("unexpected error message\nwant: %q\n got: %q", want, got)
	}
}

func TestBuildErrorErrorEmpty(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var buildErr *BuildError

		if msg := buildErr.Error(); msg != "build failed" {
			t.Fatalf("expected default message, got %q", msg)
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		buildErr := &BuildError{}

		if msg := buildErr.Error(); msg != "build failed" {
			t.Fatalf("expected default message, got %q", msg)
		}
	})
}

func TestBuildErrorUnwrapReturnsFirstError(t *testing.T) {
	first := fmt.Errorf("first failure")
	second := fmt.Errorf("second failure")
	buildErr := &BuildError{Errors: []error{first, second}}

	unwrapped := stderrors.Unwrap(buildErr)
	if unwrapped != first {
		t.Fatalf("expected first error, got %#v", unwrapped)
	}
}

func TestBuildErrorUnwrapNilCases(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var buildErr *BuildError
		if stderrors.Unwrap(buildErr) != nil {
			t.Fatal("expected unwrap of nil receiver to return nil")
		}
	})

	t.Run("empty", func(t *testing.T) {
		buildErr := &BuildError{}
		if stderrors.Unwrap(buildErr) != nil {
			t.Fatal("expected unwrap of empty BuildError to return nil")
		}
	})
}

func TestBuildErrorErrorsIs(t *testing.T) {
	sentinel := stderrors.New("sentinel")
	wrapped := fmt.Errorf("wrap: %w", sentinel)
	other := stderrors.New("other")
	buildErr := &BuildError{Errors: []error{wrapped, other}}

	if !stderrors.Is(buildErr, sentinel) {
		t.Fatal("expected errors.Is to match sentinel error")
	}

	if !stderrors.Is(buildErr, other) {
		t.Fatal("expected errors.Is to match other error")
	}
}

func TestBuildErrorErrorsIsNilReceiver(t *testing.T) {
	var buildErr *BuildError
	if stderrors.Is(buildErr, fmt.Errorf("anything")) {
		t.Fatal("expected nil BuildError to not match target")
	}
}

func TestBuildErrorErrorsIsNoMatch(t *testing.T) {
	buildErr := &BuildError{Errors: []error{nil}}
	if stderrors.Is(buildErr, fmt.Errorf("missing")) {
		t.Fatal("expected errors.Is to return false when no matches exist")
	}
}

func TestBuildErrorErrorsAs(t *testing.T) {
	custom := &typedError{code: "INVALID_AGENT"}
	wrapped := fmt.Errorf("wrap: %w", custom)
	buildErr := &BuildError{Errors: []error{wrapped}}

	var target *typedError
	if !stderrors.As(buildErr, &target) {
		t.Fatal("expected errors.As to project custom error")
	}

	if target != custom {
		t.Fatalf("expected target to be %v, got %v", custom, target)
	}
}

func TestBuildErrorErrorsAsNilReceiver(t *testing.T) {
	var buildErr *BuildError
	var target *typedError
	if stderrors.As(buildErr, &target) {
		t.Fatal("expected errors.As to return false for nil BuildError")
	}
}

func TestBuildErrorErrorsAsNoMatch(t *testing.T) {
	buildErr := &BuildError{Errors: []error{fmt.Errorf("wrapped")}}
	var target *typedError
	if stderrors.As(buildErr, &target) {
		t.Fatal("expected errors.As to return false when no target matches")
	}
}

func TestBuildErrorNonNilErrors(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var buildErr *BuildError
		if buildErr.nonNilErrors() != nil {
			t.Fatal("expected nil receiver to return nil slice")
		}
	})

	t.Run("filters nils", func(t *testing.T) {
		first := fmt.Errorf("first")
		buildErr := &BuildError{Errors: []error{nil, first, nil}}
		filtered := buildErr.nonNilErrors()
		if len(filtered) != 1 {
			t.Fatalf("expected one error, got %d", len(filtered))
		}
		if filtered[0] != first {
			t.Fatalf("expected first error, got %v", filtered[0])
		}
	})
}

func TestBuildErrorMessageClarity(t *testing.T) {
	buildErr := &BuildError{Errors: []error{nil, fmt.Errorf("workflow identifier missing")}}

	msg := buildErr.Error()
	if msg != "build failed: workflow identifier missing" {
		t.Fatalf("unexpected message: %q", msg)
	}

	if stderrors.Is(buildErr, nil) {
		t.Fatal("errors.Is should not match nil")
	}
}

func ExampleBuildError() {
	buildErr := &BuildError{Errors: []error{
		fmt.Errorf("workflow id is required"),
		fmt.Errorf("agent must include at least one action"),
	}}

	fmt.Println(buildErr.Error())
	// Output:
	// build failed with 2 errors:
	//   1. workflow id is required
	//   2. agent must include at least one action
}
