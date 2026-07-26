package skills

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestBuildCatalogReturnsEmptyStringWhenNoSkills(t *testing.T) {
	t.Parallel()

	testCases := map[string][]*Skill{
		"nil":   nil,
		"empty": {},
	}

	for name, skills := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := BuildCatalog(skills); got != "" {
				t.Fatalf("BuildCatalog() = %q, want empty string", got)
			}
		})
	}
}

func TestBuildCatalogFormatsCatalogSortedEscapedAndWithUsageInstructions(t *testing.T) {
	t.Parallel()

	skills := []*Skill{
		{
			Meta: SkillMeta{
				Name:        "zeta",
				Description: "Last skill",
			},
			Enabled: true,
		},
		{
			Meta: SkillMeta{
				Name:        `alpha"<&>`,
				Description: `Use < & > and "quotes" safely`,
			},
			Enabled: true,
		},
	}

	got := BuildCatalog(skills)

	want := strings.Join([]string{
		"<available-skills>",
		`  <skill name="alpha&quot;&lt;&amp;&gt;">Use &lt; &amp; &gt; and "quotes" safely</skill>`,
		`  <skill name="zeta">Last skill</skill>`,
		"</available-skills>",
		"",
		"Resolve canonical `agh__skill_view` through the active harness, then call the returned tool reference to load full instructions for any skill.",
		"Use the returned tool reference for canonical `agh__skill_view` to read a specific skill resource file when the skill references one.",
		"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
	}, "\n")

	if got != want {
		t.Fatalf("BuildCatalog() mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildCurrentCatalogFormatsAuthoritativeTurnScopedCatalog(t *testing.T) {
	t.Parallel()

	got := BuildCurrentCatalog([]*Skill{
		{
			Meta: SkillMeta{
				Name:        "qa-marker-skill",
				Description: "Turn-scoped marker.",
			},
			Enabled: true,
		},
	})

	want := strings.Join([]string{
		"<current-available-skills>",
		`  <skill name="qa-marker-skill">Turn-scoped marker.</skill>`,
		"</current-available-skills>",
		"",
		"The <current-available-skills> block above is the authoritative current skill state for this turn.",
		"If it differs from any earlier <available-skills> startup snapshot, trust the current block.",
		"Resolve canonical `agh__skill_view` through the active harness, then call the returned tool reference to load full instructions for any skill.",
		"Use the returned tool reference for canonical `agh__skill_view` to read a specific skill resource file when the skill references one.",
		"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
	}, "\n")

	if got != want {
		t.Fatalf("BuildCurrentCatalog() mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuildCatalogTruncatesDescriptionsAtTwoHundredCharactersWithEllipsis(t *testing.T) {
	t.Parallel()

	description := strings.Repeat("a", catalogDescriptionLimit+5)
	got := BuildCatalog([]*Skill{
		{
			Meta: SkillMeta{
				Name:        "long",
				Description: description,
			},
			Enabled: true,
		},
	})

	wantDescription := strings.Repeat("a", catalogDescriptionLimit-len(catalogEllipsis)) + catalogEllipsis
	wantLine := `  <skill name="long">` + wantDescription + `</skill>`

	if !strings.Contains(got, wantLine) {
		t.Fatalf("BuildCatalog() missing truncated line %q in %q", wantLine, got)
	}

	if utf8.RuneCountInString(wantDescription) != catalogDescriptionLimit {
		t.Fatalf(
			"truncated description rune count = %d, want %d",
			utf8.RuneCountInString(wantDescription),
			catalogDescriptionLimit,
		)
	}
}

func TestBuildCatalogTruncatesUnicodeDescriptionsAtRuneBoundary(t *testing.T) {
	t.Parallel()

	description := strings.Repeat("界", catalogDescriptionLimit+5)
	got := BuildCatalog([]*Skill{
		{
			Meta: SkillMeta{
				Name:        "unicode",
				Description: description,
			},
			Enabled: true,
		},
	})

	wantDescription := strings.Repeat("界", catalogDescriptionLimit-len(catalogEllipsis)) + catalogEllipsis
	wantLine := `  <skill name="unicode">` + wantDescription + `</skill>`

	if !strings.Contains(got, wantLine) {
		t.Fatalf("BuildCatalog() missing unicode truncated line %q in %q", wantLine, got)
	}

	if utf8.RuneCountInString(wantDescription) != catalogDescriptionLimit {
		t.Fatalf(
			"unicode truncated description rune count = %d, want %d",
			utf8.RuneCountInString(wantDescription),
			catalogDescriptionLimit,
		)
	}
}

func TestBuildCatalogDoesNotTruncateUnicodeDescriptionsBelowRuneLimit(t *testing.T) {
	t.Parallel()

	description := strings.Repeat("界", catalogDescriptionLimit-2)
	got := BuildCatalog([]*Skill{
		{
			Meta: SkillMeta{
				Name:        "unicode-within-limit",
				Description: description,
			},
			Enabled: true,
		},
	})

	wantLine := `  <skill name="unicode-within-limit">` + description + `</skill>`
	if !strings.Contains(got, wantLine) {
		t.Fatalf("BuildCatalog() missing untruncated unicode line %q in %q", wantLine, got)
	}
}

func TestBuildCatalogExcludesDisabledSkills(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude disabled skills", func(t *testing.T) {
		got := BuildCatalog([]*Skill{
			{
				Meta: SkillMeta{
					Name:        "enabled",
					Description: "Visible skill",
				},
				Enabled: true,
			},
			{
				Meta: SkillMeta{
					Name:        "disabled",
					Description: "Hidden skill",
				},
				Enabled: false,
			},
		})

		if strings.Contains(got, `name="disabled"`) {
			t.Fatalf("BuildCatalog() included disabled skill: %q", got)
		}
		if !strings.Contains(got, `name="enabled"`) {
			t.Fatalf("BuildCatalog() missing enabled skill: %q", got)
		}
	})
}

func TestBuildCatalogHonorsActivationGates(t *testing.T) {
	t.Parallel()

	t.Run("Should keep inactive skills listable while excluding their advertised tokens", func(t *testing.T) {
		t.Parallel()

		gated := &Skill{
			Meta:    SkillMeta{Name: "linux-only", Description: "Linux-only instructions"},
			Enabled: true,
			ActivationGates: ActivationGates{
				Platforms: []string{"linux"},
			},
		}
		evaluateSkillActivation(gated, ActivationContext{Platform: "darwin"})
		ungated := &Skill{
			Meta:       SkillMeta{Name: "portable", Description: "Portable instructions"},
			Enabled:    true,
			Activation: SkillActivation{Active: true},
		}

		catalog := BuildCatalog([]*Skill{gated, ungated})
		if strings.Contains(catalog, "linux-only") {
			t.Fatalf("BuildCatalog() = %q, want gated skill tokens excluded", catalog)
		}
		if !strings.Contains(catalog, "portable") {
			t.Fatalf("BuildCatalog() = %q, want ungated skill advertised", catalog)
		}
		if gated.Activation.Active {
			t.Fatalf("gated.Activation = %#v, want inactive", gated.Activation)
		}
		if got, want := gated.Activation.Reasons[0].Gate, ActivationGatePlatforms; got != want {
			t.Fatalf("gated reason = %q, want %q", got, want)
		}
	})

	t.Run("Should activate required tools on the next evaluation", func(t *testing.T) {
		t.Parallel()

		skill := &Skill{
			Meta:    SkillMeta{Name: "tool-gated", Description: "Needs a tool"},
			Enabled: true,
			ActivationGates: ActivationGates{
				RequiresTools: []string{"agh__skill_view"},
			},
		}
		evaluateSkillActivation(skill, ActivationContext{Platform: "linux"})
		if skill.Activation.Active {
			t.Fatalf("first Activation = %#v, want inactive", skill.Activation)
		}

		evaluateSkillActivation(skill, ActivationContext{
			Platform: "linux",
			Tools:    []string{"agh__skill_view"},
		})
		if !skill.Activation.Active || len(skill.Activation.Reasons) != 0 {
			t.Fatalf("second Activation = %#v, want active", skill.Activation)
		}
	})

	t.Run("Should preserve ungated catalog behavior after evaluation", func(t *testing.T) {
		t.Parallel()

		skill := &Skill{
			Meta:    SkillMeta{Name: "portable", Description: "Runs without activation gates"},
			Enabled: true,
		}
		before := BuildCatalog([]*Skill{skill})

		providerCalled := false
		registry := &Registry{activationContext: func(
			context.Context,
			ActivationTarget,
		) (ActivationContext, error) {
			providerCalled = true
			return ActivationContext{}, nil
		}}
		if _, err := registry.projectSkillActivation(t.Context(), []*Skill{skill}, ActivationTarget{}); err != nil {
			t.Fatalf("projectSkillActivation() error = %v", err)
		}
		after := BuildCatalog([]*Skill{skill})

		if before != after {
			t.Fatalf("ungated catalog changed after evaluation\nbefore=%q\nafter=%q", before, after)
		}
		if !skill.Activation.Active || len(skill.Activation.Reasons) != 0 {
			t.Fatalf("Activation = %#v, want active without reasons", skill.Activation)
		}
		if providerCalled {
			t.Fatal("ungated projection called activation context provider")
		}
	})

	t.Run("Should compose gate families with AND and values with their declared cardinality", func(t *testing.T) {
		t.Parallel()

		skill := &Skill{
			Meta:    SkillMeta{Name: "composed", Description: "Exercises every gate family"},
			Enabled: true,
			ActivationGates: ActivationGates{
				Platforms:            []string{"linux", "darwin"},
				Environments:         []string{"container", "ci"},
				RequiresTools:        []string{"agh__skill_view", "agh__skill_list"},
				RequiresCapabilities: []string{"review.code", "test.run"},
			},
		}
		evaluateSkillActivation(skill, ActivationContext{
			Platform:     "darwin",
			Environments: []string{"ci"},
			Tools:        []string{"agh__skill_list", "agh__skill_view"},
			Capabilities: []string{"test.run", "review.code"},
		})
		if !skill.Activation.Active {
			t.Fatalf("Activation = %#v, want OR platform/environment and all required values active", skill.Activation)
		}

		evaluateSkillActivation(skill, ActivationContext{
			Platform:     "darwin",
			Environments: []string{"ci"},
			Tools:        []string{"agh__skill_view"},
			Capabilities: []string{"review.code"},
		})
		if skill.Activation.Active {
			t.Fatalf("Activation = %#v, want missing tool and capability inactive", skill.Activation)
		}
		wantGates := []ActivationGate{ActivationGateRequiresTools, ActivationGateRequiresCapabilities}
		gotGates := make([]ActivationGate, 0, len(skill.Activation.Reasons))
		for _, reason := range skill.Activation.Reasons {
			gotGates = append(gotGates, reason.Gate)
		}
		if !slices.Equal(gotGates, wantGates) {
			t.Fatalf("reason gates = %#v, want stable %#v", gotGates, wantGates)
		}
	})

	t.Run("Should fail an environment gate closed without an authoritative context", func(t *testing.T) {
		t.Parallel()

		skill := &Skill{
			Meta:    SkillMeta{Name: "container-only", Description: "Requires environment context"},
			Enabled: true,
			ActivationGates: ActivationGates{
				Environments: []string{"container"},
			},
		}
		evaluateSkillActivation(skill, ActivationContext{Platform: "linux"})
		if skill.Activation.Active || len(skill.Activation.Reasons) != 1 {
			t.Fatalf("Activation = %#v, want one fail-closed reason", skill.Activation)
		}
		if got, want := skill.Activation.Reasons[0].Code, ActivationReasonEnvironmentContextUnavailable; got != want {
			t.Fatalf("reason code = %q, want %q", got, want)
		}
	})
}

func TestCatalogProviderPromptSectionReturnsEmptyStringWhenWorkspaceHasNoSkills(t *testing.T) {
	t.Parallel()

	provider := NewCatalogProvider(newTestRegistry(t, RegistryConfig{}))

	got, err := provider.PromptSection(context.Background(), &workspacepkg.ResolvedWorkspace{})
	if err != nil {
		t.Fatalf("PromptSection() error = %v", err)
	}
	if got != "" {
		t.Fatalf("PromptSection() = %q, want empty string", got)
	}
}

func TestCatalogProviderPromptSectionUsesWorkspaceScopedSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	workspaceOne := filepath.Join(root, "workspace-one")
	workspaceTwo := filepath.Join(root, "workspace-two")

	writeSkillFile(t, userDir, filepath.Join("global", skillFileName), skillWithDescription("global", "Global skill"))
	writeSkillFile(
		t,
		filepath.Join(workspaceOne, ".agh", "skills"),
		filepath.Join("alpha", skillFileName),
		skillWithDescription("alpha", "Workspace one skill"),
	)
	writeSkillFile(
		t,
		filepath.Join(workspaceTwo, ".agh", "skills"),
		filepath.Join("beta", skillFileName),
		skillWithDescription("beta", "Workspace two skill"),
	)

	registry := newTestRegistry(t, RegistryConfig{
		UserSkillsDir: userDir,
	})
	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	provider := NewCatalogProvider(registry)

	got, err := provider.PromptSection(context.Background(), resolvedWorkspacePtr(
		"ws_catalog_one",
		workspaceOne,
		resolvedSkillPath(filepath.Join(workspaceOne, ".agh", "skills", "alpha"), "workspace"),
	))
	if err != nil {
		t.Fatalf("PromptSection() error = %v", err)
	}

	want := strings.Join([]string{
		"<available-skills>",
		`  <skill name="alpha">Workspace one skill</skill>`,
		`  <skill name="global">Global skill</skill>`,
		"</available-skills>",
		"",
		"Resolve canonical `agh__skill_view` through the active harness, then call the returned tool reference to load full instructions for any skill.",
		"Use the returned tool reference for canonical `agh__skill_view` to read a specific skill resource file when the skill references one.",
		"If current tool policy denies canonical `agh__skill_view`, use `agh skill view <name>` as an operator fallback.",
	}, "\n")

	if got != want {
		t.Fatalf("PromptSection() mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}

	if strings.Contains(got, "beta") {
		t.Fatalf("PromptSection() leaked workspace-two skill into workspace-one catalog: %q", got)
	}
}

func TestBuildCatalogUsesToolFirstSkillLoadingInstructions(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer skill view tool over CLI fallback", func(t *testing.T) {
		t.Parallel()

		got := BuildCatalog([]*Skill{
			{
				Meta: SkillMeta{
					Name:        "agh",
					Description: "AGH guidance",
				},
				Enabled: true,
			},
		})

		if !strings.Contains(got, "Resolve canonical `agh__skill_view` through the active harness") {
			t.Fatalf("BuildCatalog() = %q, want harness-agnostic agh__skill_view guidance", got)
		}
		if !strings.Contains(got, "operator fallback") {
			t.Fatalf("BuildCatalog() = %q, want conditional operator fallback guidance", got)
		}
		if strings.Contains(got, "Use `agh skill view <name>` to load full instructions") {
			t.Fatalf("BuildCatalog() = %q, want no CLI-first loading guidance", got)
		}
	})
}
