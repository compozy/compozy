import { Wrench } from "lucide-react";

import { FormSection } from "@agh/ui";

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
      description="Empty lists mean the runtime defaults apply."
      icon={Wrench}
      title="Tools & skills"
    >
      <div className="grid gap-3.5 md:grid-cols-2">
        <TokenListField
          description="Canonical tool IDs or namespace wildcards."
          error={errors.tools}
          label="Allowed tools"
          onChange={tools => onDraftChange({ ...draft, tools })}
          placeholder="agh__skill_view, mcp__github__*"
          testId="agent-create-tools"
          values={draft.tools}
        />
        <TokenListField
          description="Tool groups enabled for the agent."
          error={errors.toolsets}
          label="Allowed toolsets"
          onChange={toolsets => onDraftChange({ ...draft, toolsets })}
          placeholder="agh__catalog"
          testId="agent-create-toolsets"
          values={draft.toolsets}
        />
        <TokenListField
          description="Canonical tools to deny after allow rules."
          error={errors.denyTools}
          label="Denied tools"
          onChange={denyTools => onDraftChange({ ...draft, denyTools })}
          placeholder="agh__task_*"
          testId="agent-create-deny-tools"
          values={draft.denyTools}
        />
        <TokenListField
          description="Skill names disabled only for this agent."
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
