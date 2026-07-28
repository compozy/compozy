package extensionpkg

import (
	"testing"
	"time"
)

func TestDescribeExtension(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 18, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		extension  *Extension
		active     bool
		now        time.Time
		wantType   string
		wantState  string
		wantHealth string
		wantUptime int64
	}{
		{
			name: "Should report active subprocess runtime",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "telegram-adapter",
					Version: "1.2.3",
					Source:  SourceUser,
					Enabled: true,
					Capabilities: CapabilitiesConfig{
						Provides: []string{"bridge.adapter"},
					},
					Actions: ActionsConfig{
						Requires: []string{"bridges/messages/ingest"},
					},
				},
				Status: ExtensionStatus{
					Active:        true,
					Healthy:       true,
					PID:           4242,
					LastStartedAt: now.Add(-15 * time.Minute),
				},
			},
			active:     true,
			now:        now,
			wantType:   "subprocess",
			wantState:  "active",
			wantHealth: "healthy",
			wantUptime: 900,
		},
		{
			name: "Should report registered resource health",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "workspace-review",
					Version: "0.1.0",
					Source:  SourceWorkspace,
					Enabled: true,
				},
				Status: ExtensionStatus{
					Registered: true,
				},
			},
			active:     true,
			now:        now,
			wantType:   "resource",
			wantState:  "registered",
			wantHealth: "healthy",
			wantUptime: 0,
		},
		{
			name: "Should report missing environment requirements",
			extension: &Extension{
				Info: ExtensionInfo{
					Name:    "env-ext",
					Version: "0.1.0",
					Source:  SourceUser,
					Enabled: true,
				},
				Manifest: &Manifest{
					RequiresEnv: []string{"PRESENT_TOKEN", "MISSING_TOKEN"},
				},
				Status: ExtensionStatus{
					Registered:        true,
					MissingEnv:        []string{"MISSING_TOKEN"},
					MissingEnvChecked: true,
				},
			},
			active:     true,
			now:        now,
			wantType:   "resource",
			wantState:  "registered",
			wantHealth: "healthy",
			wantUptime: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := DescribeExtension(tt.extension, tt.active, tt.now)
			if payload.Type != tt.wantType {
				t.Fatalf("DescribeExtension().Type = %q, want %q", payload.Type, tt.wantType)
			}
			if payload.State != tt.wantState {
				t.Fatalf("DescribeExtension().State = %q, want %q", payload.State, tt.wantState)
			}
			if payload.Health != tt.wantHealth {
				t.Fatalf("DescribeExtension().Health = %q, want %q", payload.Health, tt.wantHealth)
			}
			if payload.UptimeSeconds != tt.wantUptime {
				t.Fatalf("DescribeExtension().UptimeSeconds = %d, want %d", payload.UptimeSeconds, tt.wantUptime)
			}
			if tt.name == "Should report missing environment requirements" {
				if got, want := payload.RequiresEnv, []string{"PRESENT_TOKEN", "MISSING_TOKEN"}; len(got) != len(want) {
					t.Fatalf("DescribeExtension().RequiresEnv = %#v, want %#v", got, want)
				}
				if got, want := payload.MissingEnv, []string{
					"MISSING_TOKEN",
				}; len(got) != len(want) ||
					got[0] != want[0] {
					t.Fatalf("DescribeExtension().MissingEnv = %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestDescribeExtensionProjectsCuratedArchiveIdentity(t *testing.T) {
	t.Parallel()
	t.Run("Should project distinct curated archive and installed tree identity", func(t *testing.T) {
		t.Parallel()

		payload := DescribeExtension(&Extension{Info: ExtensionInfo{
			Name: "curated", Version: "1.0.0", Source: SourceMarketplace,
			Provenance: ExtensionProvenance{
				Slug: "acme/curated", CatalogEntryID: "extension.acme.curated",
				InstalledFrom:  ExtensionInstalledFromMarketplace,
				ChecksumSHA256: "tree-digest", ArchiveDigestSHA256: "archive-digest",
				ChecksumVerified: true, RegistryTier: ExtensionRegistryTierOfficial,
			},
		}}, false, time.Now())
		if payload.Provenance == nil {
			t.Fatal("DescribeExtension().Provenance = nil")
		}
		if payload.Provenance.CatalogEntryID != "extension.acme.curated" ||
			payload.Provenance.ArchiveDigestSHA256 != "archive-digest" ||
			payload.Provenance.ChecksumSHA256 != "tree-digest" {
			t.Fatalf(
				"DescribeExtension().Provenance = %#v, want separate catalog/archive/tree identity",
				payload.Provenance,
			)
		}
		if payload.Provenance.Trust == nil || payload.Provenance.Trust.Decision != ExtensionTrustDecisionVerified {
			t.Fatalf("DescribeExtension().Provenance.Trust = %#v, want verified", payload.Provenance.Trust)
		}

		allowed := extensionProvenancePayload(ExtensionProvenance{
			InstalledFrom:   ExtensionInstalledFromLocalPath,
			RegistryTier:    ExtensionRegistryTierUnverified,
			AllowUnverified: true,
		})
		if allowed == nil || allowed.Trust == nil || allowed.Trust.Decision != ExtensionTrustDecisionAllowedUnverified {
			t.Fatalf("allowed side-load trust = %#v, want allowed_unverified", allowed)
		}
		blocked := extensionProvenancePayload(ExtensionProvenance{
			InstalledFrom: ExtensionInstalledFromLocalPath,
			RegistryTier:  ExtensionRegistryTierUnverified,
		})
		if blocked == nil || blocked.Trust == nil || blocked.Trust.Decision != ExtensionTrustDecisionBlocked {
			t.Fatalf("blocked side-load trust = %#v, want blocked", blocked)
		}
	})
}
