package cli

import (
	"testing"

	core "github.com/compozy/looper/internal/looper"
	"github.com/spf13/cobra"
)

func TestStartFormHidesSequentialOnlyFields(t *testing.T) {
	t.Parallel()

	keys := formFieldKeys(newStartCommand(), newCommandState(commandKindStart, core.ModePRDTasks))

	assertFieldKeysPresent(
		t,
		keys,
		"name",
		"tasks-dir",
		"ide",
		"model",
		"add-dir",
		"signal-port",
		"reasoning-effort",
		"timeout",
		"auto-commit",
	)
	assertFieldKeysAbsent(t, keys, "concurrent", "tail-lines", "dry-run", "include-completed")
}

func TestFixReviewsFormKeepsConcurrentButHidesUnneededFields(t *testing.T) {
	t.Parallel()

	keys := formFieldKeys(newFixReviewsCommand(), newCommandState(commandKindFixReviews, core.ModePRReview))

	assertFieldKeysPresent(
		t,
		keys,
		"name",
		"round",
		"reviews-dir",
		"concurrent",
		"batch-size",
		"grouped",
		"auto-commit",
		"ide",
		"model",
		"add-dir",
		"signal-port",
		"reasoning-effort",
		"timeout",
	)
	assertFieldKeysAbsent(t, keys, "tail-lines", "dry-run", "include-resolved")
}

func formFieldKeys(cmd *cobra.Command, state *commandState) map[string]struct{} {
	inputs := newFormInputs()
	builder := newFormBuilder(cmd, state)
	inputs.register(builder)

	keys := make(map[string]struct{}, len(builder.fields))
	for _, field := range builder.fields {
		key := field.GetKey()
		if key == "" {
			continue
		}
		keys[key] = struct{}{}
	}

	return keys
}

func assertFieldKeysPresent(t *testing.T, keys map[string]struct{}, want ...string) {
	t.Helper()

	for _, key := range want {
		if _, ok := keys[key]; !ok {
			t.Fatalf("expected form fields to include %q, got %#v", key, keys)
		}
	}
}

func assertFieldKeysAbsent(t *testing.T, keys map[string]struct{}, forbidden ...string) {
	t.Helper()

	for _, key := range forbidden {
		if _, ok := keys[key]; ok {
			t.Fatalf("expected form fields to omit %q, got %#v", key, keys)
		}
	}
}
