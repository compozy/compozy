package extensionpkg

import (
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"

	looppkg "github.com/compozy/agh/internal/loop"

	skillspkg "github.com/compozy/agh/internal/skills"
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
		GrantedActions:        slices.Clone(ext.grantedActions),
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
		clone.Agents = append(clone.Agents, aghconfig.CloneAgentDef(agent))
	}
	clone.Bundles = cloneBundleSpecs(ext.bundles)
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

func cloneBundleSpecs(values []BundleSpec) []BundleSpec {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]BundleSpec, 0, len(values))
	for _, value := range values {
		next := BundleSpec{
			Name:        strings.TrimSpace(value.Name),
			Description: strings.TrimSpace(value.Description),
			Profiles:    make([]BundleProfile, 0, len(value.Profiles)),
		}
		for _, profile := range value.Profiles {
			clonedProfile := BundleProfile{
				Name:        strings.TrimSpace(profile.Name),
				Description: strings.TrimSpace(profile.Description),
				Channels: BundleChannelsConfig{
					Primary: strings.TrimSpace(profile.Channels.Primary),
					Items:   normalizeBundleChannels(profile.Channels.Items),
				},
				Layouts:  cloneBundleLayouts(profile.Layouts),
				Agents:   cloneBundleAgents(profile.Agents),
				Jobs:     append([]BundleJob(nil), profile.Jobs...),
				Triggers: append([]BundleTrigger(nil), profile.Triggers...),
				Bridges:  normalizeBundleBridges(profile.Bridges),
			}
			for idx := range clonedProfile.Jobs {
				clonedProfile.Jobs[idx].Task = cloneBundleTaskConfig(clonedProfile.Jobs[idx].Task)
			}
			next.Profiles = append(next.Profiles, clonedProfile)
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneBundleAgents(values []BundleAgent) []BundleAgent {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]BundleAgent, 0, len(values))
	for _, value := range values {
		next := BundleAgent{
			Path:  strings.TrimSpace(value.Path),
			Agent: aghconfig.CloneAgentDef(value.Agent),
		}
		if value.Soul != nil {
			soul := *value.Soul
			next.Soul = &soul
		}
		if value.Heartbeat != nil {
			heartbeat := *value.Heartbeat
			next.Heartbeat = &heartbeat
		}
		cloned = append(cloned, next)
	}
	return cloned
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
