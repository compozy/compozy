package validate

import (
	"context"
	"strings"
	"testing"
	"time"
)

func assertErrorContains(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("expected error containing %q, got %q", fragment, err.Error())
	}
}

func TestValidateRequired(t *testing.T) {
	t.Run("nil value", func(t *testing.T) {
		err := Required(t.Context(), "name", nil)
		assertErrorContains(t, err, "name is required")
	})

	t.Run("empty string", func(t *testing.T) {
		err := Required(t.Context(), "title", "  ")
		assertErrorContains(t, err, "title cannot be empty")
	})

	t.Run("empty slice", func(t *testing.T) {
		values := []string{}
		err := Required(t.Context(), "items", values)
		assertErrorContains(t, err, "items cannot be empty")
	})

	t.Run("pointer dereference", func(t *testing.T) {
		value := "  "
		err := Required(t.Context(), "pointer", &value)
		assertErrorContains(t, err, "pointer cannot be empty")
	})

	t.Run("valid value", func(t *testing.T) {
		err := Required(t.Context(), "description", "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateID(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := ID(t.Context(), "")
		assertErrorContains(t, err, "id is required")
	})

	t.Run("invalid characters", func(t *testing.T) {
		err := ID(t.Context(), "invalid_id")
		assertErrorContains(t, err, "letters, numbers, or hyphens")
	})

	t.Run("valid", func(t *testing.T) {
		err := ID(t.Context(), "abc-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil context", func(t *testing.T) {
		var missingCtx context.Context
		err := ID(missingCtx, "abc-123")
		assertErrorContains(t, err, "context is required")
	})
}

func TestValidateNonEmpty(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := NonEmpty(t.Context(), "name", "\t")
		assertErrorContains(t, err, "name cannot be empty")
	})

	t.Run("valid", func(t *testing.T) {
		err := NonEmpty(t.Context(), "name", "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateURL(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		err := URL(t.Context(), "")
		assertErrorContains(t, err, "url is required")
	})

	t.Run("missing scheme", func(t *testing.T) {
		err := URL(t.Context(), "example.com/path")
		assertErrorContains(t, err, "must include a scheme")
	})

	t.Run("missing host", func(t *testing.T) {
		err := URL(t.Context(), "mailto:user@example.com")
		assertErrorContains(t, err, "must include a host")
	})

	t.Run("valid", func(t *testing.T) {
		err := URL(t.Context(), "https://example.com/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateDuration(t *testing.T) {
	t.Run("non positive", func(t *testing.T) {
		err := Duration(t.Context(), 0)
		assertErrorContains(t, err, "must be positive")
	})

	t.Run("valid", func(t *testing.T) {
		err := Duration(t.Context(), time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateRange(t *testing.T) {
	t.Run("invalid bounds", func(t *testing.T) {
		err := Range(t.Context(), "score", 5, 10, 1)
		assertErrorContains(t, err, "range is invalid")
	})

	t.Run("out of range", func(t *testing.T) {
		err := Range(t.Context(), "score", 11, 1, 10)
		assertErrorContains(t, err, "must be between 1 and 10")
	})

	t.Run("valid", func(t *testing.T) {
		err := Range(t.Context(), "score", 5, 1, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
