package extensiontest_test

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/sdk/go/contracts"
	"github.com/compozy/compozy/sdk/go/extensiontest"
)

func TestPublicProvideConformance(t *testing.T) {
	t.Parallel()

	fixtures := extensiontest.PublicProvideFixtures()
	t.Run("Should expose every public provide fixture", func(t *testing.T) {
		t.Parallel()

		if got, want := len(fixtures), len(contracts.PublicProvideConformanceFixtures()); got != want {
			t.Fatalf("PublicProvideFixtures() count = %d, want %d", got, want)
		}
	})
	for _, fixture := range fixtures {
		t.Run("Should validate "+fixture.Provide, func(t *testing.T) {
			t.Parallel()

			if err := extensiontest.ValidateProvide(fixture.Provide, fixture.RequiredMethods); err != nil {
				t.Fatalf("ValidateProvide(%q) error = %v", fixture.Provide, err)
			}
			if len(fixture.RequiredMethods) == 0 {
				t.Fatalf("fixture %q has no required methods", fixture.Provide)
			}
			missing := fixture.RequiredMethods[0]
			if err := extensiontest.ValidateProvide(fixture.Provide, fixture.RequiredMethods[1:]); err == nil ||
				!strings.Contains(err.Error(), missing) {
				t.Fatalf("ValidateProvide(%q) error = %v, want missing %q", fixture.Provide, err, missing)
			}
		})
	}

	t.Run("Should keep bridge adapter conformance private", func(t *testing.T) {
		t.Parallel()

		if err := extensiontest.ValidateProvide("bridge.adapter", []string{
			"bridges/deliver",
			"bridges/targets/snapshot",
		}); err == nil {
			t.Fatal("ValidateProvide(bridge.adapter) error = nil, want private-fixture rejection")
		}
	})
}
