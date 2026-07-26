package extensiontest

import (
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

// CoverageTarget identifies one task-level verification target proven by a provider scenario.
type CoverageTarget string

const (
	CoverageTargetMultiInstance     CoverageTarget = "multi_instance"
	CoverageTargetRestartRecovery   CoverageTarget = "restart_recovery"
	CoverageTargetDMPolicy          CoverageTarget = "dm_policy"
	CoverageTargetAuthDegradation   CoverageTarget = "auth_degradation"
	CoverageTargetRateLimitRecovery CoverageTarget = "rate_limit_recovery"
)

func (t CoverageTarget) normalize() CoverageTarget {
	return CoverageTarget(strings.ToLower(strings.TrimSpace(string(t))))
}

// ManagedInstanceOutcome summarizes the final state observed for one managed instance.
type ManagedInstanceOutcome struct {
	InstanceID        string
	FinalStatus       bridgepkg.BridgeStatus
	DegradationReason bridgepkg.BridgeDegradationReason
}

// ProviderConformanceSummary is the reusable matrix row future providers can extend.
type ProviderConformanceSummary struct {
	Provider         string
	Platform         string
	Targets          []CoverageTarget
	ManagedInstances []ManagedInstanceOutcome
}

// OutcomeClass identifies the classified recovery bucket validated by a scenario.
type OutcomeClass string

const (
	OutcomeClassAuthFailure OutcomeClass = "auth_failure"
	OutcomeClassRateLimit   OutcomeClass = "rate_limit"
)

func (c OutcomeClass) normalize() OutcomeClass {
	return OutcomeClass(strings.ToLower(strings.TrimSpace(string(c))))
}

// ClassifiedOutcome captures the observed structured state transition for a recovery class.
type ClassifiedOutcome struct {
	Provider       string
	Classification OutcomeClass
	Status         bridgepkg.BridgeStatus
	Reason         bridgepkg.BridgeDegradationReason
	Retryable      bool
}

// ClassifiedOutcomeExpectation describes the expected structured result for a recovery class.
type ClassifiedOutcomeExpectation struct {
	Classification OutcomeClass
	Status         bridgepkg.BridgeStatus
	Reason         bridgepkg.BridgeDegradationReason
	Retryable      bool
}

// ConformanceMatrixIssue reports one matrix-level validation failure.
type ConformanceMatrixIssue struct {
	Code    string
	Message string
}

// ConformanceMatrixError aggregates matrix validation failures.
type ConformanceMatrixError struct {
	Issues []ConformanceMatrixIssue
}

func (e *ConformanceMatrixError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
	}
	return strings.Join(parts, "; ")
}

// SummarizeConformanceReport normalizes one provider report into a reusable matrix row.
func SummarizeConformanceReport(
	provider string,
	platform string,
	report ConformanceReport,
	targets ...CoverageTarget,
) ProviderConformanceSummary {
	summary := ProviderConformanceSummary{
		Provider: strings.TrimSpace(provider),
		Platform: strings.TrimSpace(platform),
		Targets:  normalizeCoverageTargets(targets),
	}
	populateSummaryIdentity(&summary, report)
	summary.ManagedInstances = summarizeManagedInstanceOutcomes(report)
	return summary
}

type providerPlatformKey struct {
	provider string
	platform string
}

// BuildConformanceMatrix clones and canonicalizes provider summaries for reporting and validation.
func BuildConformanceMatrix(entries ...ProviderConformanceSummary) []ProviderConformanceSummary {
	merged := make(map[providerPlatformKey]ProviderConformanceSummary, len(entries))
	for _, entry := range entries {
		normalized := ProviderConformanceSummary{
			Provider: strings.TrimSpace(entry.Provider),
			Platform: strings.TrimSpace(entry.Platform),
			Targets:  normalizeCoverageTargets(entry.Targets),
		}
		normalized.ManagedInstances = normalizeManagedInstanceOutcomes(entry.ManagedInstances)

		key := providerPlatformKey{
			provider: normalized.Provider,
			platform: normalized.Platform,
		}
		if existing, ok := merged[key]; ok {
			existing.Targets = normalizeCoverageTargets(append(existing.Targets, normalized.Targets...))
			existing.ManagedInstances = mergeManagedInstanceOutcomes(
				existing.ManagedInstances,
				normalized.ManagedInstances,
			)
			merged[key] = existing
			continue
		}
		merged[key] = normalized
	}
	matrix := make([]ProviderConformanceSummary, 0, len(merged))
	for _, entry := range merged {
		matrix = append(matrix, entry)
	}
	slices.SortFunc(matrix, func(left, right ProviderConformanceSummary) int {
		if cmp := strings.Compare(left.Provider, right.Provider); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.Platform, right.Platform)
	})
	return matrix
}

// ValidateConformanceMatrix checks that the reusable provider matrix covers the required targets.
func ValidateConformanceMatrix(entries []ProviderConformanceSummary, requiredTargets ...CoverageTarget) error {
	matrix := BuildConformanceMatrix(entries...)
	issues := make([]ConformanceMatrixIssue, 0)
	required := normalizeCoverageTargets(requiredTargets)

	if len(matrix) == 0 {
		issues = append(issues, ConformanceMatrixIssue{
			Code:    "missing_matrix_entries",
			Message: "conformance matrix did not include any provider summaries",
		})
	}

	coveredTargets := make(map[CoverageTarget]int)
	for _, entry := range matrix {
		issues = append(issues, validateConformanceMatrixEntry(entry)...)
		for _, target := range entry.Targets {
			coveredTargets[target]++
		}
	}

	for _, target := range required {
		if coveredTargets[target] == 0 {
			issues = append(issues, ConformanceMatrixIssue{
				Code:    "missing_required_target",
				Message: fmt.Sprintf("conformance matrix did not cover required target %q", target),
			})
		}
	}

	if len(issues) > 0 {
		return &ConformanceMatrixError{Issues: issues}
	}
	return nil
}

func populateSummaryIdentity(summary *ProviderConformanceSummary, report ConformanceReport) {
	if summary == nil || report.Handshake == nil || report.Handshake.Request.Runtime.Bridge == nil {
		return
	}
	runtime := report.Handshake.Request.Runtime.Bridge
	if summary.Provider == "" {
		summary.Provider = strings.TrimSpace(runtime.Provider)
	}
	if summary.Platform == "" {
		summary.Platform = strings.TrimSpace(runtime.Platform)
	}
}

func summarizeManagedInstanceOutcomes(report ConformanceReport) []ManagedInstanceOutcome {
	managedByID := make(map[string]ManagedInstanceOutcome)
	seedManagedInstanceOutcomes(managedByID, report)
	mergeManagedInstanceStateRecords(managedByID, report.States)
	return managedInstanceOutcomesFromMap(managedByID)
}

func seedManagedInstanceOutcomes(managedByID map[string]ManagedInstanceOutcome, report ConformanceReport) {
	if report.Handshake != nil && report.Handshake.Request.Runtime.Bridge != nil {
		for _, managed := range report.Handshake.Request.Runtime.Bridge.ManagedInstances {
			instanceID := strings.TrimSpace(managed.Instance.ID)
			if instanceID == "" {
				continue
			}
			outcome := ManagedInstanceOutcome{
				InstanceID:  instanceID,
				FinalStatus: bridgeStatusDomain(managed.Instance.Status.Normalize()),
			}
			if managed.Instance.Degradation != nil {
				outcome.DegradationReason = bridgepkg.BridgeDegradationReason(
					managed.Instance.Degradation.Reason.Normalize(),
				)
			}
			managedByID[instanceID] = outcome
		}
	}
	if report.Ownership == nil {
		return
	}
	for _, instance := range report.Ownership.Listed {
		seedOutcomeFromInstance(managedByID, instance)
	}
	for _, instance := range report.Ownership.Fetched {
		seedOutcomeFromInstance(managedByID, instance)
	}
}

func seedOutcomeFromInstance(managedByID map[string]ManagedInstanceOutcome, instance bridgepkg.BridgeInstance) {
	instanceID := strings.TrimSpace(instance.ID)
	if instanceID == "" {
		return
	}
	if _, ok := managedByID[instanceID]; ok {
		return
	}
	outcome := ManagedInstanceOutcome{
		InstanceID:  instanceID,
		FinalStatus: instance.Status.Normalize(),
	}
	if instance.Degradation != nil {
		outcome.DegradationReason = instance.Degradation.Reason.Normalize()
	}
	managedByID[instanceID] = outcome
}

func mergeManagedInstanceStateRecords(
	managedByID map[string]ManagedInstanceOutcome,
	states []StateRecord,
) {
	for _, record := range states {
		instanceID := strings.TrimSpace(record.BridgeInstanceID)
		if instanceID == "" {
			instanceID = strings.TrimSpace(record.Instance.ID)
		}
		if instanceID == "" {
			continue
		}
		outcome := managedByID[instanceID]
		outcome.InstanceID = instanceID
		outcome.FinalStatus = record.Status.Normalize()
		outcome.DegradationReason = ""
		if record.Instance.Degradation != nil {
			outcome.DegradationReason = record.Instance.Degradation.Reason.Normalize()
		}
		managedByID[instanceID] = outcome
	}
}

func managedInstanceOutcomesFromMap(managedByID map[string]ManagedInstanceOutcome) []ManagedInstanceOutcome {
	outcomes := make([]ManagedInstanceOutcome, 0, len(managedByID))
	for _, outcome := range managedByID {
		outcomes = append(outcomes, outcome)
	}
	slices.SortFunc(outcomes, func(left, right ManagedInstanceOutcome) int {
		return strings.Compare(left.InstanceID, right.InstanceID)
	})
	return outcomes
}

func validateConformanceMatrixEntry(entry ProviderConformanceSummary) []ConformanceMatrixIssue {
	issues := make([]ConformanceMatrixIssue, 0)
	if strings.TrimSpace(entry.Provider) == "" {
		issues = append(issues, ConformanceMatrixIssue{
			Code:    "missing_provider",
			Message: "conformance matrix entry omitted provider",
		})
	}
	if strings.TrimSpace(entry.Platform) == "" {
		issues = append(issues, ConformanceMatrixIssue{
			Code:    "missing_platform",
			Message: fmt.Sprintf("provider %q conformance matrix entry omitted platform", entry.Provider),
		})
	}
	if len(entry.ManagedInstances) == 0 {
		issues = append(issues, ConformanceMatrixIssue{
			Code:    "missing_managed_instances",
			Message: fmt.Sprintf("provider %q did not report any managed instances in the matrix", entry.Provider),
		})
	}
	if len(entry.Targets) == 0 {
		issues = append(issues, ConformanceMatrixIssue{
			Code:    "missing_targets",
			Message: fmt.Sprintf("provider %q did not declare any conformance targets", entry.Provider),
		})
	}
	logicalManagedInstances := logicalManagedInstanceCount(entry.ManagedInstances)
	if slices.Contains(entry.Targets, CoverageTargetMultiInstance) && logicalManagedInstances < 2 {
		issues = append(issues, ConformanceMatrixIssue{
			Code: "insufficient_multi_instance_coverage",
			Message: fmt.Sprintf(
				"provider %q marked multi-instance coverage with only %d managed instance(s)",
				entry.Provider,
				logicalManagedInstances,
			),
		})
	}
	for _, outcome := range entry.ManagedInstances {
		if strings.TrimSpace(outcome.InstanceID) == "" {
			issues = append(issues, ConformanceMatrixIssue{
				Code: "missing_instance_id",
				Message: fmt.Sprintf(
					"provider %q included a matrix row without bridge instance id",
					entry.Provider,
				),
			})
		}
		if err := outcome.FinalStatus.Validate(); err != nil {
			issues = append(issues, ConformanceMatrixIssue{
				Code: "invalid_final_status",
				Message: fmt.Sprintf(
					"provider %q instance %q reported invalid final status: %v",
					entry.Provider,
					strings.TrimSpace(outcome.InstanceID),
					err,
				),
			})
		}
		if outcome.DegradationReason != "" {
			if err := outcome.DegradationReason.Validate(); err != nil {
				issues = append(issues, ConformanceMatrixIssue{
					Code: "invalid_degradation_reason",
					Message: fmt.Sprintf(
						"provider %q instance %q reported invalid degradation reason: %v",
						entry.Provider,
						strings.TrimSpace(outcome.InstanceID),
						err,
					),
				})
			}
		}
	}
	return issues
}

// ValidateClassifiedOutcome asserts that a classified recovery outcome matches the shared expectation.
func ValidateClassifiedOutcome(actual ClassifiedOutcome, expect ClassifiedOutcomeExpectation) error {
	issues := make([]ConformanceMatrixIssue, 0)

	if got, want := actual.Classification.normalize(), expect.Classification.normalize(); got != want {
		issues = append(issues, ConformanceMatrixIssue{
			Code: "wrong_classification",
			Message: fmt.Sprintf(
				"provider %q classification = %q, want %q",
				actual.Provider,
				actual.Classification,
				expect.Classification,
			),
		})
	}
	if got, want := actual.Status.Normalize(), expect.Status.Normalize(); got != want {
		issues = append(issues, ConformanceMatrixIssue{
			Code: "wrong_status",
			Message: fmt.Sprintf(
				"provider %q status = %q, want %q for %q",
				actual.Provider,
				actual.Status,
				expect.Status,
				actual.Classification,
			),
		})
	}
	if got, want := actual.Reason.Normalize(), expect.Reason.Normalize(); got != want {
		issues = append(issues, ConformanceMatrixIssue{
			Code: "wrong_degradation_reason",
			Message: fmt.Sprintf(
				"provider %q degradation reason = %q, want %q for %q",
				actual.Provider,
				actual.Reason,
				expect.Reason,
				actual.Classification,
			),
		})
	}
	if actual.Retryable != expect.Retryable {
		issues = append(issues, ConformanceMatrixIssue{
			Code: "wrong_retryability",
			Message: fmt.Sprintf(
				"provider %q retryable = %t, want %t for %q",
				actual.Provider,
				actual.Retryable,
				expect.Retryable,
				actual.Classification,
			),
		})
	}

	if len(issues) > 0 {
		return &ConformanceMatrixError{Issues: issues}
	}
	return nil
}

func normalizeCoverageTargets(targets []CoverageTarget) []CoverageTarget {
	seen := make(map[CoverageTarget]struct{}, len(targets))
	normalized := make([]CoverageTarget, 0, len(targets))
	for _, target := range targets {
		value := target.normalize()
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	slices.SortFunc(normalized, func(left, right CoverageTarget) int {
		return strings.Compare(string(left), string(right))
	})
	return normalized
}

func mergeManagedInstanceOutcomes(
	existing []ManagedInstanceOutcome,
	incoming []ManagedInstanceOutcome,
) []ManagedInstanceOutcome {
	merged := make([]ManagedInstanceOutcome, 0, len(existing)+len(incoming))
	merged = append(merged, existing...)
	merged = append(merged, incoming...)
	return normalizeManagedInstanceOutcomes(merged)
}

func normalizeManagedInstanceOutcomes(entries []ManagedInstanceOutcome) []ManagedInstanceOutcome {
	merged := make(map[string]ManagedInstanceOutcome, len(entries))
	invalid := make([]ManagedInstanceOutcome, 0)
	for _, outcome := range entries {
		instanceID := strings.TrimSpace(outcome.InstanceID)
		outcome.InstanceID = instanceID
		outcome.FinalStatus = outcome.FinalStatus.Normalize()
		outcome.DegradationReason = outcome.DegradationReason.Normalize()
		if instanceID == "" {
			invalid = append(invalid, outcome)
			continue
		}
		merged[instanceID] = outcome
	}

	result := make([]ManagedInstanceOutcome, 0, len(invalid)+len(merged))
	result = append(result, invalid...)
	for _, outcome := range merged {
		result = append(result, outcome)
	}
	slices.SortFunc(result, func(left, right ManagedInstanceOutcome) int {
		return strings.Compare(left.InstanceID, right.InstanceID)
	})
	return result
}

func logicalManagedInstanceCount(entries []ManagedInstanceOutcome) int {
	seen := make(map[string]struct{}, len(entries))
	for _, outcome := range entries {
		instanceID := strings.TrimSpace(outcome.InstanceID)
		if instanceID == "" {
			continue
		}
		seen[instanceID] = struct{}{}
	}
	return len(seen)
}
