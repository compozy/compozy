import { ShieldCheck } from "lucide-react";

import { FormSection } from "@compozy/ui";

import { AGENT_ACCESS_COPY } from "../lib/agent-access-copy";
import type { AgentSettingsDraft } from "../lib/agent-settings-draft";
import { TokenListField } from "./token-list-field";

export interface AgentSettingsAccessSectionProps {
  draft: AgentSettingsDraft;
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
}

export function AgentSettingsAccessSection({
  draft,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsAccessSectionProps) {
  return (
    <FormSection
      data-testid="agent-settings-access"
      icon={ShieldCheck}
      title={AGENT_ACCESS_COPY.title}
    >
      <div className="grid gap-3.5 md:grid-cols-2">
        <TokenListField
          description={AGENT_ACCESS_COPY.tools.description}
          help={AGENT_ACCESS_COPY.tools.help}
          disabled={disabled}
          readOnly={readOnly}
          label={AGENT_ACCESS_COPY.tools.label}
          onChange={tools => onPatch({ tools })}
          placeholder={AGENT_ACCESS_COPY.tools.placeholder}
          testId="agent-settings-tools"
          values={draft.tools}
        />
        <TokenListField
          description={AGENT_ACCESS_COPY.toolsets.description}
          help={AGENT_ACCESS_COPY.toolsets.help}
          disabled={disabled}
          readOnly={readOnly}
          label={AGENT_ACCESS_COPY.toolsets.label}
          onChange={toolsets => onPatch({ toolsets })}
          placeholder={AGENT_ACCESS_COPY.toolsets.placeholder}
          testId="agent-settings-toolsets"
          values={draft.toolsets}
        />
        <TokenListField
          description={AGENT_ACCESS_COPY.denyTools.description}
          help={AGENT_ACCESS_COPY.denyTools.help}
          disabled={disabled}
          readOnly={readOnly}
          label={AGENT_ACCESS_COPY.denyTools.label}
          onChange={denyTools => onPatch({ denyTools })}
          placeholder={AGENT_ACCESS_COPY.denyTools.placeholder}
          testId="agent-settings-deny-tools"
          values={draft.denyTools}
        />
        <TokenListField
          description={AGENT_ACCESS_COPY.disabledSkills.description}
          help={AGENT_ACCESS_COPY.disabledSkills.help}
          disabled={disabled}
          readOnly={readOnly}
          label={AGENT_ACCESS_COPY.disabledSkills.label}
          onChange={disabledSkills => onPatch({ disabledSkills })}
          placeholder={AGENT_ACCESS_COPY.disabledSkills.placeholder}
          testId="agent-settings-disabled-skills"
          values={draft.disabledSkills}
        />
      </div>
    </FormSection>
  );
}
