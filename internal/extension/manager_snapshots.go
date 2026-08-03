package extensionpkg

import (
	"slices"
	"strings"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"

	looppkg "github.com/compozy/compozy/internal/loop"

	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (m *Manager) cloneExtension(ext *managedExtension) *Extension {
	if ext == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	clone := &Extension{
		Info:                  cloneExtensionInfo(ext.info),
		RootDir:               ext.rootDir,
		GrantedPermissions:    slices.Clone(ext.grantedPermissions),
		GrantedSecurity:       slices.Clone(ext.grantedSecurity),
		GrantedResourceKinds:  slices.Clone(ext.grantedResourceKinds),
		GrantedResourceScopes: slices.Clone(ext.grantedResourceScopes),
		Status:                m.statusLocked(ext),
	}
	if ext.manifest != nil {
		clone.Manifest = cloneManifest(ext.manifest)
	}
	for _, decl := range ext.hooks {
		clone.Hooks = append(clone.Hooks, cloneHookDecl(decl))
	}
	for _, agent := range ext.agents {
		clone.Agents = append(clone.Agents, compozyconfig.CloneAgentDef(agent))
	}
	for _, agent := range ext.staticAgents {
		clone.StaticAgents = append(clone.StaticAgents, cloneStaticAgent(agent))
	}
	clone.AutomationJobs = cloneExtensionAutomationJobs(ext.automationJobs)
	clone.AutomationTriggers = cloneExtensionAutomationTriggers(ext.automationTriggers)
	for _, layout := range ext.layouts {
		clone.Layouts = append(clone.Layouts, windowmanager.CloneLayoutResource(layout))
	}
	if len(ext.skills) > 0 {
		clone.Skills = make([]*skillspkg.Skill, 0, len(ext.skills))
		for _, skill := range ext.skills {
			clone.Skills = append(clone.Skills, cloneSkillSnapshot(skill))
		}
	}
	if len(ext.loops) > 0 {
		clone.Loops = make([]looppkg.ResourceSpec, 0, len(ext.loops))
		for _, loop := range ext.loops {
			clone.Loops = append(clone.Loops, looppkg.CloneResourceSpec(loop))
		}
	}
	if ext.initialize != nil {
		clone.InitializeResult = cloneInitializeResponse(ext.initialize)
	}
	return clone
}

func (m *Manager) reportBridgeRuntimeIssue(bridgeInstanceID string, status bridgepkg.BridgeStatus, reason error) {
	if m == nil || m.bridgeTelemetrySink == nil {
		return
	}
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" || reason == nil {
		return
	}
	m.bridgeTelemetrySink.RecordBridgeRuntimeIssue(trimmedID, status, reason.Error())
}

func (m *Manager) reportBridgeRuntimeIssues(bridgeInstanceIDs []string, status bridgepkg.BridgeStatus, reason error) {
	if len(bridgeInstanceIDs) == 0 {
		return
	}
	for _, bridgeInstanceID := range bridgeInstanceIDs {
		m.reportBridgeRuntimeIssue(bridgeInstanceID, status, reason)
	}
}

func (m *Manager) clearBridgeRuntimeIssue(bridgeInstanceID string) {
	if m == nil || m.bridgeTelemetrySink == nil {
		return
	}
	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID == "" {
		return
	}
	m.bridgeTelemetrySink.ClearBridgeRuntimeIssue(trimmedID)
}

func (m *Manager) clearBridgeRuntimeIssues(bridgeInstanceIDs []string) {
	if len(bridgeInstanceIDs) == 0 {
		return
	}
	for _, bridgeInstanceID := range bridgeInstanceIDs {
		m.clearBridgeRuntimeIssue(bridgeInstanceID)
	}
}

func managedBridgeInstanceIDs(ext *managedExtension) []string {
	if ext == nil || ext.runtime.Bridge == nil {
		return nil
	}
	return ext.runtime.Bridge.ManagedBridgeInstanceIDs()
}
