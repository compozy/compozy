package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/heartbeat"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

type extensionPackageInspector interface {
	InspectPackageResources(context.Context, string) (*extensionpkg.Extension, error)
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
	items := mergeExtensionKitInventory(desired, live)
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
	conflicts, err := s.extensionAgentConflicts(ctx, ext)
	if err != nil {
		return contract.ExtensionEnablePreviewPayload{}, err
	}
	automationStarting := []string(nil)
	if !ext.Info.Enabled {
		automationStarting, err = s.previewExtensionAutomation(ctx, ext)
		if err != nil {
			return contract.ExtensionEnablePreviewPayload{}, err
		}
	}
	return contract.ExtensionEnablePreviewPayload{
		Extension:                   ext.Info.Name,
		WouldPublish:                contractExtensionKitItems(mergeExtensionKitInventory(desired, live)),
		AgentConflicts:              conflicts,
		MissingEnv:                  slices.Clone(status.MissingEnv),
		AutomationStarting:          automationStarting,
		NetworkRequirementDigest:    status.NetworkRequirementDigest,
		NetworkConfirmationRequired: status.NetworkConfirmationRequired,
	}, nil
}

func (s *daemonExtensionService) desiredExtensionKit(
	ctx context.Context,
	name string,
) (*extensionpkg.Extension, []extensionpkg.KitItem, error) {
	inspector, ok := s.runtime.(extensionPackageInspector)
	var ext *extensionpkg.Extension
	var err error
	if ok && inspector != nil {
		ext, err = inspector.InspectPackageResources(ctx, strings.TrimSpace(name))
	} else {
		ext, err = loadExtensionSnapshot(s.registry, s.runtime, s.logger, name)
	}
	if err != nil {
		return nil, nil, err
	}
	items, err := extensionKitItems(ext)
	if err != nil {
		return nil, nil, err
	}
	return ext, items, nil
}

func extensionKitItems(ext *extensionpkg.Extension) ([]extensionpkg.KitItem, error) {
	if ext == nil || ext.Manifest == nil {
		return nil, errors.New("daemon: inspected extension manifest is required")
	}
	collector := newExtensionKitItemCollector(ext.Info.Name)
	collector.appendAgents(ext)
	collector.appendSkills(ext)
	if err := collector.appendTools(ext); err != nil {
		return nil, err
	}
	collector.appendRemainingResources(ext)
	return collector.result(), nil
}

type extensionKitItemCollector struct {
	extensionName string
	items         map[string]extensionpkg.KitItem
}

func newExtensionKitItemCollector(extensionName string) *extensionKitItemCollector {
	return &extensionKitItemCollector{
		extensionName: strings.TrimSpace(extensionName),
		items:         make(map[string]extensionpkg.KitItem),
	}
}

func (c *extensionKitItemCollector) append(kind resources.ResourceKind, id, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	key := extensionKitItemKey(kind, name)
	candidate := extensionpkg.KitItem{Kind: kind, ID: strings.TrimSpace(id), Name: name}
	if current, ok := c.items[key]; !ok || candidate.ID < current.ID {
		c.items[key] = candidate
	}
}

func (c *extensionKitItemCollector) appendAgents(ext *extensionpkg.Extension) {
	for _, staticAgent := range ext.StaticAgents {
		name := strings.TrimSpace(staticAgent.Agent.Name)
		base := "extension/" + c.extensionName + "/agent/" + name
		c.append(compozyconfig.AgentResourceKind, base, name)
		if staticAgent.Soul != nil {
			c.append(soul.ResourceKind, base+"/soul", name)
		}
		if staticAgent.Heartbeat != nil {
			c.append(heartbeat.ResourceKind, base+"/heartbeat", name)
		}
		for _, server := range staticAgent.Agent.MCPServers {
			serverName := strings.TrimSpace(server.Name)
			c.append(compozyconfig.MCPServerResourceKind, base+"/mcp_server/"+serverName, serverName)
		}
	}
}

func (c *extensionKitItemCollector) appendSkills(ext *extensionpkg.Extension) {
	for _, skill := range ext.Skills {
		if skill == nil {
			continue
		}
		name := strings.TrimSpace(skill.Meta.Name)
		base := "extension/" + c.extensionName + "/skills/skill/" + name
		c.append(skillspkg.SkillResourceKind, base, name)
		for _, server := range skill.MCPServers {
			serverName := strings.TrimSpace(server.Name)
			c.append(compozyconfig.MCPServerResourceKind, base+"/mcp_server/"+serverName, serverName)
		}
	}
}

func (c *extensionKitItemCollector) appendTools(ext *extensionpkg.Extension) error {
	tools, err := extensionpkg.ResolveManifestToolResources(ext.Manifest)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension kit tools: %w", err)
	}
	for _, tool := range tools {
		name := strings.TrimSpace(tool.ID.String())
		c.append(toolspkg.ToolResourceKind, "extension/"+c.extensionName+"/tool/"+name, name)
	}
	return nil
}

func (c *extensionKitItemCollector) appendRemainingResources(ext *extensionpkg.Extension) {
	for _, spec := range ext.Loops {
		name := strings.TrimSpace(spec.Name)
		c.append(looppkg.ResourceKind, "extension/"+c.extensionName+"/loops/"+name, name)
	}
	for name := range ext.Manifest.Resources.MCPServers {
		c.append(compozyconfig.MCPServerResourceKind, "extension/"+c.extensionName+"/mcp_server/"+name, name)
	}
	for _, hook := range ext.Hooks {
		name := strings.TrimSpace(hook.Name)
		c.append(hookBindingResourceKind, "extension/"+c.extensionName+"/hook.binding/"+name, name)
	}
	for _, job := range ext.AutomationJobs {
		c.append(automationpkg.JobResourceKind, job.ID, job.Name)
	}
	for _, trigger := range ext.AutomationTriggers {
		c.append(automationpkg.TriggerResourceKind, trigger.ID, trigger.Name)
	}
	for _, layout := range ext.Layouts {
		name := strings.TrimSpace(layout.ID)
		c.append(windowmanager.WindowLayoutResourceKind, "extension/"+c.extensionName+"/window_layout/"+name, name)
	}
}

func (c *extensionKitItemCollector) result() []extensionpkg.KitItem {
	result := make([]extensionpkg.KitItem, 0, len(c.items))
	for _, item := range c.items {
		result = append(result, item)
	}
	sortExtensionKitItems(result)
	return result
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

func mergeExtensionKitInventory(
	desired []extensionpkg.KitItem,
	live []resources.RawRecord,
) []extensionpkg.KitItem {
	byKey := make(map[string]extensionpkg.KitItem, len(desired)+len(live))
	ids := make(map[string]string, len(desired))
	for _, item := range desired {
		key := extensionKitItemKey(item.Kind, item.Name)
		byKey[key] = item
		ids[item.ID] = key
	}
	for _, record := range live {
		key := ids[record.ID]
		name := ""
		if key == "" {
			name = rawExtensionResourceName(record)
			key = extensionKitItemKey(record.Kind, name)
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

func rawExtensionResourceName(record resources.RawRecord) string {
	var payload map[string]any
	if err := json.Unmarshal(record.SpecJSON, &payload); err == nil {
		for _, key := range []string{bootNameKey, "agent_name", "id"} {
			if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	parts := strings.Split(strings.TrimSpace(record.ID), "/")
	return strings.TrimSpace(parts[len(parts)-1])
}

func extensionKitItemKey(kind resources.ResourceKind, name string) string {
	return string(kind) + "\x00" + strings.TrimSpace(name)
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
			name := rawExtensionResourceName(record)
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
) ([]string, error) {
	jobs := slices.Clone(ext.AutomationJobs)
	triggers := slices.Clone(ext.AutomationTriggers)
	if previewer, ok := s.automation.(extensionAutomationPreviewer); ok && previewer != nil {
		var err error
		jobs, triggers, err = previewer.EffectivePackageAutomation(ctx, jobs, triggers)
		if err != nil {
			return nil, fmt.Errorf("daemon: apply automation overlays for extension preview: %w", err)
		}
	} else if s.automation != nil {
		currentJobs, err := s.automation.Jobs(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list automation jobs for extension preview: %w", err)
		}
		currentTriggers, err := s.automation.Triggers(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list automation triggers for extension preview: %w", err)
		}
		overrides := make(map[string]bool, len(currentJobs)+len(currentTriggers))
		for _, job := range currentJobs {
			overrides[job.ID] = job.Enabled
		}
		for _, trigger := range currentTriggers {
			overrides[trigger.ID] = trigger.Enabled
		}
		for index := range jobs {
			if enabled, exists := overrides[jobs[index].ID]; exists {
				jobs[index].Enabled = enabled
			}
		}
		for index := range triggers {
			if enabled, exists := overrides[triggers[index].ID]; exists {
				triggers[index].Enabled = enabled
			}
		}
	}
	starting := make([]string, 0, len(jobs)+len(triggers))
	for _, job := range jobs {
		if job.Enabled {
			starting = append(starting, job.Name)
		}
	}
	for _, trigger := range triggers {
		if trigger.Enabled {
			starting = append(starting, trigger.Name)
		}
	}
	slices.Sort(starting)
	return starting, nil
}
