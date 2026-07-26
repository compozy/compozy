package daemon

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHarnessContextResolverMatrix(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
		SkillsAugmenter:            true,
		DurableMemoryAugmenter:     true,
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})

	testCases := []struct {
		name           string
		input          HarnessResolutionInput
		wantSections   []HarnessPromptSection
		wantAugmenters []HarnessAugmenter
		wantReentry    ReentryMode
		wantDetached   DetachedRunMode
		wantLabel      string
		wantTags       map[string]string
	}{
		{
			name: "user session plus user turn resolves baseline policy",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type: session.SessionTypeUser,
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceUser,
				},
			},
			wantSections:   []HarnessPromptSection{HarnessPromptSectionMemory, HarnessPromptSectionSkills},
			wantAugmenters: []HarnessAugmenter{HarnessAugmenterSkills, HarnessAugmenterDurableMemory},
			wantReentry:    ReentryModeNone,
			wantDetached:   DetachedRunModeNone,
			wantLabel:      "interactive.user",
			wantTags: map[string]string{
				"harness.surface":          "turn",
				"harness.session_type":     "user",
				"harness.session_class":    "interactive",
				"harness.turn_origin":      "user",
				"harness.network_live":     "false",
				"harness.diagnostic_label": "interactive.user",
			},
		},
		{
			name: "Should resolve local user session plus network provenance turn without live network section",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type: session.SessionTypeUser,
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceNetwork,
					PromptMeta: acp.PromptMeta{
						Network: &acp.PromptNetworkMeta{
							MessageID: "321",
							Kind:      "message",
							From:      "777",
						},
					},
				},
			},
			wantSections:   []HarnessPromptSection{HarnessPromptSectionMemory, HarnessPromptSectionSkills},
			wantAugmenters: []HarnessAugmenter{HarnessAugmenterSkills},
			wantReentry:    ReentryModeNone,
			wantDetached:   DetachedRunModeNone,
			wantLabel:      "interactive.network",
			wantTags: map[string]string{
				"harness.surface":          "turn",
				"harness.session_type":     "user",
				"harness.session_class":    "interactive",
				"harness.turn_origin":      "network",
				"harness.network_live":     "false",
				"harness.diagnostic_label": "interactive.network",
			},
		},
		{
			name: "Should resolve channel-bound user session plus network turn with network-aware policy",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type:                 session.SessionTypeUser,
					NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceNetwork,
					PromptMeta: acp.PromptMeta{
						Network: &acp.PromptNetworkMeta{
							Channel: "builders",
							From:    "ops.peer",
						},
					},
				},
			},
			wantSections: []HarnessPromptSection{
				HarnessPromptSectionMemory,
				HarnessPromptSectionSkills,
				HarnessPromptSectionNetwork,
			},
			wantAugmenters: []HarnessAugmenter{HarnessAugmenterSkills},
			wantReentry:    ReentryModeNone,
			wantDetached:   DetachedRunModeNone,
			wantLabel:      "interactive.network.network",
			wantTags: map[string]string{
				"harness.surface":          "turn",
				"harness.session_type":     "user",
				"harness.session_class":    "interactive",
				"harness.turn_origin":      "network",
				"harness.network_live":     "true",
				"harness.diagnostic_label": "interactive.network.network",
			},
		},
		{
			name: "Should resolve coordinator policy for coordinator startup session",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceStartup,
				Session: HarnessSessionInput{
					Type:                 session.SessionTypeCoordinator,
					NetworkParticipation: daemonTestLiveParticipation("ws-1", "coord-run-1"),
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceUser,
				},
			},
			wantSections: []HarnessPromptSection{
				HarnessPromptSectionMemory,
				HarnessPromptSectionSkills,
				HarnessPromptSectionNetwork,
			},
			wantAugmenters: nil,
			wantReentry:    ReentryModeNone,
			wantDetached:   DetachedRunModeNone,
			wantLabel:      "coordinator.network.user",
			wantTags: map[string]string{
				"harness.surface":          "startup",
				"harness.session_type":     "coordinator",
				"harness.session_class":    "coordinator",
				"harness.turn_origin":      "user",
				"harness.network_live":     "true",
				"harness.diagnostic_label": "coordinator.network.user",
			},
		},
		{
			name: "Should resolve spawned policy for spawned worker network turn",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type:                 session.SessionTypeSpawned,
					NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceNetwork,
					PromptMeta: acp.PromptMeta{
						Network: &acp.PromptNetworkMeta{
							Channel: "builders",
							From:    "coordinator.sess",
						},
					},
				},
			},
			wantSections: []HarnessPromptSection{
				HarnessPromptSectionMemory,
				HarnessPromptSectionSkills,
				HarnessPromptSectionNetwork,
			},
			wantAugmenters: []HarnessAugmenter{HarnessAugmenterSkills},
			wantReentry:    ReentryModeNone,
			wantDetached:   DetachedRunModeNone,
			wantLabel:      "spawned.network.network",
			wantTags: map[string]string{
				"harness.surface":          "turn",
				"harness.session_type":     "spawned",
				"harness.session_class":    "spawned",
				"harness.turn_origin":      "network",
				"harness.network_live":     "true",
				"harness.diagnostic_label": "spawned.network.network",
			},
		},
		{
			name: "Should require metadata and resolve reentry policy for a system session plus synthetic turn",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type: session.SessionTypeSystem,
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceSynthetic,
					Synthetic: &SyntheticTurnMetadata{
						Reason:  "task_complete",
						Trigger: "task.run.completed",
					},
					Detached: &DetachedRunMetadata{
						TaskRunID: "run-123",
					},
				},
			},
			wantSections:   []HarnessPromptSection{HarnessPromptSectionMemory, HarnessPromptSectionSkills},
			wantAugmenters: []HarnessAugmenter{HarnessAugmenterSkills},
			wantReentry:    ReentryModeSynthetic,
			wantDetached:   DetachedRunModeTaskRuntime,
			wantLabel:      "system.synthetic.reentry",
			wantTags: map[string]string{
				"harness.surface":           "turn",
				"harness.session_type":      "system",
				"harness.session_class":     "system",
				"harness.turn_origin":       "synthetic",
				"harness.network_live":      "false",
				"harness.diagnostic_label":  "system.synthetic.reentry",
				"harness.synthetic_reason":  "task_complete",
				"harness.synthetic_trigger": "task.run.completed",
				"harness.task_run_id":       "run-123",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolver.Resolve(tc.input)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if !slices.Equal(got.Policy.IncludeSections, tc.wantSections) {
				t.Fatalf("IncludeSections = %#v, want %#v", got.Policy.IncludeSections, tc.wantSections)
			}
			if !slices.Equal(got.Policy.EnableAugmenters, tc.wantAugmenters) {
				t.Fatalf("EnableAugmenters = %#v, want %#v", got.Policy.EnableAugmenters, tc.wantAugmenters)
			}
			if got.Policy.ReentryMode != tc.wantReentry {
				t.Fatalf("ReentryMode = %q, want %q", got.Policy.ReentryMode, tc.wantReentry)
			}
			if got.Policy.DetachedRunMode != tc.wantDetached {
				t.Fatalf("DetachedRunMode = %q, want %q", got.Policy.DetachedRunMode, tc.wantDetached)
			}
			if got.Policy.DiagnosticLabel != tc.wantLabel {
				t.Fatalf("DiagnosticLabel = %q, want %q", got.Policy.DiagnosticLabel, tc.wantLabel)
			}
			if !maps.Equal(got.Policy.ObservabilityTags, tc.wantTags) {
				t.Fatalf("ObservabilityTags = %#v, want %#v", got.Policy.ObservabilityTags, tc.wantTags)
			}
		})
	}
}

func TestHarnessContextResolverIncludesToolsSectionWhenEnabled(t *testing.T) {
	t.Parallel()

	t.Run("Should include tools section between skills and network", func(t *testing.T) {
		t.Parallel()

		resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
			MemoryPromptSectionEnabled: true,
			SkillsPromptSectionEnabled: true,
			ToolsPromptSectionEnabled:  true,
		})

		got, err := resolver.Resolve(HarnessResolutionInput{
			Surface: ResolutionSurfaceStartup,
			Session: HarnessSessionInput{
				Type:                 session.SessionTypeUser,
				NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
			},
			Turn: HarnessTurnRequest{
				Source: session.TurnSourceUser,
			},
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		wantSections := []HarnessPromptSection{
			HarnessPromptSectionMemory,
			HarnessPromptSectionSkills,
			HarnessPromptSectionTools,
			HarnessPromptSectionNetwork,
		}
		if !slices.Equal(got.Policy.IncludeSections, wantSections) {
			t.Fatalf("IncludeSections = %#v, want %#v", got.Policy.IncludeSections, wantSections)
		}
	})

	t.Run("Should include runtime identity section when enabled", func(t *testing.T) {
		t.Parallel()

		resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
			RuntimeIdentityPromptSectionEnabled: true,
			MemoryPromptSectionEnabled:          true,
			SkillsPromptSectionEnabled:          true,
		})

		got, err := resolver.Resolve(HarnessResolutionInput{
			Surface: ResolutionSurfaceStartup,
			Session: HarnessSessionInput{
				Type: session.SessionTypeUser,
			},
			Turn: HarnessTurnRequest{
				Source: session.TurnSourceUser,
			},
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		wantSections := []HarnessPromptSection{
			HarnessPromptSectionRuntimeIdentity,
			HarnessPromptSectionMemory,
			HarnessPromptSectionSkills,
		}
		if !slices.Equal(got.Policy.IncludeSections, wantSections) {
			t.Fatalf("IncludeSections = %#v, want %#v", got.Policy.IncludeSections, wantSections)
		}
	})
}

func TestHarnessContextResolverValidation(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
		SkillsAugmenter:            true,
		DurableMemoryAugmenter:     true,
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})

	testCases := []struct {
		name    string
		input   HarnessResolutionInput
		wantErr string
	}{
		{
			name: "unknown turn origin fails validation",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{Type: session.SessionTypeUser},
				Turn: HarnessTurnRequest{
					Source: session.TurnSource("mystery"),
				},
			},
			wantErr: `invalid harness turn origin "mystery"`,
		},
		{
			name: "mismatched prompt metadata fails validation",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{Type: session.SessionTypeUser},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceUser,
					PromptMeta: acp.PromptMeta{
						TurnSource: acp.PromptTurnSourceNetwork,
						Network: &acp.PromptNetworkMeta{
							Channel: "builders",
						},
					},
				},
			},
			wantErr: `does not match prompt metadata turn_source "network"`,
		},
		{
			name: "synthetic turn without metadata fails validation",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{Type: session.SessionTypeSystem},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceSynthetic,
				},
			},
			wantErr: "synthetic harness turns require runtime metadata",
		},
		{
			name: "Should reject a synthetic turn on a non-system session",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{Type: session.SessionTypeUser},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceSynthetic,
					Synthetic: &SyntheticTurnMetadata{
						Reason:  "task_complete",
						Trigger: "task.run.completed",
					},
				},
			},
			wantErr: `synthetic harness turns require a system session`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := resolver.Resolve(tc.input)
			if err == nil {
				t.Fatal("Resolve() error = nil, want validation failure")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Resolve() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestHarnessContextResolverDiagnosticLabelsAreStable(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
		SkillsAugmenter:            true,
		DurableMemoryAugmenter:     true,
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})

	input := HarnessResolutionInput{
		Surface: ResolutionSurfaceTurn,
		Session: HarnessSessionInput{
			Type:                 session.SessionTypeUser,
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
		},
		Turn: HarnessTurnRequest{
			Source: session.TurnSourceNetwork,
		},
	}

	first, err := resolver.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve(first) error = %v", err)
	}
	second, err := resolver.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve(second) error = %v", err)
	}

	if first.Policy.DiagnosticLabel != second.Policy.DiagnosticLabel {
		t.Fatalf(
			"DiagnosticLabel mismatch: first=%q second=%q",
			first.Policy.DiagnosticLabel,
			second.Policy.DiagnosticLabel,
		)
	}
}

func TestSectionSelectorSelectsEligibleStartupSectionsWithoutDuplicates(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
	})
	selector := NewSectionSelector(resolver, nil)
	descriptors := defaultStartupPromptSectionDescriptors(
		promptSectionProviderFunc(
			func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "memory", nil },
		),
		promptSectionProviderFunc(
			func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "skills", nil },
		),
		nil,
	)
	descriptors = append(descriptors, descriptors[len(descriptors)-1])

	selected, resolved, err := selector.Select(session.StartupPromptContext{
		SessionType:          session.SessionTypeUser,
		NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
	}, descriptors)
	if err != nil {
		t.Fatalf("Select(channel-bound) error = %v", err)
	}

	wantNames := []string{
		string(HarnessPromptSectionMemory),
		string(HarnessPromptSectionSkills),
		string(HarnessPromptSectionNetwork),
	}
	gotNames := make([]string, 0, len(selected))
	for _, descriptor := range selected {
		gotNames = append(gotNames, descriptor.Name)
	}
	if !slices.Equal(gotNames, wantNames) {
		t.Fatalf("selected section names = %#v, want %#v", gotNames, wantNames)
	}
	if !containsHarnessSection(resolved.Policy.IncludeSections, HarnessPromptSectionNetwork) {
		t.Fatalf("resolved IncludeSections = %#v, want network section", resolved.Policy.IncludeSections)
	}

	plain, _, err := selector.Select(session.StartupPromptContext{
		SessionType: session.SessionTypeUser,
	}, descriptors)
	if err != nil {
		t.Fatalf("Select(no channel) error = %v", err)
	}

	gotNames = gotNames[:0]
	for _, descriptor := range plain {
		gotNames = append(gotNames, descriptor.Name)
	}
	if !slices.Equal(gotNames, wantNames[:2]) {
		t.Fatalf("selected names without channel = %#v, want %#v", gotNames, wantNames[:2])
	}
}

func TestSectionSelectorAcceptsCoordinatorStartupSession(t *testing.T) {
	t.Parallel()

	t.Run("Should select coordinator startup sections", func(t *testing.T) {
		t.Parallel()

		resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
			MemoryPromptSectionEnabled: true,
			SkillsPromptSectionEnabled: true,
		})
		selector := NewSectionSelector(resolver, nil)
		descriptors := defaultStartupPromptSectionDescriptors(
			promptSectionProviderFunc(
				func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "memory", nil },
			),
			promptSectionProviderFunc(
				func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "skills", nil },
			),
			nil,
		)

		selected, resolved, err := selector.Select(session.StartupPromptContext{
			SessionType:          session.SessionTypeCoordinator,
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "coord-run-1"),
		}, descriptors)
		if err != nil {
			t.Fatalf("Select(coordinator) error = %v", err)
		}

		if resolved.Session.SessionClass != SessionClassCoordinator {
			t.Fatalf("SessionClass = %q, want %q", resolved.Session.SessionClass, SessionClassCoordinator)
		}
		wantNames := []string{
			string(HarnessPromptSectionMemory),
			string(HarnessPromptSectionSkills),
			string(HarnessPromptSectionNetwork),
		}
		gotNames := make([]string, 0, len(selected))
		for _, descriptor := range selected {
			gotNames = append(gotNames, descriptor.Name)
		}
		if !slices.Equal(gotNames, wantNames) {
			t.Fatalf("selected section names = %#v, want %#v", gotNames, wantNames)
		}
	})
}

func TestHarnessContextResolverResolvePromptUsesSessionInfo(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsAugmenter:            true,
		DurableMemoryAugmenter:     true,
	})

	resolved, err := resolver.ResolvePrompt(&session.Info{
		Type:                 session.SessionTypeUser,
		NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
	}, session.TurnSourceUser, acp.PromptMeta{})
	if err != nil {
		t.Fatalf("ResolvePrompt() error = %v", err)
	}

	if resolved.Policy.TurnOrigin != TurnOriginUser {
		t.Fatalf("TurnOrigin = %q, want %q", resolved.Policy.TurnOrigin, TurnOriginUser)
	}
	if !containsHarnessSection(resolved.Policy.IncludeSections, HarnessPromptSectionNetwork) {
		t.Fatalf("IncludeSections = %#v, want network section", resolved.Policy.IncludeSections)
	}
	if !slices.Equal(
		resolved.Policy.EnableAugmenters,
		[]HarnessAugmenter{HarnessAugmenterSkills, HarnessAugmenterDurableMemory},
	) {
		t.Fatalf("EnableAugmenters = %#v, want skills and durable memory", resolved.Policy.EnableAugmenters)
	}
}

func TestHarnessPromptInputAugmenterAppliesResolvedAugmenters(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		SkillsAugmenter:        true,
		DurableMemoryAugmenter: true,
	})

	memoryCalls := 0
	skillsCalls := 0
	augmenter, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		resolver,
		nil,
		defaultPromptInputAugmenterDescriptors(
			func(_ context.Context, _ *session.Session, message string) (string, error) {
				memoryCalls++
				return message + "\n\nmemory block", nil
			},
			func(_ context.Context, _ *session.Session, message string) (string, error) {
				skillsCalls++
				return "<current-available-skills>\n  <skill name=\"qa-marker-skill\">Marker.</skill>\n</current-available-skills>\n\n" + message, nil
			},
		)...,
	)
	if err != nil {
		t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
	}

	got, err := augmenter(
		context.Background(),
		&session.Session{Type: session.SessionTypeUser},
		"Base prompt.",
	)
	if err != nil {
		t.Fatalf("Augment() error = %v", err)
	}
	if memoryCalls != 1 {
		t.Fatalf("durable memory calls = %d, want 1", memoryCalls)
	}
	if skillsCalls != 1 {
		t.Fatalf("skills augmenter calls = %d, want 1", skillsCalls)
	}
	if !strings.Contains(got, "memory block") {
		t.Fatalf("Augment() = %q, want durable memory content", got)
	}
	if !strings.Contains(got, "<current-available-skills>") {
		t.Fatalf("Augment() = %q, want current skills content", got)
	}
}

func TestSectionSelectorValidationHelpers(t *testing.T) {
	t.Parallel()

	if got := NewSectionSelector(nil, nil); got != nil {
		t.Fatalf("NewSectionSelector(nil) = %#v, want nil", got)
	}

	err := validatePromptSectionDescriptors([]PromptSectionDescriptor{{
		Position: PromptSectionPositionPrepend,
	}})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("validatePromptSectionDescriptors(missing name) error = %v, want missing-name error", err)
	}

	err = validatePromptSectionDescriptors([]PromptSectionDescriptor{{
		Name:     "invalid",
		Position: PromptSectionPosition("sideways"),
	}})
	if err == nil || !strings.Contains(err.Error(), `invalid startup prompt section position "sideways"`) {
		t.Fatalf("validatePromptSectionDescriptors(invalid position) error = %v, want invalid-position error", err)
	}
}
