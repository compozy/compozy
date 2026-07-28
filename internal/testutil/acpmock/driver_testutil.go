package acpmock

import "testing"

// RequireDriver resolves the ACP mock driver binary or fails the current test.
func RequireDriver(t testing.TB) string {
	t.Helper()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("acpmock.DefaultDriverPath() error = %v", err)
	}
	return driverPath
}
