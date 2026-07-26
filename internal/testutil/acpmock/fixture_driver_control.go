package acpmock

import (
	"fmt"

	"math"

	"strings"
	"time"
)

// Validate ensures the driver-control payload is internally consistent.
func (d DriverControlStep) Validate(path string) error {
	if d.DelayMS < 0 {
		return fmt.Errorf("acpmock: %s.delay_ms must be >= 0", path)
	}
	if int64(d.DelayMS) > math.MaxInt64/int64(time.Millisecond) {
		return fmt.Errorf("acpmock: %s.delay_ms exceeds duration capacity", path)
	}
	switch d.Action {
	case DriverControlDisconnect, DriverControlBlockUntilCancel, DriverControlDelay:
		if strings.TrimSpace(d.RawJSONRPC) != "" {
			return fmt.Errorf("acpmock: %s.raw_jsonrpc is only valid for write_raw_jsonrpc", path)
		}
	case DriverControlWriteRawJSONRPC:
		if strings.TrimSpace(d.RawJSONRPC) == "" {
			return fmt.Errorf("acpmock: %s.raw_jsonrpc is required", path)
		}
	default:
		return fmt.Errorf("acpmock: %s.action %q is invalid", path, d.Action)
	}
	if d.Async && d.Action == DriverControlBlockUntilCancel {
		return fmt.Errorf("acpmock: %s.async is invalid for block_until_cancel", path)
	}
	if d.Action == DriverControlDelay {
		if d.DelayMS == 0 {
			return fmt.Errorf("acpmock: %s.delay_ms must be > 0 for delay", path)
		}
		if d.Async {
			return fmt.Errorf("acpmock: %s.async is invalid for delay", path)
		}
	}
	return nil
}

func hasTextPayload(text string, chunks []string) bool {
	if strings.TrimSpace(text) != "" {
		return true
	}
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) != "" {
			return true
		}
	}
	return false
}

func validateToolKind(path string, raw string) error {
	switch strings.TrimSpace(raw) {
	case "", "read", "edit", "delete", "move", "search", "execute", "think", "fetch", "switch_mode", "other":
		return nil
	default:
		return fmt.Errorf("acpmock: %s %q is invalid", path, raw)
	}
}

func validateToolStatus(path string, raw string) error {
	switch strings.TrimSpace(raw) {
	case "", "pending", "in_progress", "completed", "failed":
		return nil
	default:
		return fmt.Errorf("acpmock: %s %q is invalid", path, raw)
	}
}

func validatePermissionDecision(path string, raw string) error {
	switch strings.TrimSpace(raw) {
	case "", "allow-once", fixtureAllowAlwaysValue, "reject-once", "reject-always":
		return nil
	default:
		return fmt.Errorf("acpmock: %s %q is invalid", path, raw)
	}
}
