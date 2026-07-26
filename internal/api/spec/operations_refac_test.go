package spec

import "testing"

func TestOperationsReturnDefensiveCopies(t *testing.T) {
	t.Run("Should isolate operation slices from registry mutations", func(t *testing.T) {
		t.Parallel()

		const (
			path   = "/api/automation/jobs"
			method = "GET"
		)
		operations := Operations()
		operation := operationSpecFor(t, operations, path, method)
		paramIndex := firstParameterWithEnum(t, operation)
		wantTag := operation.Tags[0]
		wantTransport := operation.Transports[0]
		wantParamName := operation.Parameters[paramIndex].Name
		wantParamEnum := operation.Parameters[paramIndex].Enum[0]
		wantResponseStatus := operation.Responses[0].Status

		operation.Tags[0] = "mutated-tag"
		operation.Transports[0] = Transport("mutated-transport")
		operation.Parameters[paramIndex].Name = "mutated-param"
		operation.Parameters[paramIndex].Enum[0] = "mutated-enum"
		operation.Responses[0].Status = 599

		fresh := operationSpecFor(t, Operations(), path, method)
		if got := fresh.Tags[0]; got != wantTag {
			t.Fatalf("Tags[0] = %q, want %q", got, wantTag)
		}
		if got := fresh.Transports[0]; got != wantTransport {
			t.Fatalf("Transports[0] = %q, want %q", got, wantTransport)
		}
		if got := fresh.Parameters[paramIndex].Name; got != wantParamName {
			t.Fatalf("Parameters[%d].Name = %q, want %q", paramIndex, got, wantParamName)
		}
		if got := fresh.Parameters[paramIndex].Enum[0]; got != wantParamEnum {
			t.Fatalf("Parameters[%d].Enum[0] = %q, want %q", paramIndex, got, wantParamEnum)
		}
		if got := fresh.Responses[0].Status; got != wantResponseStatus {
			t.Fatalf("Responses[0].Status = %d, want %d", got, wantResponseStatus)
		}
	})

	t.Run("Should isolate request body maps from registry mutations", func(t *testing.T) {
		t.Parallel()

		const (
			path       = "/api/webhooks/global/{endpoint}"
			method     = "POST"
			mutatedKey = "mutated"
		)
		operation := operationSpecFor(t, Operations(), path, method)
		body, ok := operation.RequestBody.(map[string]any)
		if !ok {
			t.Fatalf("RequestBody = %T, want map[string]any", operation.RequestBody)
		}

		body[mutatedKey] = "value"

		fresh := operationSpecFor(t, Operations(), path, method)
		freshBody, ok := fresh.RequestBody.(map[string]any)
		if !ok {
			t.Fatalf("fresh RequestBody = %T, want map[string]any", fresh.RequestBody)
		}
		if _, ok := freshBody[mutatedKey]; ok {
			t.Fatalf("fresh RequestBody contains %q after mutating returned copy", mutatedKey)
		}
	})

	t.Run("Should deep clone nested map and slice values", func(t *testing.T) {
		t.Parallel()

		original := map[string]any{
			"nested": map[string]any{
				"name": "stable",
			},
			"items": []any{
				map[string]any{"id": "item-1"},
			},
		}

		cloned, ok := cloneSpecValue(original).(map[string]any)
		if !ok {
			t.Fatalf("cloneSpecValue() = %T, want map[string]any", cloneSpecValue(original))
		}

		cloned["nested"].(map[string]any)["name"] = "mutated"
		cloned["items"].([]any)[0].(map[string]any)["id"] = "item-2"

		if got, want := original["nested"].(map[string]any)["name"], "stable"; got != want {
			t.Fatalf("original nested name = %v, want %v", got, want)
		}
		if got, want := original["items"].([]any)[0].(map[string]any)["id"], "item-1"; got != want {
			t.Fatalf("original nested slice item = %v, want %v", got, want)
		}
	})
}

func operationSpecFor(t *testing.T, operations []OperationSpec, path string, method string) OperationSpec {
	t.Helper()

	for _, operation := range operations {
		if operation.Path == path && operation.Method == method {
			return operation
		}
	}
	t.Fatalf("missing operation %s %s", method, path)
	return OperationSpec{}
}

func firstParameterWithEnum(t *testing.T, operation OperationSpec) int {
	t.Helper()

	for index, parameter := range operation.Parameters {
		if len(parameter.Enum) > 0 {
			return index
		}
	}
	t.Fatalf("operation %s %s has no enum parameter", operation.Method, operation.Path)
	return 0
}
