package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	registrypkg "github.com/compozy/agh/internal/registry"
)

var (
	// ErrExtensionUnverifiedPolicyBlocked reports a side-load denied by daemon policy.
	ErrExtensionUnverifiedPolicyBlocked = errors.New("extension: unverified install blocked by policy")
	// ErrExtensionArchiveDigestMismatch reports a curated archive that differs from its feed pin.
	ErrExtensionArchiveDigestMismatch = errors.New("extension: curated archive digest mismatch")
)

// MarketplaceCatalogRegistryName identifies feed-owned extension artifacts in installed provenance.
const MarketplaceCatalogRegistryName = "agh-catalog"

// MarketplaceTrustEvidence contains feed-authoritative install metadata.
type MarketplaceTrustEvidence struct {
	CatalogEntryID      string
	Version             string
	ArchiveDigestSHA256 string
	RegistryTier        string
	ArtifactURL         string
	Repository          string
}

// MarketplaceTrustResolver resolves curated evidence; nil marks a non-curated side-load.
type MarketplaceTrustResolver func(
	context.Context,
	string,
	string,
) (*MarketplaceTrustEvidence, error)

// MarketplaceDigestVerificationObserver receives a pre-extraction verification outcome.
type MarketplaceDigestVerificationObserver func(*MarketplaceTrustEvidence, error)

// MarketplaceEntryTrustReport projects feed evidence and live policy before acquisition.
func MarketplaceEntryTrustReport(
	evidence MarketplaceTrustEvidence,
	policyAllowsUnverified bool,
) (contract.ExtensionTrustReportPayload, error) {
	if err := evidence.validate(evidence.CatalogEntryID); err != nil {
		return contract.ExtensionTrustReportPayload{}, err
	}
	tier := strings.ToLower(strings.TrimSpace(evidence.RegistryTier))
	report := contract.ExtensionTrustReportPayload{
		RegistryTier:     tier,
		ChecksumVerified: false,
		AllowUnverified:  policyAllowsUnverified,
	}
	switch tier {
	case ExtensionRegistryTierOfficial, ExtensionRegistryTierCommunity:
		report.Decision = ExtensionTrustDecisionVerified
	case ExtensionRegistryTierUnverified:
		report.Decision = ExtensionTrustDecisionBlocked
		if policyAllowsUnverified {
			report.Decision = ExtensionTrustDecisionAllowedUnverified
		}
		report.Warnings = []diagnosticcontract.DiagnosticItem{
			marketplaceRegistryTierUnverifiedDiagnostic(evidence),
		}
	default:
		return contract.ExtensionTrustReportPayload{}, fmt.Errorf(
			"extension: curated marketplace registry tier %q is unsupported",
			tier,
		)
	}
	return report, nil
}

// ValidateUnverifiedSideLoad enforces daemon policy plus per-request consent.
func ValidateUnverifiedSideLoad(
	slug string,
	source string,
	policyAllowsUnverified bool,
	allowUnverified bool,
) error {
	return validateMarketplaceTrustGate(slug, source, policyAllowsUnverified, allowUnverified, nil)
}

type ExtensionPolicyBlockedError struct {
	Slug string
	Item diagnosticcontract.DiagnosticItem
}

func (e *ExtensionPolicyBlockedError) Error() string {
	if e == nil || strings.TrimSpace(e.Slug) == "" {
		return ErrExtensionUnverifiedPolicyBlocked.Error()
	}
	return fmt.Sprintf("%s: %s", ErrExtensionUnverifiedPolicyBlocked, strings.TrimSpace(e.Slug))
}

func (e *ExtensionPolicyBlockedError) Unwrap() error {
	return ErrExtensionUnverifiedPolicyBlocked
}

func (e *ExtensionPolicyBlockedError) DiagnosticItem() diagnosticcontract.DiagnosticItem {
	if e == nil {
		return diagnostics.EmptyItem()
	}
	return e.Item
}

type ExtensionArchiveDigestMismatchError struct {
	EntryID string
	Version string
	Cause   error
	Item    diagnosticcontract.DiagnosticItem
}

func (e *ExtensionArchiveDigestMismatchError) Error() string {
	if e == nil {
		return ErrExtensionArchiveDigestMismatch.Error()
	}
	return fmt.Sprintf(
		"%s for entry %q version %q",
		ErrExtensionArchiveDigestMismatch,
		strings.TrimSpace(e.EntryID),
		strings.TrimSpace(e.Version),
	)
}

func (e *ExtensionArchiveDigestMismatchError) Unwrap() error {
	if e == nil || e.Cause == nil {
		return ErrExtensionArchiveDigestMismatch
	}
	return e.Cause
}

func (e *ExtensionArchiveDigestMismatchError) Is(target error) bool {
	if e == nil {
		return target == ErrExtensionArchiveDigestMismatch
	}
	return target == ErrExtensionArchiveDigestMismatch || errors.Is(e.Cause, target)
}

func (e *ExtensionArchiveDigestMismatchError) DiagnosticItem() diagnosticcontract.DiagnosticItem {
	if e == nil {
		return diagnostics.EmptyItem()
	}
	return e.Item
}

func validateMarketplaceTrustGate(
	slug string,
	source string,
	policyAllowsUnverified bool,
	allowUnverified bool,
	trust *MarketplaceTrustEvidence,
) error {
	if trust != nil {
		if err := trust.validate(slug); err != nil {
			return err
		}
		if normalizedMarketplaceRegistryTier(trust.RegistryTier) != ExtensionRegistryTierUnverified {
			return nil
		}
	}
	if !policyAllowsUnverified {
		return newExtensionPolicyBlockedError(slug, source)
	}
	if !allowUnverified {
		return newExtensionChecksumUnverifiedError(slug, source)
	}
	return nil
}

func (e *MarketplaceTrustEvidence) validate(slug string) error {
	if e == nil {
		return nil
	}
	switch {
	case strings.TrimSpace(e.CatalogEntryID) == "":
		return fmt.Errorf("extension: curated marketplace entry id is required for %q", strings.TrimSpace(slug))
	case strings.TrimSpace(e.Version) == "":
		return fmt.Errorf("extension: curated marketplace version is required for %q", strings.TrimSpace(slug))
	case strings.TrimSpace(e.ArchiveDigestSHA256) == "":
		return fmt.Errorf("extension: curated marketplace archive digest is required for %q", strings.TrimSpace(slug))
	case strings.TrimSpace(e.RegistryTier) == "":
		return fmt.Errorf("extension: curated marketplace registry tier is required for %q", strings.TrimSpace(slug))
	}
	tier := normalizedMarketplaceRegistryTier(e.RegistryTier)
	switch tier {
	case ExtensionRegistryTierOfficial, ExtensionRegistryTierCommunity, ExtensionRegistryTierUnverified:
		return nil
	default:
		return fmt.Errorf("extension: curated marketplace registry tier %q is unsupported", tier)
	}
}

func normalizedMarketplaceRegistryTier(tier string) string {
	return strings.ToLower(strings.TrimSpace(tier))
}

func marketplaceTrustWarnings(evidence *MarketplaceTrustEvidence) []diagnosticcontract.DiagnosticItem {
	if evidence == nil ||
		normalizedMarketplaceRegistryTier(evidence.RegistryTier) != ExtensionRegistryTierUnverified {
		return nil
	}
	return []diagnosticcontract.DiagnosticItem{marketplaceRegistryTierUnverifiedDiagnostic(*evidence)}
}

func newExtensionPolicyBlockedError(slug string, source string) *ExtensionPolicyBlockedError {
	item := diagnostics.NewItem(
		"extension.unverified_policy_blocked",
		diagnosticcontract.CodeExtensionUnverifiedPolicyBlocked,
		diagnosticcontract.CategoryExtension,
		"Unverified extension install is blocked",
		"This side-load is disabled by extensions marketplace policy. "+
			"Enable it in Settings › Extensions, then confirm the individual install.",
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithSuggestedCommand("agh config set extensions.marketplace.allow_unverified true"),
		diagnostics.WithEvidence(map[string]any{
			"slug":                          strings.TrimSpace(slug),
			extensionTrustEvidenceSourceKey: strings.TrimSpace(source),
			"settings_section":              "Settings › Extensions",
			"settings_path":                 "/settings/extensions",
		}),
	)
	return &ExtensionPolicyBlockedError{Slug: strings.TrimSpace(slug), Item: item}
}

func marketplaceRegistryTierUnverifiedDiagnostic(
	evidence MarketplaceTrustEvidence,
) diagnosticcontract.DiagnosticItem {
	return diagnostics.NewItem(
		"extension.registry_tier_unverified",
		diagnosticcontract.CodeExtensionRegistryTierUnverified,
		diagnosticcontract.CategoryExtension,
		"Extension publisher tier is unverified",
		"The catalog pins this archive digest, but the publisher is not in a trusted registry tier. "+
			"Review the source before installing.",
		diagnosticcontract.SeverityWarn,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"catalog_entry_id":               strings.TrimSpace(evidence.CatalogEntryID),
			"registry_tier":                  strings.TrimSpace(evidence.RegistryTier),
			extensionTrustEvidenceVersionKey: strings.TrimSpace(evidence.Version),
		}),
	)
}

func wrapCuratedDigestMismatch(
	err error,
	trust *MarketplaceTrustEvidence,
) error {
	if !errors.Is(err, registrypkg.ErrArchiveDigestMismatch) || trust == nil {
		return err
	}
	entryID := strings.TrimSpace(trust.CatalogEntryID)
	version := strings.TrimSpace(trust.Version)
	item := diagnostics.NewItem(
		"extension.archive_digest_mismatch",
		diagnosticcontract.CodeExtensionArchiveDigestMismatch,
		diagnosticcontract.CategoryExtension,
		"Curated extension archive does not match its digest",
		"The downloaded archive differs from the version reviewed in the curated catalog. Nothing was installed.",
		diagnosticcontract.SeverityError,
		diagnosticcontract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{"entry_id": entryID, extensionTrustEvidenceVersionKey: version}),
	)
	return &ExtensionArchiveDigestMismatchError{
		EntryID: entryID,
		Version: version,
		Cause:   err,
		Item:    item,
	}
}
