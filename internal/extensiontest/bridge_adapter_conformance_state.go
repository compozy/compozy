package extensiontest

import (
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func validateOwnershipConformance(
	ownership *OwnershipRecord,
	expect ConformanceExpectation,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if expect.RequireOwnedInstanceList || expect.RequireOwnedInstanceFetch {
		if ownership == nil {
			return []ConformanceIssue{{
				Code:    "missing_ownership_marker",
				Message: "adapter did not write provider ownership markers",
			}}
		}
		if strings.TrimSpace(ownership.Error) != "" {
			issues = append(issues, ConformanceIssue{
				Code:    "owned_instance_lookup_error",
				Message: ownership.Error,
			})
		}
	}
	if expect.RequireOwnedInstanceList && ownership != nil {
		issues = append(issues, validateOwnedInstanceList(ownership.Listed, expectedByID)...)
	}
	if expect.RequireOwnedInstanceFetch && ownership != nil {
		issues = append(issues, validateOwnedInstanceFetch(ownership.Fetched, expectedByID)...)
	}
	return issues
}

func validateOwnedInstanceList(
	listedInstances []bridgepkg.BridgeInstance,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if len(listedInstances) == 0 {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_owned_instance_list",
			Message: "adapter did not list its owned bridge instances",
		})
	}

	listed := make(map[string]struct{}, len(listedInstances))
	for _, instance := range listedInstances {
		instanceID := strings.TrimSpace(instance.ID)
		listed[instanceID] = struct{}{}
		if len(expectedByID) > 0 {
			if _, ok := expectedByID[instanceID]; !ok {
				issues = append(issues, ConformanceIssue{
					Code:    "unexpected_owned_instance",
					Message: fmt.Sprintf("ownership list included unexpected instance %q", instanceID),
				})
			}
		}
	}
	for instanceID := range expectedByID {
		if _, ok := listed[instanceID]; !ok {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_owned_instance",
				Message: fmt.Sprintf("ownership list omitted expected instance %q", instanceID),
			})
		}
	}
	return issues
}

func validateOwnedInstanceFetch(
	fetchedInstances []bridgepkg.BridgeInstance,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if len(fetchedInstances) == 0 {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_owned_instance_fetch",
			Message: "adapter did not fetch owned bridge instances explicitly",
		})
	}

	fetched := make(map[string]struct{}, len(fetchedInstances))
	for _, instance := range fetchedInstances {
		fetched[strings.TrimSpace(instance.ID)] = struct{}{}
	}
	for instanceID := range expectedByID {
		if _, ok := fetched[instanceID]; !ok {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_owned_instance_fetch",
				Message: fmt.Sprintf("adapter did not fetch owned instance %q explicitly", instanceID),
			})
		}
	}
	return issues
}

func validateStateConformance(
	states []StateRecord,
	expect ConformanceExpectation,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if expect.RequireStateReport && len(states) == 0 {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_state_report",
			Message: "adapter did not report bridge state",
		})
	}

	lastStateByID := make(map[string]StateRecord)
	for _, record := range states {
		instanceID := strings.TrimSpace(record.BridgeInstanceID)
		if instanceID == "" {
			instanceID = strings.TrimSpace(record.Instance.ID)
		}
		if instanceID == "" {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_state_instance_id",
				Message: "adapter reported bridge state without bridge_instance_id",
			})
			continue
		}
		if len(expectedByID) > 0 {
			if _, ok := expectedByID[instanceID]; !ok {
				issues = append(issues, ConformanceIssue{
					Code:    "unexpected_state_instance",
					Message: fmt.Sprintf("state report targeted unexpected instance %q", instanceID),
				})
			}
		}
		record.BridgeInstanceID = instanceID
		lastStateByID[instanceID] = record
	}

	for _, managedExpect := range expect.ManagedInstances {
		if !expect.RequireStateReport && managedExpect.ExpectedFinalStatus == "" {
			continue
		}
		last, ok := lastStateByID[strings.TrimSpace(managedExpect.InstanceID)]
		if !ok {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_managed_state",
				Message: fmt.Sprintf("adapter did not report state for managed instance %q", managedExpect.InstanceID),
			})
			continue
		}
		if managedExpect.ExpectedFinalStatus != "" &&
			last.Status.Normalize() != managedExpect.ExpectedFinalStatus.Normalize() {
			issues = append(issues, ConformanceIssue{
				Code: "wrong_final_status",
				Message: fmt.Sprintf(
					"managed instance %q last reported status = %q, want %q",
					managedExpect.InstanceID,
					last.Status,
					managedExpect.ExpectedFinalStatus,
				),
			})
		}
	}
	return issues
}
