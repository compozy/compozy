package kinds

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestTaskRunMultiplePayloadFieldsDocumented guards docs/events.md against drift
// by asserting every JSON field of TaskRunMultiplePayload is documented. This
// keeps the public multi-run event reference aligned with the shipped payload,
// including the additive parallel_limit and worktree_* metadata fields.
func TestTaskRunMultiplePayloadFieldsDocumented(t *testing.T) {
	t.Parallel()
	t.Run("Should document every TaskRunMultiplePayload field", func(t *testing.T) {
		t.Parallel()

		content := readEventsDocumentation(t)
		payloadType := reflect.TypeOf(TaskRunMultiplePayload{})
		for i := range payloadType.NumField() {
			tag := jsonFieldName(payloadType.Field(i).Tag.Get("json"))
			if tag == "" || tag == "-" {
				continue
			}
			want := "`" + tag + "`"
			if !strings.Contains(content, want) {
				t.Fatalf("expected docs/events.md to document TaskRunMultiplePayload field %s", want)
			}
		}
	})
}

// TestTaskParallelPayloadFieldsDocumented guards docs/events.md against drift by
// asserting every JSON field of TaskParallelPayload is documented, keeping the
// public parallel-task event reference aligned with the shipped payload.
func TestTaskParallelPayloadFieldsDocumented(t *testing.T) {
	t.Parallel()
	t.Run("Should document every TaskParallelPayload field", func(t *testing.T) {
		t.Parallel()

		content := readEventsDocumentation(t)
		payloadType := reflect.TypeOf(TaskParallelPayload{})
		for i := range payloadType.NumField() {
			tag := jsonFieldName(payloadType.Field(i).Tag.Get("json"))
			if tag == "" || tag == "-" {
				continue
			}
			want := "`" + tag + "`"
			if !strings.Contains(content, want) {
				t.Fatalf("expected docs/events.md to document TaskParallelPayload field %s", want)
			}
		}
	})
}

func TestRunRecoveryPayloadFieldsDocumented(t *testing.T) {
	t.Parallel()

	payloads := []any{
		RunRecoveryStartedPayload{},
		RunRecoveryRestartingPayload{},
		RunRecoveredPayload{},
		RunRecoveryExhaustedPayload{},
	}
	content := readEventsDocumentation(t)
	for _, payload := range payloads {
		payloadType := reflect.TypeOf(payload)
		for i := range payloadType.NumField() {
			tag := jsonFieldName(payloadType.Field(i).Tag.Get("json"))
			if tag == "" || tag == "-" {
				continue
			}
			want := "`" + tag + "`"
			if !strings.Contains(content, want) {
				t.Fatalf("expected docs/events.md to document %s field %s", payloadType.Name(), want)
			}
		}
	}
}

func jsonFieldName(tag string) string {
	name, _, _ := strings.Cut(tag, ",")
	return strings.TrimSpace(name)
}

func readEventsDocumentation(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve docs test file path")
	}
	docsPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "docs", "events.md")
	content, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read %s: %v", docsPath, err)
	}
	return string(content)
}
