package cmdpalette

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	viewPatchOpAdd     = "add"
	viewPatchOpRemove  = "remove"
	viewPatchOpReplace = "replace"
	viewPatchOpTest    = "test"
)

// ApplyViewPatch applies an RFC 6902 patch only when its revision fence matches.
// The resync result is true for every fence gap so callers can request a full payload.
func ApplyViewPatch(
	kind ViewKind,
	currentRevision string,
	current ViewPayload,
	patch ViewPatch,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) (payload ViewPayload, revision string, resync bool, err error) {
	if patch.From != currentRevision {
		return current, currentRevision, true, &ViewRevisionMismatchError{
			Current: currentRevision,
			From:    patch.From,
		}
	}
	if err := ValidateViewPatch(patch); err != nil {
		return current, currentRevision, false, err
	}
	wire, err := json.Marshal(current)
	if err != nil {
		return current, currentRevision, false, fmt.Errorf("cmd palette view: encode patch base: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(wire))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return current, currentRevision, false, fmt.Errorf("cmd palette view: decode patch base: %w", err)
	}
	for index, operation := range patch.Ops {
		document, err = applyPatchOperation(document, operation)
		if err != nil {
			return current, currentRevision, false, viewValidationError(
				fmt.Sprintf("ops[%d]", index), "%v", err,
			)
		}
	}
	patchedWire, err := json.Marshal(document)
	if err != nil {
		return current, currentRevision, false, fmt.Errorf("cmd palette view: encode patched payload: %w", err)
	}
	var patched ViewPayload
	if err := json.Unmarshal(patchedWire, &patched); err != nil {
		return current, currentRevision, false, fmt.Errorf("cmd palette view: decode patched payload: %w", err)
	}
	validated, err := ValidateViewPayload(kind, patched, capabilities, reporter)
	if err != nil {
		return current, currentRevision, false, err
	}
	return validated, patch.To, false, nil
}

func ValidateViewPatch(patch ViewPatch) error {
	if strings.TrimSpace(patch.ViewID) == "" {
		return viewValidationError("view_id", "is required")
	}
	if strings.TrimSpace(patch.From) == "" {
		return viewValidationError("from", "is required")
	}
	if strings.TrimSpace(patch.To) == "" || patch.To == patch.From {
		return viewValidationError("to", "must be a new non-empty revision")
	}
	wire, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("cmd palette view: encode patch: %w", err)
	}
	if len(wire) > MaxViewWireBytes {
		return viewValidationError("patch", "is %d bytes; maximum is %d", len(wire), MaxViewWireBytes)
	}
	rowChanges := 0
	for index, operation := range patch.Ops {
		switch operation.Op {
		case viewPatchOpAdd, viewPatchOpRemove, viewPatchOpReplace, viewPatchOpTest:
		default:
			return viewValidationError(fmt.Sprintf("ops[%d].op", index), "unknown operation %q", operation.Op)
		}
		if _, err := parseJSONPointer(operation.Path); err != nil {
			return viewValidationError(fmt.Sprintf("ops[%d].path", index), "%v", err)
		}
		if strings.Contains(operation.Path, "/rows/") || strings.HasSuffix(operation.Path, "/rows") {
			rowChanges += patchValueRows(operation.Value)
		}
	}
	if rowChanges > MaxViewPatchRows {
		return viewValidationError("ops", "changes %d rows; maximum is %d", rowChanges, MaxViewPatchRows)
	}
	return nil
}

func patchValueRows(value json.RawMessage) int {
	if len(value) == 0 {
		return 1
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(value, &rows); err == nil {
		return max(1, len(rows))
	}
	return 1
}

func applyPatchOperation(document any, operation PatchOp) (any, error) {
	path, err := parseJSONPointer(operation.Path)
	if err != nil {
		return document, err
	}
	switch operation.Op {
	case viewPatchOpAdd:
		value, err := decodePatchValue(operation.Value)
		if err != nil {
			return document, err
		}
		return patchAdd(document, path, value)
	case viewPatchOpRemove:
		updated, err := patchRemove(document, path)
		return updated, err
	case viewPatchOpReplace:
		value, err := decodePatchValue(operation.Value)
		if err != nil {
			return document, err
		}
		if _, err := patchGet(document, path); err != nil {
			return document, err
		}
		updated, err := patchRemove(document, path)
		if err != nil {
			return document, err
		}
		return patchAdd(updated, path, value)
	case viewPatchOpTest:
		value, err := decodePatchValue(operation.Value)
		if err != nil {
			return document, err
		}
		actual, err := patchGet(document, path)
		if err != nil {
			return document, err
		}
		if !reflect.DeepEqual(actual, value) {
			return document, errors.New("test operation failed")
		}
		return document, nil
	default:
		return document, fmt.Errorf("unknown operation %q", operation.Op)
	}
}

func decodePatchValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, errors.New("operation value is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("invalid operation value: %w", err)
	}
	return value, nil
}

func parseJSONPointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("must be an RFC 6901 JSON pointer")
	}
	parts := strings.Split(pointer[1:], "/")
	for index, part := range parts {
		if strings.Contains(part, "~") {
			for cursor := 0; cursor < len(part); cursor++ {
				if part[cursor] == '~' && (cursor+1 >= len(part) || (part[cursor+1] != '0' && part[cursor+1] != '1')) {
					return nil, fmt.Errorf("contains invalid escape in segment %d", index)
				}
			}
		}
		parts[index] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func patchGet(document any, path []string) (any, error) {
	current := document
	for _, segment := range path {
		switch value := current.(type) {
		case map[string]any:
			var exists bool
			current, exists = value[segment]
			if !exists {
				return nil, fmt.Errorf("path member %q does not exist", segment)
			}
		case []any:
			index, err := patchArrayIndex(segment, len(value), false)
			if err != nil {
				return nil, err
			}
			current = value[index]
		default:
			return nil, fmt.Errorf("path traverses non-container at %q", segment)
		}
	}
	return current, nil
}

func patchAdd(document any, path []string, value any) (any, error) {
	if len(path) == 0 {
		return value, nil
	}
	return patchMutateParent(document, path, func(parent any, segment string) (any, error) {
		switch container := parent.(type) {
		case map[string]any:
			container[segment] = value
			return container, nil
		case []any:
			index, err := patchArrayIndex(segment, len(container), true)
			if err != nil {
				return nil, err
			}
			container = append(container, nil)
			copy(container[index+1:], container[index:])
			container[index] = value
			return container, nil
		default:
			return nil, errors.New("add target parent is not a container")
		}
	})
}

func patchRemove(document any, path []string) (any, error) {
	if len(path) == 0 {
		return nil, nil
	}
	updated, err := patchMutateParent(document, path, func(parent any, segment string) (any, error) {
		switch container := parent.(type) {
		case map[string]any:
			_, exists := container[segment]
			if !exists {
				return nil, fmt.Errorf("path member %q does not exist", segment)
			}
			delete(container, segment)
			return container, nil
		case []any:
			index, err := patchArrayIndex(segment, len(container), false)
			if err != nil {
				return nil, err
			}
			return append(container[:index], container[index+1:]...), nil
		default:
			return nil, errors.New("remove target parent is not a container")
		}
	})
	return updated, err
}

func patchMutateParent(
	document any,
	path []string,
	mutate func(parent any, segment string) (any, error),
) (any, error) {
	if len(path) == 1 {
		return mutate(document, path[0])
	}
	segment := path[0]
	switch container := document.(type) {
	case map[string]any:
		child, exists := container[segment]
		if !exists {
			return nil, fmt.Errorf("path member %q does not exist", segment)
		}
		updated, err := patchMutateParent(child, path[1:], mutate)
		if err != nil {
			return nil, err
		}
		container[segment] = updated
		return container, nil
	case []any:
		index, err := patchArrayIndex(segment, len(container), false)
		if err != nil {
			return nil, err
		}
		updated, err := patchMutateParent(container[index], path[1:], mutate)
		if err != nil {
			return nil, err
		}
		container[index] = updated
		return container, nil
	default:
		return nil, fmt.Errorf("path traverses non-container at %q", segment)
	}
}

func patchArrayIndex(segment string, length int, allowAppend bool) (int, error) {
	if allowAppend && segment == "-" {
		return length, nil
	}
	index, err := strconv.Atoi(segment)
	if err != nil || index < 0 || index > length || (!allowAppend && index == length) {
		return 0, fmt.Errorf("invalid array index %q", segment)
	}
	return index, nil
}
