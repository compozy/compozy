package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func projectExtensionKitItems(
	ctx context.Context,
	ext *extensionpkg.Extension,
	codecs *resources.CodecRegistry,
	getenv func(string) string,
) ([]extensionpkg.KitItem, error) {
	if ext == nil || ext.Manifest == nil {
		return nil, errors.New("daemon: inspected extension manifest is required")
	}
	if !extensionHasKitItems(ext) {
		return nil, nil
	}
	if codecs == nil {
		return nil, errors.New("daemon: extension resource codecs are required")
	}

	collector := newExtensionKitItemCollector()
	if err := collector.appendAgentSkillItems(ctx, codecs, ext); err != nil {
		return nil, err
	}
	if err := collector.appendToolMCPItems(ctx, codecs, getenv, ext); err != nil {
		return nil, err
	}
	if err := collector.appendLoopItems(ctx, codecs, ext); err != nil {
		return nil, err
	}
	if err := collector.appendHookItems(ctx, codecs, ext); err != nil {
		return nil, err
	}
	if err := collector.appendAutomationItems(ctx, codecs, ext); err != nil {
		return nil, err
	}
	if err := collector.appendLayoutItems(ctx, codecs, ext); err != nil {
		return nil, err
	}
	return collector.result(), nil
}

func extensionHasKitItems(ext *extensionpkg.Extension) bool {
	if ext == nil || ext.Manifest == nil {
		return false
	}
	return len(ext.StaticAgents) > 0 || len(ext.Skills) > 0 || len(ext.Loops) > 0 || len(ext.Hooks) > 0 ||
		len(ext.AutomationJobs) > 0 || len(ext.AutomationTriggers) > 0 || len(ext.Layouts) > 0 ||
		len(ext.Manifest.Resources.Tools) > 0 || len(ext.Manifest.Resources.MCPServers) > 0
}

type extensionKitItemCollector struct {
	items map[extensionKitItemKey]extensionpkg.KitItem
}

func newExtensionKitItemCollector() *extensionKitItemCollector {
	return &extensionKitItemCollector{items: make(map[extensionKitItemKey]extensionpkg.KitItem)}
}

func (c *extensionKitItemCollector) append(kind resources.ResourceKind, id, name string, spec []byte) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	candidate := extensionpkg.KitItem{
		Kind: kind, ID: strings.TrimSpace(id), Name: name, SpecJSON: append([]byte(nil), spec...),
	}
	key := newExtensionKitItemKey(kind, name)
	if current, ok := c.items[key]; !ok || candidate.ID < current.ID {
		c.items[key] = candidate
	}
}

func (c *extensionKitItemCollector) appendAgentSkillItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	ext *extensionpkg.Extension,
) error {
	if len(ext.StaticAgents) == 0 && len(ext.Skills) == 0 {
		return nil
	}
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	declarations := agentSkillDeclarations{}
	appendExtensionAgentResources(&declarations, globalScope, ext.Info.Name, ext.StaticAgents)
	appendSkillResources(&declarations, globalScope, skillPublicationSource{
		prefix: "extension/" + strings.TrimSpace(ext.Info.Name) + "/skills",
		owner:  extensionOwner(ext.Info.Name),
	}, ext.Skills)

	agentCodec, err := resources.ResolveCodec[compozyconfig.AgentDef](codecs, compozyconfig.AgentResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview agent codec: %w", err)
	}
	skillCodec, err := resources.ResolveCodec[skillspkg.SkillResourceSpec](codecs, skillspkg.SkillResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview skill codec: %w", err)
	}
	mcpCodec, err := resources.ResolveCodec[compozyconfig.MCPServer](codecs, compozyconfig.MCPServerResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview mcp codec: %w", err)
	}
	soulCodec, err := resources.ResolveCodec[soul.ResourceSpec](codecs, soul.ResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview soul codec: %w", err)
	}
	heartbeatCodec, err := resources.ResolveCodec[heartbeat.ResourceSpec](codecs, heartbeat.ResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview heartbeat codec: %w", err)
	}

	syncer := &agentSkillSourceSyncer{
		agentCodec: agentCodec, skillCodec: skillCodec, mcpCodec: mcpCodec,
		soulCodec: soulCodec, heartbeatCodec: heartbeatCodec,
	}
	desired := newIndexedAgentSkillResources()
	if err := syncer.appendDesiredResources(ctx, &desired, make(map[string]string), declarations); err != nil {
		return fmt.Errorf("daemon: project extension preview agent and skill resources: %w", err)
	}
	for _, item := range desired.agents {
		c.append(compozyconfig.AgentResourceKind, item.id, item.spec.Name, item.encoded)
	}
	for _, item := range desired.skills {
		c.append(skillspkg.SkillResourceKind, item.id, item.spec.Name, item.encoded)
	}
	for _, item := range desired.mcpServers {
		c.append(compozyconfig.MCPServerResourceKind, item.id, item.spec.Name, item.encoded)
	}
	for _, item := range desired.souls {
		c.append(soul.ResourceKind, item.id, item.spec.AgentName, item.encoded)
	}
	for _, item := range desired.heartbeats {
		c.append(heartbeat.ResourceKind, item.id, item.spec.AgentName, item.encoded)
	}
	return nil
}

func (c *extensionKitItemCollector) appendToolMCPItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	getenv func(string) string,
	ext *extensionpkg.Extension,
) error {
	if len(ext.Manifest.Resources.Tools) == 0 && len(ext.Manifest.Resources.MCPServers) == 0 {
		return nil
	}
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	declarations := toolMCPDesiredResources{}
	tools, err := extensionpkg.ResolveManifestToolResources(ext.Manifest)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview tools: %w", err)
	}
	for _, tool := range tools {
		declarations.tools = append(declarations.tools, toolPublicationInput{
			sourceKey: "extension/" + ext.Info.Name + "/tool/" + strings.TrimSpace(tool.ID.String()),
			scope:     globalScope, owner: extensionOwner(ext.Info.Name), spec: cloneToolSpec(tool),
		})
	}
	if err := appendExtensionMCPServerDeclarations(&declarations, ext, getenv, globalScope); err != nil {
		return err
	}

	toolCodec, err := resources.ResolveCodec[toolspkg.Tool](codecs, toolspkg.ToolResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview tool codec: %w", err)
	}
	mcpCodec, err := resources.ResolveCodec[compozyconfig.MCPServer](codecs, compozyconfig.MCPServerResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview mcp codec: %w", err)
	}
	syncer := &toolMCPSourceSyncer{
		toolCodec: toolCodec,
		mcpCodec:  mcpCodec,
		providers: []toolMCPDeclarationProvider{func(context.Context) (toolMCPDesiredResources, error) {
			return declarations, nil
		}},
	}
	desired, err := syncer.desiredResources(ctx, nil)
	if err != nil {
		return fmt.Errorf("daemon: project extension preview tool and mcp resources: %w", err)
	}
	for _, item := range desired.tools {
		c.append(toolspkg.ToolResourceKind, item.id, item.spec.ID.String(), item.encoded)
	}
	for _, item := range desired.mcpServers {
		c.append(compozyconfig.MCPServerResourceKind, item.id, item.spec.Name, item.encoded)
	}
	return nil
}

func (c *extensionKitItemCollector) appendLoopItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	ext *extensionpkg.Extension,
) error {
	if len(ext.Loops) == 0 {
		return nil
	}
	codec, err := resources.ResolveCodec[looppkg.ResourceSpec](codecs, looppkg.ResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview loop codec: %w", err)
	}
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	inputs := make([]loopPublicationInput, 0, len(ext.Loops))
	for _, spec := range ext.Loops {
		inputs = append(inputs, loopPublicationInput{
			sourceKey: "extension/" + ext.Info.Name + "/loops/" + strings.TrimSpace(spec.Name),
			scope:     globalScope, owner: extensionOwner(ext.Info.Name), spec: looppkg.CloneResourceSpec(spec),
		})
	}
	syncer := &loopSourceSyncer{
		codec: codec,
		providers: []loopDeclarationProvider{func(context.Context) ([]loopPublicationInput, error) {
			return inputs, nil
		}},
	}
	desired, err := syncer.desiredLoops(ctx)
	if err != nil {
		return fmt.Errorf("daemon: project extension preview loop resources: %w", err)
	}
	for _, item := range desired {
		c.append(looppkg.ResourceKind, item.id, item.spec.Name, item.encoded)
	}
	return nil
}

func (c *extensionKitItemCollector) appendHookItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	ext *extensionpkg.Extension,
) error {
	if len(ext.Hooks) == 0 {
		return nil
	}
	codec, err := resources.ResolveCodec[hookspkg.HookDecl](codecs, hookBindingResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview hook codec: %w", err)
	}
	for _, hook := range ext.Hooks {
		scope := hookBindingScope(hook)
		validated, encoded, err := validateAndEncodeResource(ctx, codec, scope, hook)
		if err != nil {
			return fmt.Errorf("daemon: validate extension preview hook %q: %w", hook.Name, err)
		}
		c.append(
			hookBindingResourceKind,
			extensionHookBindingID(ext.Info.Name, validated.Name),
			validated.Name,
			encoded,
		)
	}
	return nil
}

func (c *extensionKitItemCollector) appendAutomationItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	ext *extensionpkg.Extension,
) error {
	if len(ext.AutomationJobs) == 0 && len(ext.AutomationTriggers) == 0 {
		return nil
	}
	jobCodec, err := resources.ResolveCodec[automationpkg.Job](codecs, automationpkg.JobResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview job codec: %w", err)
	}
	triggerCodec, err := resources.ResolveCodec[automationpkg.Trigger](codecs, automationpkg.TriggerResourceKind)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview trigger codec: %w", err)
	}
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for _, job := range ext.AutomationJobs {
		validated, encoded, err := validateAndEncodeResource(ctx, jobCodec, globalScope, job)
		if err != nil {
			return fmt.Errorf("daemon: validate extension preview job %q: %w", job.Name, err)
		}
		c.append(automationpkg.JobResourceKind, validated.ID, validated.Name, encoded)
	}
	for _, trigger := range ext.AutomationTriggers {
		validated, encoded, err := validateAndEncodeResource(ctx, triggerCodec, globalScope, trigger)
		if err != nil {
			return fmt.Errorf("daemon: validate extension preview trigger %q: %w", trigger.Name, err)
		}
		c.append(automationpkg.TriggerResourceKind, validated.ID, validated.Name, encoded)
	}
	return nil
}

func (c *extensionKitItemCollector) appendLayoutItems(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	ext *extensionpkg.Extension,
) error {
	if len(ext.Layouts) == 0 {
		return nil
	}
	codec, err := resources.ResolveCodec[windowmanager.LayoutResource](
		codecs,
		windowmanager.WindowLayoutResourceKind,
	)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension preview layout codec: %w", err)
	}
	globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	for _, layout := range ext.Layouts {
		name := strings.TrimSpace(layout.ID)
		id := "extension/" + ext.Info.Name + "/window_layout/" + name
		materialized := windowmanager.CloneLayoutResource(layout)
		materialized.ID = id
		_, encoded, err := validateAndEncodeResource(ctx, codec, globalScope, materialized)
		if err != nil {
			return fmt.Errorf("daemon: validate extension preview layout %q: %w", name, err)
		}
		c.append(windowmanager.WindowLayoutResourceKind, id, name, encoded)
	}
	return nil
}

func (c *extensionKitItemCollector) result() []extensionpkg.KitItem {
	result := make([]extensionpkg.KitItem, 0, len(c.items))
	for _, item := range c.items {
		result = append(result, item)
	}
	sortExtensionKitItems(result)
	return result
}
