// Package extensiontest provides public conformance helpers for CompozyOS extensions.
package extensiontest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

// ProvideFixture describes one public provide capability's required service methods.
type ProvideFixture struct {
	Provide         string
	RequiredMethods []string
}

// PublicProvideFixtures returns a defensive copy of every public conformance fixture.
func PublicProvideFixtures() []ProvideFixture {
	generated := contracts.PublicProvideConformanceFixtures()
	fixtures := make([]ProvideFixture, 0, len(generated))
	for _, fixture := range generated {
		fixtures = append(fixtures, ProvideFixture{
			Provide:         fixture.Provide,
			RequiredMethods: append([]string(nil), fixture.RequiredMethods...),
		})
	}
	return fixtures
}

// ValidateProvide verifies that an extension implements every method required by a public provide.
func ValidateProvide(provide string, implementedMethods []string) error {
	provide = strings.TrimSpace(provide)
	for _, fixture := range PublicProvideFixtures() {
		if fixture.Provide != provide {
			continue
		}
		missing := make([]string, 0, len(fixture.RequiredMethods))
		for _, required := range fixture.RequiredMethods {
			if !slices.Contains(implementedMethods, required) {
				missing = append(missing, required)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("provide %q is missing required methods: %s", provide, strings.Join(missing, ", "))
		}
		return nil
	}
	return fmt.Errorf("provide %q has no public conformance fixture", provide)
}
