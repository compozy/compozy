// Suite: extension upstream attribution
// Invariant: the shipped extension carries the MIT attribution for the adapted upstream skills.
// Boundary IN: the shipped NOTICE file and README as distributed attribution artifacts.
// Boundary OUT: license validity judgments and PR-description credits.
package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtensionShipsUpstreamMITNotice(t *testing.T) {
	t.Parallel()

	noticePath := filepath.Join(extensionRoot(t), "NOTICE")
	data, err := os.ReadFile(noticePath)
	if err != nil {
		t.Fatalf("read NOTICE: %v", err)
	}

	notice := string(data)
	for _, want := range []string{
		"MIT License",
		"Copyright (c) 2026 Matt Pocock",
		"The above copyright notice and this permission notice shall be included in all",
		"https://github.com/mattpocock/skills",
	} {
		if !strings.Contains(notice, want) {
			t.Fatalf("NOTICE is missing required attribution text %q", want)
		}
	}
}

func TestREADMECreditsUpstreamAuthor(t *testing.T) {
	t.Parallel()

	readme := readREADME(t)
	for _, want := range []string{
		"## Credits",
		"Matt Pocock",
		"https://github.com/mattpocock/skills",
		"NOTICE",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README is missing required credit text %q", want)
		}
	}
}

func TestShippedSkillsCarryUpstreamAttribution(t *testing.T) {
	t.Parallel()

	for _, skillName := range expectedSkillNames {
		skillName := skillName
		t.Run(skillName, func(t *testing.T) {
			t.Parallel()

			skillPath := filepath.Join(extensionRoot(t), "skills", skillName, "SKILL.md")
			data, err := os.ReadFile(skillPath)
			if err != nil {
				t.Fatalf("read %s: %v", skillPath, err)
			}

			skill := string(data)
			for _, want := range []string{"Matt Pocock", "NOTICE"} {
				if !strings.Contains(skill, want) {
					t.Fatalf("%s SKILL.md is missing attribution text %q", skillName, want)
				}
			}
		})
	}
}
