import { Wrench } from "lucide-react";

import { FormSection } from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { TokenListField } from "./token-list-field";

export interface AgentCreateToolsSectionProps {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/**
 * Advanced tier: the allow/deny surface a session inherits.
 *
 * There is no MCP-servers control here: `CreateAgentPayload` has no
 * `mcp_servers` field, and the read-only `AgentPayload.MCPServers` is populated
 * on the response path only. Authoring MCP-on-agent is a contract change, not a
 * form field (T1/D5).
 */
export function AgentCreateToolsSection({
  draft,
  errors,
  onDraftChange,
}: AgentCreateToolsSectionProps) {
  return (
    <FormSection
      data-testid="agent-create-tools-section"
      help="Empty lists mean the runtime defaults apply."
      icon={Wrench}
      title="Tools & skills"
    >
      <div className="grid gap-3.5 md:grid-cols-2">
        <TokenListField
          error={errors.tools}
          help="Canonical tool IDs or namespace wildcards."
          label="Allowed tools"
          onChange={tools => onDraftChange({ ...draft, tools })}
          placeholder="compozy__skill_view, mcp__github__*"
          testId="agent-create-tools"
          values={draft.tools}
        />
        <TokenListField
          error={errors.toolsets}
          help="Tool groups enabled for the agent."
          label="Allowed toolsets"
          onChange={toolsets => onDraftChange({ ...draft, toolsets })}
          placeholder="compozy__catalog"
          testId="agent-create-toolsets"
          values={draft.toolsets}
        />
        <TokenListField
          error={errors.denyTools}
          help="Canonical tools to deny after allow rules."
          label="Denied tools"
          onChange={denyTools => onDraftChange({ ...draft, denyTools })}
          placeholder="compozy__task_*"
          testId="agent-create-deny-tools"
          values={draft.denyTools}
        />
        <TokenListField
          help="Skill names disabled only for this agent."
          label="Disabled skills"
          onChange={disabledSkills => onDraftChange({ ...draft, disabledSkills })}
          placeholder="code-review, release-notes"
          testId="agent-create-disabled-skills"
          values={draft.disabledSkills}
        />
      </div>
    </FormSection>
  );
}
