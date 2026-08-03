package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
)

type extensionAutomationPreviewer interface {
	EffectivePackageAutomation(
		context.Context,
		[]automationpkg.Job,
		[]automationpkg.Trigger,
	) ([]automationpkg.Job, []automationpkg.Trigger, error)
}

func (s *daemonExtensionService) Inventory(
	ctx context.Context,
	name string,
) (contract.ExtensionInventoryPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	ext, desired, err := s.desiredExtensionKit(ctx, name)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	live, err := s.extensionOwnedResourceRecords(ctx, name)
	if err != nil {
		return contract.ExtensionInventoryPayload{}, err
	}
	if !ext.Info.Enabled {
		live = nil
	}
	items := s.mergeExtensionKitInventory(ctx, desired, live)
	return contract.ExtensionInventoryPayload{
		Extension: ext.Info.Name,
		Enabled:   ext.Info.Enabled,
		Items:     contractExtensionKitItems(items),
	}, nil
}

func (s *daemonExtensionService) Preview(
	ctx context.Context,
	name string,
) (contract.ExtensionEnablePreviewPayload, error) {
	if err := s.checkReady(); err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	return s.previewExtension(ctx, name)
}

func (s *daemonExtensionService) previewExtension(
	ctx context.Context,
	name string,
) (contract.ExtensionEnablePreviewPayload, error) {
	ext, desired, err := s.desiredExtensionKit(ctx, name)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	live, err := s.extensionOwnedResourceRecords(ctx, name)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	status, err := s.Status(ctx, name)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	networkDigest, networkConfirmationRequired, err := candidateNetworkConfirmationRequirement(ext.Info, ext.Manifest)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	conflicts, err := s.extensionAgentConflicts(ctx, ext)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	changes, err := s.extensionKitChanges(ctx, desired, live)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	automationStarting, err := s.previewExtensionAutomation(ctx, ext, changes)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	return contract.ExtensionEnablePreviewPayload{
		Extension:                   ext.Info.Name,
		Changes:                     changes,
		AgentConflicts:              conflicts,
		MissingEnv:                  slices.Clone(status.MissingEnv),
		AutomationStarting:          automationStarting,
		NetworkRequirementDigest:    networkDigest,
		NetworkConfirmationRequired: networkConfirmationRequired,
	}, nil
}

func (s *daemonExtensionService) extensionKitChanges(
	ctx context.Context,
	desired []extensionpkg.KitItem,
	live []resources.RawRecord,
) ([]contract.ExtensionKitChangePayload, error) {
	desiredByKey := make(map[extensionKitItemKey]extensionpkg.KitItem, len(desired))
	for _, item := range desired {
		desiredByKey[newExtensionKitItemKey(item.Kind, item.Name)] = item
	}
	liveByKey := make(map[extensionKitItemKey]resources.RawRecord, len(live))
	for _, record := range live {
		name := s.extensionResourceName(ctx, record)
		liveByKey[newExtensionKitItemKey(record.Kind, name)] = record
	}
	changes := make([]contract.ExtensionKitChangePayload, 0, len(desiredByKey)+len(liveByKey))
	for key, item := range desiredByKey {
		record, exists := liveByKey[key]
		if !exists {
			changes = append(changes, contract.ExtensionKitChangePayload{
				Kind: item.Kind, ID: item.ID, Name: item.Name, Change: contract.ExtensionKitChangeAdded,
			})
			continue
		}
		delete(liveByKey, key)
		unchanged, err := s.extensionKitSpecMatches(ctx, item, record)
		if err != nil {
			return nil, err
		}
		if unchanged {
			continue
		}
		changes = append(changes, contract.ExtensionKitChangePayload{
			Kind: item.Kind, ID: item.ID, Name: item.Name, Change: contract.ExtensionKitChangeChanged,
		})
	}
	for _, record := range liveByKey {
		changes = append(changes, contract.ExtensionKitChangePayload{
			Kind: record.Kind, ID: record.ID, Name: s.extensionResourceName(ctx, record),
			Change: contract.ExtensionKitChangeRemoved,
		})
	}
	slices.SortFunc(changes, func(left, right contract.ExtensionKitChangePayload) int {
		if byKind := strings.Compare(string(left.Kind), string(right.Kind)); byKind != 0 {
			return byKind
		}
		if byName := strings.Compare(left.Name, right.Name); byName != 0 {
			return byName
		}
		return strings.Compare(string(left.Change), string(right.Change))
	})
	return changes, nil
}

func (s *daemonExtensionService) extensionKitSpecMatches(
	ctx context.Context,
	desired extensionpkg.KitItem,
	live resources.RawRecord,
) (bool, error) {
	desiredSpec, desiredValidated, err := resources.ValidateAndCanonicalizeIfRegistered(
		ctx,
		s.resourceCodecs,
		desired.Kind,
		live.Scope,
		desired.SpecJSON,
	)
	if err != nil {
		return false, fmt.Errorf("daemon: canonicalize desired extension resource %q: %w", desired.ID, err)
	}
	liveSpec, liveValidated, err := resources.ValidateAndCanonicalizeIfRegistered(
		ctx,
		s.resourceCodecs,
		live.Kind,
		live.Scope,
		live.SpecJSON,
	)
	if err != nil {
		return false, fmt.Errorf("daemon: canonicalize live extension resource %q: %w", live.ID, err)
	}
	if !desiredValidated || !liveValidated {
		return false, fmt.Errorf("daemon: extension resource codec for kind %q is required", desired.Kind)
	}
	return bytes.Equal(desiredSpec, liveSpec), nil
}

func (s *daemonExtensionService) desiredExtensionKit(
	ctx context.Context,
	name string,
) (*extensionpkg.Extension, []extensionpkg.KitItem, error) {
	ext, err := s.runtime.InspectPackageResources(ctx, strings.TrimSpace(name))
	if err != nil {
		return nil, nil, err
	}
	items, err := projectExtensionKitItems(ctx, ext, s.resourceCodecs, s.getenv)
	if err != nil {
		return nil, nil, err
	}
	return ext, items, nil
}

func (s *daemonExtensionService) extensionOwnedResourceRecords(
	ctx context.Context,
	name string,
) ([]resources.RawRecord, error) {
	if s.resourceStore == nil {
		return nil, nil
	}
	owner := extensionOwner(name)
	records, err := s.resourceStore.ListRaw(ctx, s.resourceActor, resources.ResourceFilter{Owner: owner})
	if err != nil {
		return nil, fmt.Errorf("daemon: list extension-owned resources: %w", err)
	}
	return records, nil
}

func (s *daemonExtensionService) mergeExtensionKitInventory(
	ctx context.Context,
	desired []extensionpkg.KitItem,
	live []resources.RawRecord,
) []extensionpkg.KitItem {
	byKey := make(map[extensionKitItemKey]extensionpkg.KitItem, len(desired)+len(live))
	ids := make(map[string]extensionKitItemKey, len(desired))
	for _, item := range desired {
		key := newExtensionKitItemKey(item.Kind, item.Name)
		byKey[key] = item
		ids[item.ID] = key
	}
	for _, record := range live {
		key, matchedID := ids[record.ID]
		name := ""
		if !matchedID {
			name = s.extensionResourceName(ctx, record)
			key = newExtensionKitItemKey(record.Kind, name)
		}
		item := byKey[key]
		if item.Name == "" {
			item = extensionpkg.KitItem{Kind: record.Kind, Name: name}
		}
		item.ID = record.ID
		item.Live = true
		byKey[key] = item
	}
	result := make([]extensionpkg.KitItem, 0, len(byKey))
	for _, item := range byKey {
		result = append(result, item)
	}
	sortExtensionKitItems(result)
	return result
}

type extensionKitItemKey struct {
	kind resources.ResourceKind
	name string
}

func newExtensionKitItemKey(kind resources.ResourceKind, name string) extensionKitItemKey {
	return extensionKitItemKey{kind: kind, name: strings.TrimSpace(name)}
}

func sortExtensionKitItems(items []extensionpkg.KitItem) {
	slices.SortFunc(items, func(left, right extensionpkg.KitItem) int {
		if byKind := strings.Compare(string(left.Kind), string(right.Kind)); byKind != 0 {
			return byKind
		}
		return strings.Compare(left.Name, right.Name)
	})
}

func contractExtensionKitItems(items []extensionpkg.KitItem) []contract.ExtensionKitItemPayload {
	result := make([]contract.ExtensionKitItemPayload, 0, len(items))
	for _, item := range items {
		result = append(result, contract.ExtensionKitItemPayload{
			Kind: item.Kind, ID: item.ID, Name: item.Name, Live: item.Live,
		})
	}
	return result
}

func (s *daemonExtensionService) extensionAgentConflicts(
	ctx context.Context,
	ext *extensionpkg.Extension,
) ([]string, error) {
	shipped := make(map[string]struct{}, len(ext.StaticAgents))
	for _, agent := range ext.StaticAgents {
		shipped[strings.TrimSpace(agent.Agent.Name)] = struct{}{}
	}
	conflicts := make(map[string]struct{})
	for _, name := range compozyconfig.BuiltinAgentNames() {
		if _, ok := shipped[name]; ok {
			conflicts[name] = struct{}{}
		}
	}
	if s.resourceStore != nil {
		records, err := s.resourceStore.ListRaw(ctx, s.resourceActor, resources.ResourceFilter{
			Kind: compozyconfig.AgentResourceKind,
		})
		if err != nil {
			return nil, fmt.Errorf("daemon: list visible agents for extension preview: %w", err)
		}
		owner := extensionOwner(ext.Info.Name).Normalize()
		for _, record := range records {
			if record.Owner.Normalize() == owner {
				continue
			}
			name := s.extensionResourceName(ctx, record)
			if _, ok := shipped[name]; ok {
				conflicts[name] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(conflicts))
	for name := range conflicts {
		result = append(result, name)
	}
	slices.Sort(result)
	return result, nil
}

func (s *daemonExtensionService) previewExtensionAutomation(
	ctx context.Context,
	ext *extensionpkg.Extension,
	changes []contract.ExtensionKitChangePayload,
) ([]string, error) {
	if s.automation == nil {
		return nil, errors.New("daemon: extension automation previewer is required")
	}
	jobs := slices.Clone(ext.AutomationJobs)
	triggers := slices.Clone(ext.AutomationTriggers)
	jobs, triggers, err := s.automation.EffectivePackageAutomation(ctx, jobs, triggers)
	if err != nil {
		return nil, fmt.Errorf("daemon: apply automation overlays for extension preview: %w", err)
	}
	changedAutomation := make(map[extensionKitItemKey]struct{}, len(changes))
	for _, change := range changes {
		if change.Change == contract.ExtensionKitChangeRemoved {
			continue
		}
		switch change.Kind {
		case automationpkg.JobResourceKind, automationpkg.TriggerResourceKind:
			changedAutomation[newExtensionKitItemKey(change.Kind, change.Name)] = struct{}{}
		}
	}
	starting := make([]string, 0, len(jobs)+len(triggers))
	for _, job := range jobs {
		_, changed := changedAutomation[newExtensionKitItemKey(automationpkg.JobResourceKind, job.Name)]
		if job.Enabled && (!ext.Info.Enabled || changed) {
			starting = append(starting, job.Name)
		}
	}
	for _, trigger := range triggers {
		_, changed := changedAutomation[newExtensionKitItemKey(automationpkg.TriggerResourceKind, trigger.Name)]
		if trigger.Enabled && (!ext.Info.Enabled || changed) {
			starting = append(starting, trigger.Name)
		}
	}
	slices.Sort(starting)
	return starting, nil
}
