package update

import (
	"strings"
	"testing"
)

func TestRuntimeCompatibilityBackstop(t *testing.T) {
	t.Run("Should refuse a runtime requiring a newer installed app", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v1.1.0",
		}, "v1.0.0")
		if err == nil || !strings.Contains(err.Error(), "requires CompozyOS app v1.1.0 or newer") {
			t.Fatalf("CheckRuntimeCompatibility() error = %v, want compatibility refusal", err)
		}
	})

	t.Run("Should pass when no desktop app is installed", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v9.0.0",
		}, "")
		if err != nil {
			t.Fatalf("CheckRuntimeCompatibility(no app) error = %v", err)
		}
	})

	t.Run("Should pass when the installed app meets the minimum", func(t *testing.T) {
		t.Parallel()

		err := CheckRuntimeCompatibility(Compatibility{
			RuntimeVersion: "v1.2.0",
			MinAppVersion:  "v1.1.0",
		}, "v1.1.0")
		if err != nil {
			t.Fatalf("CheckRuntimeCompatibility(equal app) error = %v", err)
		}
	})
}
