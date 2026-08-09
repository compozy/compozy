package gateway

import (
	"reflect"
	"testing"
	"time"
)

func TestGatewayAudit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	t.Run("Should report complete ranked posture with stable remediation [UT-130]", func(t *testing.T) {
		t.Parallel()

		status := Status{
			Enabled: true,
			Tiers: []TierStatus{
				{Tier: TierPrivate, Desired: DesiredEnabled, Observed: string(ProviderDegraded)},
				{Tier: TierPublic, Desired: DesiredEnabled, Observed: string(ProviderUp), Advertised: true},
			},
			Surfaces: []SurfaceStatus{
				{
					Surface: SurfaceOperatorUI, Tier: TierPrivate,
					Desired: DesiredEnabled, Observed: SurfaceOff, Generation: 3,
				},
				{
					Surface: SurfaceOperatorUI, Tier: TierPublic,
					Desired: DesiredEnabled, Observed: SurfaceOn, Generation: 7,
				},
			},
			Providers: []ProviderStatus{{
				Name: "connectivity-test", Tier: TierPrivate,
				Desired: DesiredEnabled, Observed: ProviderDegraded, Health: HealthDegraded,
			}},
			Addresses: []AddressStatus{{Tier: TierPublic, Address: "https://gateway.test", Live: true}},
			Devices: []DeviceSession{
				{ID: "dev_stale", CreatedAt: now.Add(-100 * 24 * time.Hour)},
				{ID: "dev_recent", CreatedAt: now.Add(-time.Hour)},
				{ID: "dev_revoked", CreatedAt: now.Add(-time.Hour), RevokedAt: now.Add(-time.Minute)},
			},
			Bindings: []IngressBindingStatus{{
				Subject:      IngressSubjectRef{Kind: IngressSubjectWebhookTrigger, ID: "trigger-1"},
				Reachability: IngressReachabilityReconfirmation,
			}},
		}

		report := auditStatus(status, now)
		if !report.Ran || report.NoFindings || report.LocalOnly {
			t.Fatalf("audit state = %#v, want completed findings for remote posture", report)
		}
		if report.Auth.Mode != "device_session" || !report.Auth.RequiredRemotely ||
			report.Auth.ActiveDeviceCount != 2 {
			t.Fatalf("auth posture = %#v", report.Auth)
		}
		if report.DeviceHighlights.Stale != 1 || report.DeviceHighlights.RecentlyPaired != 1 ||
			report.DeviceHighlights.Revoked != 1 {
			t.Fatalf("device highlights = %#v", report.DeviceHighlights)
		}
		if len(report.Findings) < 5 {
			t.Fatalf(
				"findings = %#v, want provider, drift, public UI, ingress, and stale-device findings",
				report.Findings,
			)
		}
		findingIDs := make(map[string]struct{}, len(report.Findings))
		for index := 1; index < len(report.Findings); index++ {
			if auditSeverityRank(report.Findings[index-1].Severity) >
				auditSeverityRank(report.Findings[index].Severity) {
				t.Fatalf("findings are not severity-ranked: %#v", report.Findings)
			}
		}
		for _, finding := range report.Findings {
			if finding.ID == "" || finding.Severity == "" || finding.Summary == "" || finding.Remediation == "" {
				t.Fatalf("finding is not actionable: %#v", finding)
			}
			if _, exists := findingIDs[finding.ID]; exists {
				t.Fatalf("finding id %q is duplicated", finding.ID)
			}
			findingIDs[finding.ID] = struct{}{}
		}
	})

	t.Run("Should return provider outage as a finding instead of an audit error [UT-131]", func(t *testing.T) {
		t.Parallel()

		report := auditStatus(Status{Providers: []ProviderStatus{{
			Name: "connectivity-test", Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: ProviderDegraded, Health: HealthDegraded,
			Cause: "provider unavailable",
		}}}, now)
		if !report.Ran || report.NoFindings || len(report.Findings) != 1 {
			t.Fatalf("provider outage audit = %#v", report)
		}
		if got := report.Findings[0].ID; got != "gateway.provider.unhealthy.private" {
			t.Fatalf("finding id = %q", got)
		}
	})

	t.Run("Should distinguish an explicit no-findings result from a missing run [UT-132]", func(t *testing.T) {
		t.Parallel()

		report := auditStatus(Status{
			Enabled: false,
			Tiers: []TierStatus{
				{Tier: TierPrivate, Desired: DesiredDisabled, Observed: string(ProviderDown)},
				{Tier: TierPublic, Desired: DesiredDisabled, Observed: string(ProviderDown)},
			},
		}, now)
		if !report.Ran || !report.NoFindings || !report.LocalOnly || report.Findings == nil ||
			len(report.Findings) != 0 {
			t.Fatalf("local-only audit = %#v", report)
		}
	})

	t.Run("Should be stable and side-effect-free across repeated runs [UT-133]", func(t *testing.T) {
		t.Parallel()

		status := Status{Providers: []ProviderStatus{{
			Name: "connectivity-test", Tier: TierPrivate,
			Desired: DesiredEnabled, Observed: ProviderDown, Health: HealthDown,
		}}}
		before := cloneAuditStatus(status)
		first := auditStatus(status, now)
		second := auditStatus(status, now)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("repeated audit differs:\nfirst=%#v\nsecond=%#v", first, second)
		}
		if !reflect.DeepEqual(status, before) {
			t.Fatalf("audit mutated status:\nbefore=%#v\nafter=%#v", before, status)
		}
	})
}

func cloneAuditStatus(status Status) Status {
	cloned := status
	cloned.Tiers = append([]TierStatus(nil), status.Tiers...)
	cloned.Surfaces = append([]SurfaceStatus(nil), status.Surfaces...)
	cloned.Providers = append([]ProviderStatus(nil), status.Providers...)
	cloned.Addresses = append([]AddressStatus(nil), status.Addresses...)
	cloned.Devices = append([]DeviceSession(nil), status.Devices...)
	cloned.Bindings = append([]IngressBindingStatus(nil), status.Bindings...)
	return cloned
}
