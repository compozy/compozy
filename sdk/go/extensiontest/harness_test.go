package extensiontest_test

import (
	"reflect"
	"slices"
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

		generated := contracts.PublicProvideConformanceFixtures()
		got := normalizeProvideFixtures(fixtures)
		want := normalizeGeneratedProvideFixtures(generated)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("PublicProvideFixtures() = %#v, want %#v", got, want)
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

type provideFixtureIdentity struct {
	Provide         string
	RequiredMethods []string
}

func normalizeProvideFixtures(fixtures []extensiontest.ProvideFixture) []provideFixtureIdentity {
	normalized := make([]provideFixtureIdentity, 0, len(fixtures))
	for _, fixture := range fixtures {
		normalized = append(normalized, provideFixtureIdentity{
			Provide:         fixture.Provide,
			RequiredMethods: slices.Clone(fixture.RequiredMethods),
		})
	}
	slices.SortFunc(normalized, compareProvideFixtureIdentity)
	return normalized
}

func normalizeGeneratedProvideFixtures(
	fixtures []contracts.ProvideConformanceFixture,
) []provideFixtureIdentity {
	normalized := make([]provideFixtureIdentity, 0, len(fixtures))
	for _, fixture := range fixtures {
		normalized = append(normalized, provideFixtureIdentity{
			Provide:         fixture.Provide,
			RequiredMethods: slices.Clone(fixture.RequiredMethods),
		})
	}
	slices.SortFunc(normalized, compareProvideFixtureIdentity)
	return normalized
}

func compareProvideFixtureIdentity(left, right provideFixtureIdentity) int {
	if left.Provide != right.Provide {
		return strings.Compare(left.Provide, right.Provide)
	}
	return slices.Compare(left.RequiredMethods, right.RequiredMethods)
}
