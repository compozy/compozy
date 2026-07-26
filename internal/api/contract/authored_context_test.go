package contract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	heartbeatpkg "github.com/compozy/agh/internal/heartbeat"
	soulpkg "github.com/compozy/agh/internal/soul"
)

func TestAuthoredContextPayloadJSONShapeAndRedaction(t *testing.T) {
	t.Run("Should keep compact Soul context body-free while allowing the full read surface", func(t *testing.T) {
		t.Parallel()

		contextPayload := AgentContextResponse{
			Context: AgentContextPayload{
				Soul: AgentSoulSectionPayload{
					Enabled:          true,
					Present:          true,
					Active:           true,
					Valid:            true,
					ValidationStatus: AuthoredValidationValid,
					SnapshotID:       "soul-snapshot-1",
					Digest:           "sha256:soul",
					ConfigDigest:     "sha256:config",
					SourcePath:       ".agh/agents/coder/SOUL.md",
					Role:             "reviewer",
					Tone:             []string{"direct"},
					Principles:       []string{"state facts"},
					MaxBytes:         2048,
					MaxBodyBytes:     32768,
				},
			},
		}
		contextJSON := marshalContractString(t, contextPayload)
		if strings.Contains(contextJSON, `"body"`) || strings.Contains(contextJSON, "full persona prose") {
			t.Fatalf("compact context leaked full body: %s", contextJSON)
		}

		fullPayload := AgentSoulPayload{
			Enabled:          true,
			Present:          true,
			Active:           true,
			Valid:            true,
			ValidationStatus: AuthoredValidationValid,
			Digest:           "sha256:soul",
			SourcePath:       ".agh/agents/coder/SOUL.md",
			Frontmatter: AgentSoulFrontmatterPayload{
				Role: "reviewer",
				Tone: []string{"direct"},
			},
			Body: "full persona prose",
			Limits: AuthoredContextLimitsPayload{
				MaxBodyBytes:           32768,
				ContextProjectionBytes: 2048,
			},
			ConfigProvenance: AgentSoulConfigProvenancePayload{
				Digest:                 "sha256:config",
				Enabled:                true,
				MaxBodyBytes:           32768,
				ContextProjectionBytes: 2048,
			},
		}
		fullJSON := marshalContractString(t, fullPayload)
		if !strings.Contains(fullJSON, `"body":"full persona prose"`) {
			t.Fatalf("full Soul read surface omitted body: %s", fullJSON)
		}
		if err := ValidateAuthoredContextRedacted(fullPayload); err != nil {
			t.Fatalf("ValidateAuthoredContextRedacted(fullPayload) error = %v", err)
		}
	})

	t.Run("Should reject raw credential keys and token-shaped values", func(t *testing.T) {
		t.Parallel()

		unsafePayload := map[string]any{
			"policy": map[string]string{
				"provider_token": "raw-provider-token",
			},
		}
		if err := ValidateAuthoredContextRedacted(unsafePayload); !errors.Is(err, ErrUnsafeAuthoredContextPayload) {
			t.Fatalf(
				"ValidateAuthoredContextRedacted(provider_token) error = %v, want ErrUnsafeAuthoredContextPayload",
				err,
			)
		}

		rawClaimPayload := map[string]any{
			"diagnostics": []map[string]string{
				{"message": "contains agh_claim_raw"},
			},
		}
		if err := ValidateAuthoredContextRedacted(rawClaimPayload); !errors.Is(err, ErrUnsafeAuthoredContextPayload) {
			t.Fatalf(
				"ValidateAuthoredContextRedacted(raw claim value) error = %v, want ErrUnsafeAuthoredContextPayload",
				err,
			)
		}

		secretRefPayload := map[string]any{
			"client_secret_ref": "vault://providers/openai",
		}
		if err := ValidateAuthoredContextRedacted(secretRefPayload); !errors.Is(err, ErrUnsafeAuthoredContextPayload) {
			t.Fatalf(
				"ValidateAuthoredContextRedacted(secret ref) error = %v, want ErrUnsafeAuthoredContextPayload",
				err,
			)
		}

		bindingPayload := map[string]any{
			"diagnostics": []string{
				"failed to resolve env:OPENAI_API_KEY",
				"oauth_code=oauth-raw pkce_verifier=pkce-raw",
			},
		}
		if err := ValidateAuthoredContextRedacted(bindingPayload); !errors.Is(err, ErrUnsafeAuthoredContextPayload) {
			t.Fatalf(
				"ValidateAuthoredContextRedacted(secret binding) error = %v, want ErrUnsafeAuthoredContextPayload",
				err,
			)
		}

		safePayload := map[string]any{
			"claim_token_hash": "sha256:redacted",
			"safe_status":      "redacted",
		}
		if err := ValidateAuthoredContextRedacted(safePayload); err != nil {
			t.Fatalf("ValidateAuthoredContextRedacted(safe redacted payload) error = %v", err)
		}
	})
}

func TestAuthoredContextDomainConversions(t *testing.T) {
	t.Run("Should convert Soul resolver output with config provenance", func(t *testing.T) {
		t.Parallel()

		resolved := soulpkg.ResolvedSoul{
			Enabled:    true,
			Present:    true,
			Active:     true,
			Valid:      true,
			SourcePath: ".agh/agents/coder/SOUL.md",
			Digest:     "sha256:soul",
			Compact: soulpkg.CompactProjection{
				Enabled:      true,
				Present:      true,
				Active:       true,
				Digest:       "sha256:soul",
				SourcePath:   ".agh/agents/coder/SOUL.md",
				Role:         "reviewer",
				Tone:         []string{"direct"},
				Principles:   []string{"state facts"},
				MaxBytes:     2048,
				MaxBodyBytes: 32768,
			},
			ReadModel: soulpkg.ReadModel{
				Enabled:                true,
				Present:                true,
				Active:                 true,
				Valid:                  true,
				SourcePath:             ".agh/agents/coder/SOUL.md",
				Digest:                 "sha256:soul",
				Frontmatter:            soulpkg.Frontmatter{Role: "reviewer", Tone: []string{"direct"}},
				Body:                   "full persona prose",
				MaxBodyBytes:           32768,
				ContextProjectionBytes: 2048,
			},
		}
		provenance := soulpkg.ConfigProvenance{
			Digest:                 "sha256:config",
			Source:                 "workspace",
			Enabled:                true,
			MaxBodyBytes:           32768,
			ContextProjectionBytes: 2048,
		}

		section := AgentSoulSectionPayloadFromResolved(&resolved, "soul-snapshot-1", provenance)
		if section.ValidationStatus != AuthoredValidationValid ||
			section.SnapshotID != "soul-snapshot-1" ||
			section.ConfigDigest != "sha256:config" {
			t.Fatalf("AgentSoulSectionPayloadFromResolved() = %#v", section)
		}

		full := AgentSoulPayloadFromResolved("coder", &resolved, "soul-snapshot-1", provenance)
		if full.AgentName != "coder" ||
			full.Body != "full persona prose" ||
			full.ConfigProvenance.Source != "workspace" ||
			full.Frontmatter.Role != "reviewer" {
			t.Fatalf("AgentSoulPayloadFromResolved() = %#v", full)
		}
	})

	t.Run("Should reject unsupported session health enum values during conversion", func(t *testing.T) {
		t.Parallel()

		_, err := SessionHealthPayloadFromDomain(heartbeatpkg.SessionHealth{
			SessionID:       "sess-1",
			WorkspaceID:     "ws-1",
			AgentName:       "coder",
			State:           heartbeatpkg.SessionHealthState("running"),
			Health:          heartbeatpkg.SessionHealthHealthy,
			EligibleForWake: true,
			UpdatedAt:       time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, ErrInvalidAuthoredContextEnum) {
			t.Fatalf("SessionHealthPayloadFromDomain() error = %v, want ErrInvalidAuthoredContextEnum", err)
		}
	})

	t.Run("Should redact session health last error before returning public payload", func(t *testing.T) {
		t.Parallel()

		payload, err := SessionHealthPayloadFromDomain(heartbeatpkg.SessionHealth{
			SessionID:       "sess-1",
			WorkspaceID:     "ws-1",
			AgentName:       "coder",
			State:           heartbeatpkg.SessionHealthStateIdle,
			Health:          heartbeatpkg.SessionHealthDegraded,
			EligibleForWake: true,
			LastError:       `failed oauth_code=oauth-raw secret_ref=env:OPENAI_API_KEY Bearer bearer-raw`,
			UpdatedAt:       time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("SessionHealthPayloadFromDomain() error = %v", err)
		}
		for _, leaked := range []string{
			"oauth-raw",
			"env:OPENAI_API_KEY",
			"bearer-raw",
		} {
			if strings.Contains(payload.LastError, leaked) {
				t.Fatalf("SessionHealthPayloadFromDomain().LastError = %q leaked %q", payload.LastError, leaked)
			}
		}
		if err := ValidateAuthoredContextRedacted(payload); err != nil {
			t.Fatalf("ValidateAuthoredContextRedacted(session health payload) error = %v", err)
		}
	})

	t.Run("Should convert wake decisions with closed result and reason enums", func(t *testing.T) {
		t.Parallel()

		decision, err := HeartbeatWakeDecisionPayloadFromDomain(heartbeatpkg.WakeDecision{
			WakeEventID:       "hwe-1",
			Result:            heartbeatpkg.WakeResultSent,
			Reason:            heartbeatpkg.WakeReasonSent,
			PolicySnapshotID:  "hb-1",
			PolicyDigest:      "sha256:policy",
			ConfigDigest:      "sha256:config",
			SyntheticPromptID: "turn-1",
		})
		if err != nil {
			t.Fatalf("HeartbeatWakeDecisionPayloadFromDomain() error = %v", err)
		}
		if decision.Result != HeartbeatWakeResultSent ||
			decision.Reason != HeartbeatWakeReasonSent ||
			decision.PolicySnapshotID != "hb-1" {
			t.Fatalf("HeartbeatWakeDecisionPayloadFromDomain() = %#v", decision)
		}
	})
}

func TestAuthoredContextClosedEnumsRejectUnknownValues(t *testing.T) {
	testCases := []struct {
		name string
		err  error
	}{
		{name: "ShouldRejectUnknownValidationStatus", err: AuthoredValidationStatus("pending").Validate()},
		{name: "ShouldRejectUnknownDiagnosticSeverity", err: AuthoredDiagnosticSeverity("fatal").Validate()},
		{name: "ShouldRejectUnknownHealthState", err: SessionHealthState("attached").Validate()},
		{name: "ShouldRejectUnknownHealthStatus", err: SessionHealthStatus("lost").Validate()},
		{name: "ShouldRejectUnknownWakeReason", err: HeartbeatWakeReason("task_claimed").Validate()},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tc.err, ErrInvalidAuthoredContextEnum) {
				t.Fatalf("%s error = %v, want ErrInvalidAuthoredContextEnum", tc.name, tc.err)
			}
		})
	}
}

func TestAuthoredContextCASRequestsUseBodyExpectedDigest(t *testing.T) {
	testCases := []struct {
		name    string
		request any
	}{
		{
			name: "ShouldUseBodyExpectedDigestForSoulPut",
			request: AgentSoulPutRequest{
				AgentName:      "coder",
				Body:           "body",
				ExpectedDigest: "sha256:current",
			},
		},
		{
			name: "ShouldUseBodyExpectedDigestForSoulDelete",
			request: AgentSoulDeleteRequest{
				AgentName:      "coder",
				ExpectedDigest: "sha256:current",
			},
		},
		{
			name: "ShouldUseBodyExpectedDigestForSoulRollback",
			request: AgentSoulRollbackRequest{
				AgentName:      "coder",
				RevisionID:     "rev-1",
				ExpectedDigest: "sha256:current",
			},
		},
		{
			name: "ShouldUseBodyExpectedDigestForHeartbeatPut",
			request: HeartbeatPutRequest{
				AgentName:      "coder",
				Body:           "body",
				ExpectedDigest: "sha256:current",
			},
		},
		{
			name: "ShouldUseBodyExpectedDigestForHeartbeatDelete",
			request: HeartbeatDeleteRequest{
				AgentName:      "coder",
				ExpectedDigest: "sha256:current",
			},
		},
		{
			name: "ShouldUseBodyExpectedDigestForHeartbeatRollback",
			request: HeartbeatRollbackRequest{
				AgentName:      "coder",
				RevisionID:     "rev-1",
				ExpectedDigest: "sha256:current",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw := marshalContractString(t, tc.request)
			if !strings.Contains(raw, `"expected_digest":"sha256:current"`) {
				t.Fatalf("request JSON missing body expected_digest: %s", raw)
			}
			if strings.Contains(raw, "If-Match") || strings.Contains(raw, "if_match") {
				t.Fatalf("request JSON exposed transport-specific If-Match shape: %s", raw)
			}
		})
	}
}

func TestAuthoredContextConfigProvenanceSerializesDeterministically(t *testing.T) {
	t.Run("Should preserve concrete Heartbeat config subset order", func(t *testing.T) {
		t.Parallel()

		payload := HeartbeatConfigProvenancePayload{
			Digest: "sha256:config",
			Subset: HeartbeatConfigSubsetPayload{
				Enabled:                      true,
				MaxBodyBytes:                 32768,
				ContextProjectionBytes:       2048,
				MinInterval:                  "5m0s",
				DefaultInterval:              "15m0s",
				WakeCooldown:                 "1m0s",
				MaxWakesPerCycle:             25,
				ActiveSessionOnly:            true,
				AllowActiveHoursPreferences:  true,
				WakeEventRetention:           "168h0m0s",
				SessionHealthStaleAfter:      "2m0s",
				SessionHealthHookMinInterval: "1m0s",
			},
		}

		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(HeartbeatConfigProvenancePayload) error = %v", err)
		}
		want := `{"digest":"sha256:config","subset":{"enabled":true,"max_body_bytes":32768,"context_projection_bytes":2048,"min_interval":"5m0s","default_interval":"15m0s","wake_cooldown":"1m0s","max_wakes_per_cycle":25,"active_session_only":true,"allow_active_hours_preferences":true,"wake_event_retention":"168h0m0s","session_health_stale_after":"2m0s","session_health_hook_min_interval":"1m0s"}}`
		if string(encoded) != want {
			t.Fatalf("HeartbeatConfigProvenancePayload JSON = %s, want %s", encoded, want)
		}
	})
}
