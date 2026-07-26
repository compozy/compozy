package extensiontest

import (
	"bytes"

	"encoding/json"

	"os"
	"path/filepath"

	"strings"

	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func readJSONFile[T any](path string) (T, error) {
	var item T
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return item, err
	}
	err = json.Unmarshal(payload, &item)
	return item, err
}

func readJSONLinesFile[T any](path string) ([]T, error) {
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, nil
	}
	lines := bytes.Split(payload, []byte{'\n'})
	items := make([]T, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func waitForJSONLinesCondition[T any](
	t testing.TB,
	path string,
	timeout time.Duration,
	label string,
	predicate func([]T) bool,
) []T {
	t.Helper()

	var records []T
	waitForCondition(t, timeout, label, func() bool {
		loaded, err := readJSONLinesFile[T](path)
		if err != nil {
			return false
		}
		records = loaded
		return predicate(records)
	})
	return records
}

func appendJSONLine(t testing.TB, path string, value any) {
	t.Helper()
	target := strings.TrimSpace(path)
	if target == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(target), err)
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v", target, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("file.Close(%q) error = %v", target, closeErr)
		}
	}()
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatalf("encoder.Encode(%q) error = %v", target, err)
	}
}

func cloneBridgeDegradation(degradation *bridgepkg.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if degradation == nil {
		return nil
	}
	cloned := *degradation
	return &cloned
}

func copyBridgeInstance(instance bridgepkg.BridgeInstance) bridgepkg.BridgeInstance {
	copied := instance
	copied.ProviderConfig = append([]byte(nil), instance.ProviderConfig...)
	copied.DeliveryDefaults = append([]byte(nil), instance.DeliveryDefaults...)
	copied.Degradation = cloneBridgeDegradation(instance.Degradation)
	return copied
}

func waitForCondition(t testing.TB, timeout time.Duration, label string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s did not satisfy condition before timeout", label)
}

func normalizeEventType(eventType bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(eventType))))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
