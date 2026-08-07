package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	auditStaleDeviceAfter = 90 * 24 * time.Hour
	auditRecentPairingAge = 24 * time.Hour
)

// AuditSeverity ranks one actionable gateway posture finding.
type AuditSeverity string

const (
	AuditSeverityCritical AuditSeverity = "critical"
	AuditSeverityError    AuditSeverity = "error"
	AuditSeverityWarning  AuditSeverity = "warning"
	AuditSeverityInfo     AuditSeverity = "info"
)

// AuditFinding is one stable, machine-actionable gateway posture finding.
type AuditFinding struct {
	ID          string
	Severity    AuditSeverity
	Summary     string
	Remediation string
	Tier        Tier
	Resource    string
}

// AuditAuthPosture describes the authentication boundary without exposing credentials.
type AuditAuthPosture struct {
	Mode              string
	RequiredRemotely  bool
	ActiveDeviceCount int
}

// AuditDeviceHighlights summarizes inventory signals used during routine review.
type AuditDeviceHighlights struct {
	Active         int
	Revoked        int
	Stale          int
	RecentlyPaired int
	StaleDeviceIDs []string
}

// AuditReport is the side-effect-free result of one completed gateway audit.
type AuditReport struct {
	Ran              bool
	NoFindings       bool
	LocalOnly        bool
	Status           Status
	Auth             AuditAuthPosture
	DeviceHighlights AuditDeviceHighlights
	Findings         []AuditFinding
}

// Audit reads the current posture and computes deterministic findings without mutating state.
func (s *Service) Audit(ctx context.Context) (AuditReport, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return AuditReport{}, err
	}
	now := time.Now
	if s != nil && s.devices != nil && s.devices.now != nil {
		now = s.devices.now
	}
	return auditStatus(status, now().UTC()), nil
}

func auditStatus(status Status, now time.Time) AuditReport {
	highlights := auditDeviceHighlights(status.Devices, now)
	report := AuditReport{
		Ran:       true,
		LocalOnly: auditLocalOnly(status),
		Status:    status,
		Auth: AuditAuthPosture{
			Mode:              "device_session",
			RequiredRemotely:  true,
			ActiveDeviceCount: highlights.Active,
		},
		DeviceHighlights: highlights,
		Findings:         auditFindings(status, highlights),
	}
	report.NoFindings = len(report.Findings) == 0
	return report
}

func auditDeviceHighlights(devices []DeviceSession, now time.Time) AuditDeviceHighlights {
	highlights := AuditDeviceHighlights{StaleDeviceIDs: []string{}}
	for _, device := range devices {
		if !device.RevokedAt.IsZero() {
			highlights.Revoked++
			continue
		}
		highlights.Active++
		if !device.CreatedAt.IsZero() && now.Sub(device.CreatedAt) <= auditRecentPairingAge {
			highlights.RecentlyPaired++
		}
		lastActivity := device.LastSeenAt
		if lastActivity.IsZero() {
			lastActivity = device.CreatedAt
		}
		if !lastActivity.IsZero() && now.Sub(lastActivity) > auditStaleDeviceAfter {
			highlights.Stale++
			highlights.StaleDeviceIDs = append(highlights.StaleDeviceIDs, strings.TrimSpace(device.ID))
		}
	}
	sort.Strings(highlights.StaleDeviceIDs)
	return highlights
}

func auditLocalOnly(status Status) bool {
	for _, tier := range status.Tiers {
		if tier.Advertised {
			return false
		}
	}
	return len(status.Addresses) == 0
}

func auditFindings(status Status, highlights AuditDeviceHighlights) []AuditFinding {
	findings := make([]AuditFinding, 0)
	if status.Refusal != nil {
		findings = append(findings, AuditFinding{
			ID: "gateway.exposure.refused", Severity: AuditSeverityCritical,
			Summary:     "Gateway exposure is refused by the active safety policy.",
			Remediation: strings.TrimSpace(status.Refusal.Fix),
		})
	}
	findings = append(findings, auditProviderFindings(status.Providers)...)
	findings = append(findings, auditSurfaceFindings(status.Surfaces)...)
	findings = append(findings, auditIngressFindings(status.Bindings)...)
	if highlights.Stale > 0 {
		findings = append(findings, AuditFinding{
			ID: "gateway.devices.stale", Severity: AuditSeverityWarning,
			Summary:     fmt.Sprintf("%d active gateway device(s) have been idle for more than 90 days.", highlights.Stale),
			Remediation: "Review the stale device IDs and run `compozy device revoke <device-id>` for each device no longer in use.",
			Resource:    strings.Join(highlights.StaleDeviceIDs, ","),
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		left := auditSeverityRank(findings[i].Severity)
		right := auditSeverityRank(findings[j].Severity)
		if left != right {
			return left < right
		}
		return findings[i].ID < findings[j].ID
	})
	return findings
}

func auditProviderFindings(providers []ProviderStatus) []AuditFinding {
	findings := make([]AuditFinding, 0)
	for _, provider := range providers {
		if provider.Desired != DesiredEnabled || provider.Observed == ProviderUp {
			continue
		}
		severity := AuditSeverityError
		if provider.Observed == ProviderEstablishing {
			severity = AuditSeverityWarning
		}
		findings = append(findings, AuditFinding{
			ID:       "gateway.provider.unhealthy." + string(provider.Tier),
			Severity: severity,
			Summary: fmt.Sprintf(
				"Gateway provider %q for the %s tier is %s.",
				strings.TrimSpace(provider.Name), provider.Tier, provider.Observed,
			),
			Remediation: fmt.Sprintf(
				"Repair provider %q, or run `compozy gateway provider disable %s --tier %s` before enabling it again.",
				strings.TrimSpace(provider.Name), strings.TrimSpace(provider.Name), provider.Tier,
			),
			Tier: provider.Tier, Resource: strings.TrimSpace(provider.Name),
		})
	}
	return findings
}

func auditSurfaceFindings(surfaces []SurfaceStatus) []AuditFinding {
	findings := make([]AuditFinding, 0)
	for _, surface := range surfaces {
		observedEnabled := surface.Observed == SurfaceOn
		desiredEnabled := surface.Desired == DesiredEnabled
		if observedEnabled != desiredEnabled {
			findings = append(findings, AuditFinding{
				ID:       "gateway.surface.drift." + string(surface.Tier) + "." + string(surface.Surface),
				Severity: AuditSeverityError,
				Summary: fmt.Sprintf(
					"Gateway surface %s on the %s tier has not reached its desired state.",
					surface.Surface, surface.Tier,
				),
				Remediation: "Wait for reconciliation, then inspect provider health and rerun `compozy gateway audit`.",
				Tier:        surface.Tier, Resource: string(surface.Surface),
			})
		}
		if surface.Tier == TierPublic && surface.Surface == SurfaceOperatorUI &&
			desiredEnabled && observedEnabled {
			findings = append(findings, AuditFinding{
				ID: "gateway.public.operator_ui_exposed", Severity: AuditSeverityWarning,
				Summary: "The operator UI and management API are reachable on the public tier.",
				Remediation: fmt.Sprintf(
					"Run `compozy gateway surface disable operator_ui --tier public --generation %d` when public operator access is not required.",
					surface.Generation,
				),
				Tier: TierPublic, Resource: string(SurfaceOperatorUI),
			})
		}
	}
	return findings
}

func auditIngressFindings(bindings []IngressBindingStatus) []AuditFinding {
	findings := make([]AuditFinding, 0)
	for _, binding := range bindings {
		if binding.Reachability == IngressReachabilityLive {
			continue
		}
		findings = append(findings, AuditFinding{
			ID: "gateway.ingress." + string(binding.Reachability) + "." +
				string(binding.Subject.Kind) + "." + auditResourceRef(binding.Subject.ID),
			Severity: AuditSeverityWarning,
			Summary: fmt.Sprintf(
				"Gateway ingress %s %q is %s.",
				binding.Subject.Kind, strings.TrimSpace(binding.Subject.ID), binding.Reachability,
			),
			Remediation: "Restore a verified public ingress endpoint, then confirm the subject again through `/api/gateway/ingress-bindings`.",
			Tier:        TierPublic, Resource: strings.TrimSpace(binding.Subject.ID),
		})
	}
	return findings
}

func auditResourceRef(value string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(digest[:6])
}

func auditSeverityRank(severity AuditSeverity) int {
	switch severity {
	case AuditSeverityCritical:
		return 0
	case AuditSeverityError:
		return 1
	case AuditSeverityWarning:
		return 2
	case AuditSeverityInfo:
		return 3
	default:
		return 4
	}
}
