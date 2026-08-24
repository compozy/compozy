package compozysdk

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

func normalizeDescribeResourcePaths(resources []contracts.DescribeResourcePath) []contracts.DescribeResourcePath {
	normalized := make([]contracts.DescribeResourcePath, 0, len(resources))
	for _, resource := range resources {
		normalized = append(normalized, contracts.DescribeResourcePath{
			Path: strings.TrimSpace(resource.Path), Profile: strings.TrimSpace(resource.Profile),
		})
	}
	slices.SortFunc(normalized, func(left, right contracts.DescribeResourcePath) int {
		if compared := strings.Compare(left.Path, right.Path); compared != 0 {
			return compared
		}
		return strings.Compare(left.Profile, right.Profile)
	})
	return slices.Compact(normalized)
}

func normalizeDescribeHookEvents(events []contracts.DescribeHookEvent) []contracts.DescribeHookEvent {
	normalized := make([]contracts.DescribeHookEvent, 0, len(events))
	for _, event := range events {
		normalized = append(normalized, contracts.DescribeHookEvent{
			Event:   contracts.HookEvent(strings.TrimSpace(string(event.Event))),
			Profile: strings.TrimSpace(event.Profile),
		})
	}
	slices.SortFunc(normalized, func(left, right contracts.DescribeHookEvent) int {
		if compared := strings.Compare(string(left.Event), string(right.Event)); compared != 0 {
			return compared
		}
		return strings.Compare(left.Profile, right.Profile)
	})
	return slices.Compact(normalized)
}

func describedHookEventNames(events []contracts.DescribeHookEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range normalizeDescribeHookEvents(events) {
		names = append(names, string(event.Event))
	}
	return slices.Compact(names)
}

func normalizeDescribeProfiles(profiles []contracts.DescribeProfile) []contracts.DescribeProfile {
	normalized := make([]contracts.DescribeProfile, 0, len(profiles))
	for _, profile := range profiles {
		credentials := make([]contracts.DescribeProfileCredential, 0, len(profile.Credentials))
		for _, credential := range profile.Credentials {
			credentials = append(credentials, contracts.DescribeProfileCredential{
				Provider: strings.TrimSpace(credential.Provider), Slot: strings.TrimSpace(credential.Slot),
			})
		}
		slices.SortFunc(credentials, func(left, right contracts.DescribeProfileCredential) int {
			if compared := strings.Compare(left.Provider, right.Provider); compared != 0 {
				return compared
			}
			return strings.Compare(left.Slot, right.Slot)
		})
		normalized = append(normalized, contracts.DescribeProfile{
			Name: strings.TrimSpace(profile.Name), Color: strings.TrimSpace(profile.Color),
			Icon: strings.TrimSpace(profile.Icon), Emoji: strings.TrimSpace(profile.Emoji),
			Defaults: contracts.DescribeProfileDefaults{
				Agent: strings.TrimSpace(profile.Defaults.Agent), Provider: strings.TrimSpace(profile.Defaults.Provider),
				Sandbox: strings.TrimSpace(profile.Defaults.Sandbox),
			},
			Credentials: slices.Compact(credentials),
		})
	}
	slices.SortFunc(normalized, func(left, right contracts.DescribeProfile) int {
		return strings.Compare(left.Name, right.Name)
	})
	return normalized
}
