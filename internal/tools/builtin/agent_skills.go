package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func agentSkillDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeDescriptor(
			toolspkg.ToolIDAgentSkillPropose,
			"agent_skill_propose",
			"Propose Agent Skill",
			"Propose a reusable skill for the calling stays-on instance (experimental). "+
				"The draft stays inactive until an operator reviews and approves its exact digest. "+
				"Agent credentials cannot approve it.",
			`{"type":"object","required":["name","description","body"],"properties":{"name":{"type":"string","pattern":"^[a-z0-9][a-z0-9-]{0,63}$"},"description":{"type":"string","minLength":1},"body":{"type":"string","minLength":1},"files":{"type":"array","maxItems":64,"items":{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"}},"additionalProperties":false}},"evidence_session_ids":{"type":"array","maxItems":20,"items":{"type":"string"}}},"additionalProperties":false}`,
			toolspkg.RiskMutating,
			false,
			false,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
			[]string{"agents", skillsSkillsKey},
			[]string{"propose skill", "review skill"},
		),
		nativeDescriptor(
			toolspkg.ToolIDAgentSkillReviews,
			"agent_skill_reviews",
			"Agent Skill Reviews",
			"Read pending, approved or rejected skill reviews for the calling stays-on instance (experimental).",
			`{"type":"object","properties":{"state":{"type":"string","enum":["pending","approved","rejected"]},"cursor":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`,
			toolspkg.RiskRead,
			true,
			false,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext},
			[]string{"agents", skillsSkillsKey},
			[]string{"skill review status"},
		),
	}
}
