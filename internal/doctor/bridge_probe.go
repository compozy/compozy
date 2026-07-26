package doctor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"
)

const bridgeProbeID = "runtime.bridges"

// BridgeControlSource is the read-only bridge surface required by doctor.
type BridgeControlSource interface {
	ListInstances(context.Context) ([]bridgepkg.BridgeInstance, error)
	CheckBridge(
		context.Context,
		string,
		bridgepkg.BridgeCheckRequest,
	) (bridgepkg.BridgeCheckResponse, error)
}

// BridgeProbe runs the provider-owned checks for every configured bridge.
type BridgeProbe struct {
	Source BridgeControlSource
}

var _ Probe = (*BridgeProbe)(nil)

func (p *BridgeProbe) ID() string { return bridgeProbeID }

func (p *BridgeProbe) Category() string { return contract.CategoryBridge }

func (p *BridgeProbe) Run(
	ctx context.Context,
	_ *ProbeEnv,
) ([]contract.DiagnosticItem, error) {
	if p == nil || p.Source == nil {
		return nil, errors.New("doctor: bridge control source is required")
	}
	instances, err := p.Source.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("doctor: list bridge instances: %w", err)
	}
	sort.Slice(instances, func(left int, right int) bool {
		return instances[left].ID < instances[right].ID
	})
	if len(instances) == 0 {
		return []contract.DiagnosticItem{diagnostics.NewItem(
			"doctor.bridge.empty",
			contract.CodeBridgeReady,
			contract.CategoryBridge,
			"No bridge instances configured",
			"Bridge verification has no configured instances to inspect.",
			contract.SeverityOK,
			contract.FreshnessLive,
		)}, nil
	}

	items := make([]contract.DiagnosticItem, 0, len(instances)*2)
	for _, instance := range instances {
		response, checkErr := p.Source.CheckBridge(ctx, instance.ExtensionName, bridgepkg.BridgeCheckRequest{
			BridgeInstanceID: instance.ID,
		})
		if checkErr != nil {
			items = append(items, bridgeControlErrorItem(instance, checkErr))
			continue
		}
		for _, check := range response.Checks {
			items = append(items, bridgeCheckDiagnosticItem(instance, check))
		}
	}
	return items, nil
}

func bridgeControlErrorItem(instance bridgepkg.BridgeInstance, err error) contract.DiagnosticItem {
	return diagnostics.NewItem(
		bridgeDiagnosticID(instance.ID, "control"),
		contract.CodeBridgeHealthUnavailable,
		contract.CategoryBridge,
		"Bridge verification could not complete",
		"The provider control process did not return bridge checks.",
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand("agh bridge verify "+instance.ID),
		diagnostics.WithEvidence(map[string]any{
			"bridge_instance_id": instance.ID,
			"extension_name":     instance.ExtensionName,
			"error":              err,
		}),
	)
}

func bridgeCheckDiagnosticItem(
	instance bridgepkg.BridgeInstance,
	check bridgepkg.BridgeCheckRecord,
) contract.DiagnosticItem {
	code, severity, title := bridgeDiagnosticPresentation(check.Status)
	message := strings.TrimSpace(check.Remediation)
	if message == "" {
		message = bridgeCheckFallbackMessage(check)
	}
	return diagnostics.NewItem(
		bridgeDiagnosticID(instance.ID, check.Check),
		code,
		contract.CategoryBridge,
		title,
		message,
		severity,
		contract.FreshnessLive,
		diagnostics.WithSuggestedCommand("agh bridge verify "+instance.ID),
		diagnostics.WithEvidence(map[string]any{
			"bridge_instance_id": instance.ID,
			"platform":           instance.Platform,
			"extension_name":     instance.ExtensionName,
			"check":              check.Check,
			"status":             check.Status,
		}),
	)
}

func bridgeCheckFallbackMessage(check bridgepkg.BridgeCheckRecord) string {
	checkName := strings.TrimSpace(check.Check)
	status := bridgepkg.BridgeCheckStatus(strings.ToLower(strings.TrimSpace(string(check.Status))))
	switch status {
	case bridgepkg.BridgeCheckStatusPass:
		return fmt.Sprintf("Bridge check %q passed.", checkName)
	case bridgepkg.BridgeCheckStatusSkipped:
		return fmt.Sprintf("Bridge check %q was skipped without additional detail.", checkName)
	default:
		return fmt.Sprintf(
			"Bridge check %q returned %q with no remediation provided. Inspect provider diagnostics and retry verification.",
			checkName,
			status,
		)
	}
}

func bridgeDiagnosticPresentation(status bridgepkg.BridgeCheckStatus) (string, string, string) {
	switch status {
	case bridgepkg.BridgeCheckStatusPass:
		return contract.CodeBridgeReady, contract.SeverityOK, "Bridge check passed"
	case bridgepkg.BridgeCheckStatusSkipped:
		return contract.CodeBridgeHealthUnavailable, contract.SeverityInfo, "Bridge check skipped"
	case bridgepkg.BridgeCheckStatusWarn:
		return contract.CodeBridgeHealthUnavailable, contract.SeverityWarn, "Bridge check needs attention"
	default:
		return contract.CodeBridgeHealthUnavailable, contract.SeverityError, "Bridge check failed"
	}
}

func bridgeDiagnosticID(instanceID string, check string) string {
	return "doctor.bridge." + diagnosticIDPart(instanceID) + "." + diagnosticIDPart(check)
}

func diagnosticIDPart(value string) string {
	var builder strings.Builder
	separatorPending := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(char), unicode.IsDigit(char):
			if separatorPending && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			separatorPending = false
		case builder.Len() > 0:
			separatorPending = true
		}
	}
	return builder.String()
}
