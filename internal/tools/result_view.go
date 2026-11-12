package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type ResultView string

const (
	ResultViewCompact        ResultView = "compact"
	ResultViewFull           ResultView = "full"
	ResultViewInputKey                  = "result_view"
	ResultViewSelectorSchema            = `{"type":"string","enum":["compact","full"]}`
)

type ResultViewContract struct {
	Schema json.RawMessage `json:"schema"`
	Digest string          `json:"digest,omitempty"`
}

type ResultViews struct {
	Compact ResultViewContract `json:"compact"`
	Full    ResultViewContract `json:"full"`
}

func CloneResultViews(src *ResultViews) *ResultViews {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Compact.Schema = cloneRawMessage(src.Compact.Schema)
	cloned.Full.Schema = cloneRawMessage(src.Full.Schema)
	return &cloned
}

func checkResultViewSelector(d Descriptor, input json.RawMessage) error {
	if d.Backend.Kind != BackendNativeGo {
		return nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil {
		return nil
	}
	raw, exists := fields[ResultViewInputKey]
	if !exists {
		return nil
	}
	message := ""
	if d.ResultViews() == nil {
		message = fmt.Sprintf("tool %q returns a single result and does not accept %q", d.ID, ResultViewInputKey)
	} else {
		var view string
		if json.Unmarshal(raw, &view) == nil && (view == string(ResultViewCompact) || view == string(ResultViewFull)) {
			return nil
		}
		message = fmt.Sprintf(
			"tool %q input %q must be one of %q, %q",
			d.ID,
			ResultViewInputKey,
			ResultViewCompact,
			ResultViewFull,
		)
	}
	code := d.ExecutionMetadata().InputErrorCode
	if code == "" {
		code = ErrorCodeInvalidInput
	}
	return NewToolError(code, d.ID, message, ErrToolInvalidInput, ReasonSchemaInvalid)
}

func selectedResultView(input json.RawMessage) ResultView {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) == nil &&
		bytes.Equal(bytes.TrimSpace(fields[ResultViewInputKey]), []byte(`"full"`)) {
		return ResultViewFull
	}
	// Decode escaped strings as well as the ordinary wire spelling.
	var view ResultView
	if json.Unmarshal(fields[ResultViewInputKey], &view) == nil && view == ResultViewFull {
		return view
	}
	return ResultViewCompact
}

func resolveResultView(d Descriptor, input json.RawMessage) (ResultView, json.RawMessage) {
	if d.Backend.Kind != BackendNativeGo || d.ResultViews() == nil {
		return "", input
	}
	return selectedResultView(input), ApprovalOperationInput(d, input)
}

func checkPinnedResultView(d Descriptor, req CallRequest) error {
	if d.Backend.Kind != BackendNativeGo || d.ResultViews() == nil || req.PinnedView == "" {
		return nil
	}
	if selectedResultView(req.Input) == req.PinnedView {
		return nil
	}
	return NewToolError(ErrorCodeDenied, d.ID,
		fmt.Sprintf("tool %q result view is pinned to %q by the caller", d.ID, req.PinnedView),
		ErrToolDenied, ReasonResultViewPinned)
}

// ApprovalOperationInput preserves member bytes and order because approval digests are not canonical JSON.
func ApprovalOperationInput(d Descriptor, input json.RawMessage) json.RawMessage {
	if d.Backend.Kind != BackendNativeGo || d.ResultViews() == nil || !json.Valid(input) {
		return input
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return input
	}
	result := []byte{'{'}
	found := false
	retained := false
	for decoder.More() {
		start := int(decoder.InputOffset())
		token, err = decoder.Token()
		if err != nil {
			return input
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return input
		}
		end := int(decoder.InputOffset())
		if token == ResultViewInputKey {
			found = true
			continue
		}
		member := bytes.TrimLeft(input[start:end], " \t\r\n,")
		if retained {
			result = append(result, ',')
		}
		result = append(result, member...)
		retained = true
	}
	if !found {
		return input
	}
	return append(result, '}')
}
