// Suite: extension README packaging
// Invariant: installers receive the exact source, memory wiring, durability, and companion guidance.
// Boundary IN: the shipped README as an installer-visible product contract.
// Boundary OUT: executing CLI commands and audit behavior, owned by workflow E2E scenarios.
package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canonicalInstallSource = "compozy/compozy --remote github --ref <tag> --subdir extensions/cy-improve-architecture"

func TestREADMEUsesCanonicalInstallSource(t *testing.T) {
	t.Parallel()

	readme := readREADME(t)
	for _, command := range []string{
		"compozy ext enable cy-improve-architecture",
		"compozy setup",
	} {
		if !strings.Contains(readme, command) {
			t.Fatalf("README does not document install lifecycle command %q", command)
		}
	}

	installLines := linesContaining(readme, "compozy ext install")
	if len(installLines) == 0 {
		t.Fatal("README does not contain a compozy ext install command")
	}
	for _, line := range installLines {
		if !strings.Contains(line, canonicalInstallSource) {
			t.Fatalf("install command does not use the canonical source: %q", line)
		}
	}
}

func TestREADMEDocumentsArchitectureMapWiring(t *testing.T) {
	t.Parallel()

	readme := readREADME(t)
	for _, fileName := range []string{"CLAUDE.md", "AGENTS.md"} {
		fileName := fileName
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(readme, fileName) {
				t.Fatalf("README does not document %s wiring", fileName)
			}
		})
	}
	if !strings.Contains(readme, "@.compozy/ARCHITECTURE.md") {
		t.Fatal("README does not document the architecture map import")
	}
	if count := strings.Count(readme, "@.compozy/ARCHITECTURE.md"); count < 2 {
		t.Fatalf(
			"README must show the architecture map import for both instruction files, found %d occurrence(s)",
			count,
		)
	}
}

func TestREADMEDocumentsGitignoreNegations(t *testing.T) {
	t.Parallel()

	readme := readREADME(t)
	negations := []string{
		"!.compozy/ARCHITECTURE.md",
		"!.compozy/arch-reviews/",
		"!.compozy/arch-reviews/**",
	}
	for _, negation := range negations {
		negation := negation
		t.Run(negation, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(readme, negation) {
				t.Fatalf("README does not document gitignore negation %q", negation)
			}
		})
	}
}

func TestREADMERecommendsOptionalDecisionCompanion(t *testing.T) {
	t.Parallel()

	readme := readREADME(t)
	companionLines := linesContaining(readme, "cy-capture-decisions")
	if len(companionLines) == 0 {
		t.Fatal("README does not mention cy-capture-decisions")
	}

	for _, line := range companionLines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "recommend") && strings.Contains(lowerLine, "optional") {
			return
		}
	}
	t.Fatal("README does not recommend cy-capture-decisions as optional")
}

func readREADME(t *testing.T) string {
	t.Helper()

	readmePath := filepath.Join(extensionRoot(t), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %q: %v", readmePath, err)
	}
	return string(data)
}

func linesContaining(content string, substring string) []string {
	var matches []string
	for line := range strings.Lines(content) {
		if strings.Contains(line, substring) {
			matches = append(matches, line)
		}
	}
	return matches
}
