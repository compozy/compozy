package cmdpalette

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	viewPatchRootPath   = ""
	viewPatchRootSlash  = "/"
	viewPatchChromePath = "/chrome"
)

// updateViewEventHandlerRoles records semantic handler roles from the current
// frame. Admission uses these declared roles instead of guessing from event
// arguments, because controlled selection events also carry numeric counters.
func updateViewEventHandlerRoles(session *viewSession, frame ViewFrame) error {
	if frame.Payload != nil {
		replaceViewEventHandlerRoles(session, frame.Payload.Chrome)
		return nil
	}
	if frame.Patch == nil {
		return nil
	}
	for _, operation := range frame.Patch.Ops {
		chrome, found, err := chromeFromPatchOperation(operation)
		if err != nil {
			return err
		}
		if found {
			replaceViewEventHandlerRoles(session, chrome)
		}
	}
	return nil
}

func chromeFromPatchOperation(operation PatchOp) (*ViewChrome, bool, error) {
	if operation.Op != viewPatchOpAdd && operation.Op != viewPatchOpReplace {
		return nil, false, nil
	}
	switch operation.Path {
	case viewPatchRootPath, viewPatchRootSlash:
		var payload ViewPayload
		if err := json.Unmarshal(operation.Value, &payload); err != nil {
			return nil, false, fmt.Errorf("decode root replacement payload: %w", err)
		}
		return payload.Chrome, true, nil
	case viewPatchChromePath:
		if len(operation.Value) == 0 || string(operation.Value) == "null" {
			return nil, true, nil
		}
		var chrome ViewChrome
		if err := json.Unmarshal(operation.Value, &chrome); err != nil {
			return nil, false, fmt.Errorf("decode chrome replacement: %w", err)
		}
		return &chrome, true, nil
	default:
		return nil, false, nil
	}
}

func replaceViewEventHandlerRoles(session *viewSession, chrome *ViewChrome) {
	clear(session.coalescibleHandlers)
	if chrome == nil {
		return
	}
	for _, handler := range []string{chrome.OnSearch, chrome.OnChip} {
		if handler = strings.TrimSpace(handler); handler != "" {
			session.coalescibleHandlers[handler] = struct{}{}
		}
	}
}
