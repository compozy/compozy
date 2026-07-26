package contract

import (
	"encoding/json"
	"testing"
)

func TestSettingsMutationResultsJSONShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        any
		wantPresent  map[string]any
		wantAbsent   []string
		wantWarnings []string
	}{
		{
			name: "ShouldOmitOptionalFieldsForGlobalSectionMutations",
			input: SettingsGlobalSectionMutationResult{
				Section:         SettingsSectionGeneral,
				Scope:           SettingsGlobalScope,
				Behavior:        SettingsMutationBehaviorRestartRequired,
				Applied:         false,
				RestartRequired: true,
			},
			wantPresent: map[string]any{
				"section":          string(SettingsSectionGeneral),
				"scope":            string(SettingsGlobalScope),
				"behavior":         string(SettingsMutationBehaviorRestartRequired),
				"applied":          false,
				"restart_required": true,
			},
			wantAbsent: []string{"write_target", "workspace_id", "agent_name", "restart_scope", "warnings"},
		},
		{
			name: "ShouldPreserveAgentContextForSkillsMutations",
			input: SettingsSkillsMutationResult{
				Section:         SettingsSectionSkills,
				Scope:           SettingsAgentScopeAgent,
				WriteTarget:     SettingsWriteTargetWorkspaceAgentFile,
				WorkspaceID:     "ws-alpha",
				AgentName:       "coder",
				Behavior:        SettingsMutationBehaviorAppliedNow,
				Applied:         true,
				RestartRequired: false,
				RestartScope:    "none",
				Warnings:        []string{"agent scope changed"},
			},
			wantPresent: map[string]any{
				"section":          string(SettingsSectionSkills),
				"scope":            string(SettingsAgentScopeAgent),
				"write_target":     string(SettingsWriteTargetWorkspaceAgentFile),
				"workspace_id":     "ws-alpha",
				"agent_name":       "coder",
				"behavior":         string(SettingsMutationBehaviorAppliedNow),
				"applied":          true,
				"restart_required": false,
				"restart_scope":    "none",
			},
			wantWarnings: []string{"agent scope changed"},
		},
		{
			name: "ShouldKeepGlobalCollectionMutationsAgentFree",
			input: SettingsGlobalCollectionMutationResult{
				Section:         SettingsCollectionProviders,
				Scope:           SettingsGlobalScope,
				WriteTarget:     SettingsWriteTargetGlobalConfig,
				Behavior:        SettingsMutationBehaviorRestartRequired,
				Applied:         false,
				RestartRequired: true,
			},
			wantPresent: map[string]any{
				"section":          string(SettingsCollectionProviders),
				"scope":            string(SettingsGlobalScope),
				"write_target":     string(SettingsWriteTargetGlobalConfig),
				"behavior":         string(SettingsMutationBehaviorRestartRequired),
				"applied":          false,
				"restart_required": true,
			},
			wantAbsent: []string{"workspace_id", "agent_name", "restart_scope", "warnings"},
		},
		{
			name: "ShouldPreserveWorkspaceMetadataForScopedCollectionMutations",
			input: SettingsGlobalWorkspaceCollectionMutationResult{
				Section:         SettingsCollectionMCPServers,
				Scope:           SettingsWorkspaceScopeWorkspace,
				WriteTarget:     SettingsWriteTargetWorkspaceMCPSidecar,
				WorkspaceID:     "ws-alpha",
				Behavior:        SettingsMutationBehaviorAppliedNow,
				Applied:         true,
				RestartRequired: false,
				RestartScope:    "daemon",
			},
			wantPresent: map[string]any{
				"section":          string(SettingsCollectionMCPServers),
				"scope":            string(SettingsWorkspaceScopeWorkspace),
				"write_target":     string(SettingsWriteTargetWorkspaceMCPSidecar),
				"workspace_id":     "ws-alpha",
				"behavior":         string(SettingsMutationBehaviorAppliedNow),
				"applied":          true,
				"restart_required": false,
				"restart_scope":    "daemon",
			},
			wantAbsent: []string{"agent_name", "warnings"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			for key, want := range tt.wantPresent {
				got, ok := decoded[key]
				if !ok {
					t.Fatalf("decoded JSON missing key %q in %s", key, string(data))
				}
				switch wantValue := want.(type) {
				case bool:
					gotBool, ok := got.(bool)
					if !ok || gotBool != wantValue {
						t.Fatalf("%s = %#v, want %v", key, got, wantValue)
					}
				default:
					if got != want {
						t.Fatalf("%s = %#v, want %#v", key, got, want)
					}
				}
			}

			for _, key := range tt.wantAbsent {
				if _, ok := decoded[key]; ok {
					t.Fatalf("decoded JSON unexpectedly included %q in %s", key, string(data))
				}
			}

			if tt.wantWarnings == nil {
				return
			}
			warnings, ok := decoded["warnings"].([]any)
			if !ok || len(warnings) != len(tt.wantWarnings) {
				t.Fatalf("warnings = %#v, want %d entries", decoded["warnings"], len(tt.wantWarnings))
			}
			for idx, want := range tt.wantWarnings {
				if warnings[idx] != want {
					t.Fatalf("warnings[%d] = %#v, want %q", idx, warnings[idx], want)
				}
			}
		})
	}
}

func TestSettingsProviderAuthStatusPayloadJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("Should include auth status code when populated", func(t *testing.T) {
		t.Parallel()

		payload := SettingsProviderAuthStatusPayload{
			Mode:       "native_cli",
			EnvPolicy:  "filtered",
			HomePolicy: "operator",
			State:      "not_authenticated",
			Code:       "provider_not_authenticated",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded["code"] != "provider_not_authenticated" {
			t.Fatalf("code = %#v, want %#v", decoded["code"], "provider_not_authenticated")
		}
	})

	t.Run("Should omit auth status code when empty", func(t *testing.T) {
		t.Parallel()

		payload := SettingsProviderAuthStatusPayload{
			Mode:       "native_cli",
			EnvPolicy:  "filtered",
			HomePolicy: "operator",
			State:      "authenticated",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if _, ok := decoded["code"]; ok {
			t.Fatalf("decoded JSON unexpectedly included code in %s", string(data))
		}
	})
}
