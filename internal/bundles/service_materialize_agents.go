package bundles

import (
	"context"

	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/soul"
)

type materializedAgentResources struct {
	agents     []ownedAgentResource
	souls      []ownedSoulResource
	heartbeats []ownedHeartbeatResource
	inventory  []InventoryItem
}

func materializeActivationAgentResources(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	scope resources.ResourceScope,
) (materializedAgentResources, error) {
	result := materializedAgentResources{
		agents:     make([]ownedAgentResource, 0, len(profile.Agents)),
		souls:      make([]ownedSoulResource, 0, len(profile.Agents)),
		heartbeats: make([]ownedHeartbeatResource, 0, len(profile.Agents)),
		inventory:  make([]InventoryItem, 0, len(profile.Agents)*3),
	}
	for _, agentDef := range profile.Agents {
		agent, agentID, err := materializeAgent(activation, bundle, profile, agentDef)
		if err != nil {
			return materializedAgentResources{}, err
		}
		result.agents = append(result.agents, ownedAgentResource{ID: agentID, Scope: scope, Spec: agent})
		result.inventory = append(result.inventory, InventoryItem{
			ActivationID: activation.ID,
			ResourceKind: string(aghconfig.AgentResourceKind),
			ResourceID:   agentID,
			ResourceName: agent.Name,
		})
		result = appendActivationAgentSidecars(result, activation, agent, agentID, agentDef, scope)
	}
	return result, nil
}

func appendActivationAgentSidecars(
	result materializedAgentResources,
	activation Activation,
	agent aghconfig.AgentDef,
	agentID string,
	agentDef extensionpkg.BundleAgent,
	scope resources.ResourceScope,
) materializedAgentResources {
	if agentDef.Soul != nil {
		sidecarID := stableID("sol", activation.ID, agent.Name)
		result.souls = append(result.souls, ownedSoulResource{
			ID:    sidecarID,
			Scope: scope,
			Spec: soul.ResourceSpec{
				AgentName:       agent.Name,
				AgentResourceID: agentID,
				SourcePath:      bundleAgentSyntheticSourcePath(activation.ID, agent.Name, soul.FileName),
				Body:            agentDef.Soul.Body,
			},
		})
		result.inventory = appendSidecarInventory(
			result.inventory,
			activation.ID,
			soul.ResourceKind,
			sidecarID,
			agent.Name,
		)
	}
	if agentDef.Heartbeat != nil {
		sidecarID := stableID("hbt", activation.ID, agent.Name)
		result.heartbeats = append(result.heartbeats, ownedHeartbeatResource{
			ID:    sidecarID,
			Scope: scope,
			Spec: heartbeat.ResourceSpec{
				AgentName:       agent.Name,
				AgentResourceID: agentID,
				SourcePath:      bundleAgentSyntheticSourcePath(activation.ID, agent.Name, heartbeat.FileName),
				Body:            agentDef.Heartbeat.Body,
			},
		})
		result.inventory = appendSidecarInventory(
			result.inventory,
			activation.ID,
			heartbeat.ResourceKind,
			sidecarID,
			agent.Name,
		)
	}
	return result
}

func appendSidecarInventory(
	inventory []InventoryItem,
	activationID string,
	kind resources.ResourceKind,
	resourceID string,
	resourceName string,
) []InventoryItem {
	return append(inventory, InventoryItem{
		ActivationID: activationID,
		ResourceKind: string(kind),
		ResourceID:   resourceID,
		ResourceName: resourceName,
	})
}

func materializeActivationAutomation(
	activation Activation,
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
) ([]automationpkg.Job, []automationpkg.Trigger, []InventoryItem, error) {
	jobs := make([]automationpkg.Job, 0, len(profile.Jobs))
	triggers := make([]automationpkg.Trigger, 0, len(profile.Triggers))
	inventory := make([]InventoryItem, 0, len(profile.Jobs)+len(profile.Triggers))
	for _, jobDef := range profile.Jobs {
		job, err := materializeJob(activation, bundle, profile, jobDef)
		if err != nil {
			return nil, nil, nil, err
		}
		jobs = append(jobs, job)
		inventory = append(inventory, InventoryItem{
			ActivationID: activation.ID,
			ResourceKind: string(automationpkg.JobResourceKind),
			ResourceID:   job.ID,
			ResourceName: job.Name,
		})
	}
	for _, triggerDef := range profile.Triggers {
		trigger, err := materializeTrigger(activation, bundle, profile, triggerDef)
		if err != nil {
			return nil, nil, nil, err
		}
		triggers = append(triggers, trigger)
		inventory = append(inventory, InventoryItem{
			ActivationID: activation.ID,
			ResourceKind: string(automationpkg.TriggerResourceKind),
			ResourceID:   trigger.ID,
			ResourceName: trigger.Name,
		})
	}
	return jobs, triggers, inventory, nil
}

func (s *Service) materializeActivationBridges(
	ctx context.Context,
	activation Activation,
	bundleRecord resources.Record[BundleResourceSpec],
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
) ([]bridgepkg.BridgeInstance, []InventoryItem, error) {
	bridges := make([]bridgepkg.BridgeInstance, 0, len(profile.Bridges))
	inventory := make([]InventoryItem, 0, len(profile.Bridges))
	for _, bridgeDef := range profile.Bridges {
		instance, err := s.materializeBridge(ctx, activation, bundleRecord, bundle, profile, bridgeDef)
		if err != nil {
			return nil, nil, err
		}
		bridges = append(bridges, instance)
		inventory = append(inventory, InventoryItem{
			ActivationID: activation.ID,
			ResourceKind: string(bridgepkg.BridgeInstanceResourceKind),
			ResourceID:   instance.ID,
			ResourceName: instance.DisplayName,
		})
	}
	return bridges, inventory, nil
}

func materializedInventoryCapacity(profile extensionpkg.BundleProfile) int {
	return len(profile.Layouts) +
		len(profile.Agents)*3 +
		len(profile.Jobs) +
		len(profile.Triggers) +
		len(profile.Bridges)
}
