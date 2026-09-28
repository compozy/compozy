import { Wrench } from "lucide-react";

import { FormSection } from "@compozy/ui";

import { AGENT_ACCESS_COPY } from "../lib/agent-access-copy";
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
      icon={Wrench}
      title={AGENT_ACCESS_COPY.title}
    >
      <div className="grid gap-3.5 md:grid-cols-2">
        <TokenListField
          error={errors.tools}
          description={AGENT_ACCESS_COPY.tools.description}
          help={AGENT_ACCESS_COPY.tools.help}
          label={AGENT_ACCESS_COPY.tools.label}
          onChange={tools => onDraftChange({ ...draft, tools })}
          placeholder={AGENT_ACCESS_COPY.tools.placeholder}
          testId="agent-create-tools"
          values={draft.tools}
        />
        <TokenListField
          error={errors.toolsets}
          description={AGENT_ACCESS_COPY.toolsets.description}
          help={AGENT_ACCESS_COPY.toolsets.help}
          label={AGENT_ACCESS_COPY.toolsets.label}
          onChange={toolsets => onDraftChange({ ...draft, toolsets })}
          placeholder={AGENT_ACCESS_COPY.toolsets.placeholder}
          testId="agent-create-toolsets"
          values={draft.toolsets}
        />
        <TokenListField
          error={errors.denyTools}
          description={AGENT_ACCESS_COPY.denyTools.description}
          help={AGENT_ACCESS_COPY.denyTools.help}
          label={AGENT_ACCESS_COPY.denyTools.label}
          onChange={denyTools => onDraftChange({ ...draft, denyTools })}
          placeholder={AGENT_ACCESS_COPY.denyTools.placeholder}
          testId="agent-create-deny-tools"
          values={draft.denyTools}
        />
        <TokenListField
          description={AGENT_ACCESS_COPY.disabledSkills.description}
          help={AGENT_ACCESS_COPY.disabledSkills.help}
          label={AGENT_ACCESS_COPY.disabledSkills.label}
          onChange={disabledSkills => onDraftChange({ ...draft, disabledSkills })}
          placeholder={AGENT_ACCESS_COPY.disabledSkills.placeholder}
          testId="agent-create-disabled-skills"
          values={draft.disabledSkills}
        />
      </div>
    </FormSection>
  );
}
