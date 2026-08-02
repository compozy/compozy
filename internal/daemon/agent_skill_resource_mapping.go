package daemon

import (
	"context"

	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/resources"

	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
)

func appendAgentResources(
	desired *agentSkillDesiredResources,
	scope resources.ResourceScope,
	sourcePrefix string,
	agents []compozyconfig.AgentDef,
) {
	if desired == nil {
		return
	}
	for _, agent := range agents {
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			continue
		}
		desired.agents = append(desired.agents, agentPublicationInput{
			sourceKey: sourcePrefix + "/agent/" + name,
			scope:     scope,
			spec:      cloneAgentDef(agent),
		})
		for _, server := range agent.MCPServers {
			serverName := strings.TrimSpace(server.Name)
			if serverName == "" {
				continue
			}
			desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
				sourceKey: sourcePrefix + "/agent/" + name + "/mcp_server/" + serverName,
				scope:     scope,
				spec:      cloneDaemonMCPServer(server),
			})
		}
	}
}

func appendExtensionAgentResources(
	desired *agentSkillDesiredResources,
	scope resources.ResourceScope,
	extensionName string,
	agents []extensionpkg.StaticAgent,
) {
	if desired == nil {
		return
	}
	owner := extensionOwner(extensionName)
	for _, staticAgent := range agents {
		name := strings.TrimSpace(staticAgent.Agent.Name)
		if name == "" {
			continue
		}
		agentSourceKey := "extension/" + strings.TrimSpace(extensionName) + "/agent/" + name
		desired.agents = append(desired.agents, agentPublicationInput{
			sourceKey: agentSourceKey,
			scope:     scope,
			owner:     owner,
			spec:      cloneAgentDef(staticAgent.Agent),
		})
		for _, server := range staticAgent.Agent.MCPServers {
			serverName := strings.TrimSpace(server.Name)
			if serverName == "" {
				continue
			}
			desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
				sourceKey: agentSourceKey + "/mcp_server/" + serverName,
				scope:     scope,
				owner:     owner,
				spec:      cloneDaemonMCPServer(server),
			})
		}
		if staticAgent.Soul != nil {
			desired.souls = append(desired.souls, soulPublicationInput{
				sourceKey:      agentSourceKey + "/soul",
				agentSourceKey: agentSourceKey,
				scope:          scope,
				owner:          owner,
				agentName:      name,
				sourcePath:     staticAgent.Soul.SourcePath,
				body:           staticAgent.Soul.Body,
			})
		}
		if staticAgent.Heartbeat != nil {
			desired.heartbeats = append(desired.heartbeats, heartbeatPublicationInput{
				sourceKey:      agentSourceKey + "/heartbeat",
				agentSourceKey: agentSourceKey,
				scope:          scope,
				owner:          owner,
				agentName:      name,
				sourcePath:     staticAgent.Heartbeat.SourcePath,
				body:           staticAgent.Heartbeat.Body,
			})
		}
	}
}

func appendSkillResources(
	desired *agentSkillDesiredResources,
	scope resources.ResourceScope,
	sourcePrefix string,
	skills []*skillspkg.Skill,
) {
	if desired == nil {
		return
	}
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		name := strings.TrimSpace(skill.Meta.Name)
		if name == "" {
			continue
		}
		desired.skills = append(desired.skills, skillPublicationInput{
			sourceKey: sourcePrefix + "/skill/" + name,
			scope:     scope,
			spec:      skillspkg.SkillToResourceSpec(skill),
		})
		for _, server := range skill.MCPServers {
			serverName := strings.TrimSpace(server.Name)
			if serverName == "" {
				continue
			}
			desired.mcpServers = append(desired.mcpServers, mcpServerPublicationInput{
				sourceKey: sourcePrefix + "/skill/" + name + "/mcp_server/" + serverName,
				scope:     scope,
				spec:      mcpServerFromSkillDecl(server),
			})
		}
	}
}

func appendExtensionSkillResources(
	desired *agentSkillDesiredResources,
	scope resources.ResourceScope,
	extensionName string,
	skills []*skillspkg.Skill,
) {
	if desired == nil {
		return
	}
	firstSkill := len(desired.skills)
	firstMCP := len(desired.mcpServers)
	appendSkillResources(
		desired,
		scope,
		"extension/"+strings.TrimSpace(extensionName)+"/skills",
		skills,
	)
	owner := extensionOwner(extensionName)
	for idx := firstSkill; idx < len(desired.skills); idx++ {
		desired.skills[idx].owner = owner
	}
	for idx := firstMCP; idx < len(desired.mcpServers); idx++ {
		desired.mcpServers[idx].owner = owner
	}
}

func mcpServerFromSkillDecl(decl skillspkg.MCPServerDecl) compozyconfig.MCPServer {
	return compozyconfig.MCPServer{
		Name:    strings.TrimSpace(decl.Name),
		Command: strings.TrimSpace(decl.Command),
		Args:    slices.Clone(decl.Args),
		Env:     cloneStringMap(decl.Env),
	}
}

func validateAndEncodeAgent(
	ctx context.Context,
	codec resources.KindCodec[compozyconfig.AgentDef],
	scope resources.ResourceScope,
	spec compozyconfig.AgentDef,
) (compozyconfig.AgentDef, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return compozyconfig.AgentDef{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return compozyconfig.AgentDef{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return compozyconfig.AgentDef{}, nil, err
	}
	return validated, canonical, nil
}

func validateAndEncodeSkill(
	ctx context.Context,
	codec resources.KindCodec[skillspkg.SkillResourceSpec],
	scope resources.ResourceScope,
	spec skillspkg.SkillResourceSpec,
) (skillspkg.SkillResourceSpec, []byte, error) {
	encoded, err := codec.Encode(spec)
	if err != nil {
		return skillspkg.SkillResourceSpec{}, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return skillspkg.SkillResourceSpec{}, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return skillspkg.SkillResourceSpec{}, nil, err
	}
	return validated, canonical, nil
}

func cloneAgentDef(agent compozyconfig.AgentDef) compozyconfig.AgentDef {
	return compozyconfig.CloneAgentDef(agent)
}

func cloneSoulResourceSpec(spec soul.ResourceSpec) soul.ResourceSpec {
	return soul.ResourceSpec{
		AgentName:       strings.TrimSpace(spec.AgentName),
		AgentResourceID: strings.TrimSpace(spec.AgentResourceID),
		SourcePath:      strings.TrimSpace(spec.SourcePath),
		Body:            spec.Body,
	}
}

func cloneHeartbeatResourceSpec(spec heartbeat.ResourceSpec) heartbeat.ResourceSpec {
	return heartbeat.ResourceSpec{
		AgentName:       strings.TrimSpace(spec.AgentName),
		AgentResourceID: strings.TrimSpace(spec.AgentResourceID),
		SourcePath:      strings.TrimSpace(spec.SourcePath),
		Body:            spec.Body,
	}
}

func cloneSkillResourceSpec(src skillspkg.SkillResourceSpec) skillspkg.SkillResourceSpec {
	skill, err := skillspkg.SkillFromResourceSpec(src)
	if err != nil {
		return src
	}
	return skillspkg.SkillToResourceSpec(skill)
}

func agentRecordSortKey(record resources.Record[compozyconfig.AgentDef]) string {
	return string(record.Scope.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Scope.ID) + "\x00" +
		string(record.Source.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Source.ID) + "\x00" +
		strings.TrimSpace(record.ID)
}

func sidecarRecordSortKey[T any](record resources.Record[T]) string {
	return string(record.Scope.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Scope.ID) + "\x00" +
		string(record.Owner.Kind.Normalize()) + "\x00" +
		strings.TrimSpace(record.Owner.ID) + "\x00" +
		strings.TrimSpace(record.ID)
}
