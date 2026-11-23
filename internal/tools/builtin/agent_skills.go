package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func agentSkillDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeDescriptor(
			toolspkg.ToolIDAgentSkillPropose,
			"agent_skill_propose",
			"Propose Agent Skill",
			"Propose a reusable skill for the calling always-on instance (experimental), with a lesson "+
				"(cue, lesson, evidence, confidence) and 1-20 sessions that show it, from this instance's "+
				"workspace (or its profile, for a User-scope instance). "+
				"The draft stays inactive until an operator reviews and approves its exact digest. "+
				"mode edit changes a skill already installed for this instance as an overlay: SKILL.md is regenerated "+
				"from name, description and body, files add or replace text files, remove_paths deletes files, and "+
				"every other installed file is kept exactly; approval succeeds only while the installed skill is unchanged. "+
				"Agent credentials cannot approve it. Re-submitting the same contents returns the pending review "+
				"with duplicate_of_pending; contents already rejected are refused. "+
				"When the operator turned on skill auto-approve for this instance, a high-confidence lesson whose "+
				"resulting tree runs nothing automatically (no hooks, MCP servers or scripts) and, for edits, changes "+
				"a skill this instance installed is approved at once (state approved, decided_via auto_approve), at "+
				"most 3 per UTC day; otherwise auto_approve_skipped names the first failing check. "+
				"Use for a reviewed, evidence-backed procedure change; for a quick unreviewed note, use compozy__agent_memory_add.",
			`{"type":"object","required":["name","description","body","lesson","evidence_session_ids"],"properties":{"name":{"type":"string","pattern":"^[a-z0-9][a-z0-9-]{0,63}$"},"description":{"type":"string","minLength":1},"body":{"type":"string","minLength":1},"files":{"type":"array","maxItems":64,"items":{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"}},"additionalProperties":false}},"mode":{"type":"string","enum":["new","edit"],"default":"new","description":"new proposes a skill not installed yet; edit changes the installed skill of that name"},"remove_paths":{"type":"array","maxItems":64,"items":{"type":"string","minLength":1},"description":"Edit only: installed files to delete"},"lesson":{"type":"object","required":["cue","lesson","evidence"],"properties":{"cue":{"type":"string","minLength":1,"maxLength":2000,"description":"When the lesson applies"},"lesson":{"type":"string","minLength":1,"maxLength":2000,"description":"What to do differently next time"},"evidence":{"type":"string","minLength":1,"maxLength":2000,"description":"What the cited sessions showed"},"confidence":{"type":"string","enum":["low","medium","high"],"default":"medium"}},"additionalProperties":false},"evidence_session_ids":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1}}},"additionalProperties":false}`,
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
			"Read pending, approved, rejected or reverted skill reviews, with their lessons and evidence sessions, "+
				"for the calling always-on instance (experimental).",
			`{"type":"object","properties":{"state":{"type":"string","enum":["pending","approved","rejected","reverted"]},"cursor":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`,
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
