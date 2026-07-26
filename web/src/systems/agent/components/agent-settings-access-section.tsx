import { ShieldCheck } from "lucide-react";

import { FormSection } from "@agh/ui";

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
      title="Access"
      description="Constrain the tools and skills available to sessions started from this agent."
    >
      <div className="grid gap-3.5 md:grid-cols-2">
        <TokenListField
          description="Canonical tool IDs or namespace wildcards."
          disabled={disabled}
          readOnly={readOnly}
          label="Tools"
          onChange={tools => onPatch({ tools })}
          placeholder="agh__skill_view, mcp__github__*"
          testId="agent-settings-tools"
          values={draft.tools}
        />
        <TokenListField
          description="Tool groups enabled for the agent."
          disabled={disabled}
          readOnly={readOnly}
          label="Toolsets"
          onChange={toolsets => onPatch({ toolsets })}
          placeholder="agh__catalog"
          testId="agent-settings-toolsets"
          values={draft.toolsets}
        />
        <TokenListField
          description="Canonical tools to deny after allow rules."
          disabled={disabled}
          readOnly={readOnly}
          label="Denied tools"
          onChange={denyTools => onPatch({ denyTools })}
          placeholder="agh__task_*"
          testId="agent-settings-deny-tools"
          values={draft.denyTools}
        />
        <TokenListField
          description="Skill names disabled only for this agent."
          disabled={disabled}
          readOnly={readOnly}
          label="Disabled skills"
          onChange={disabledSkills => onPatch({ disabledSkills })}
          placeholder="code-review, release-notes"
          testId="agent-settings-disabled-skills"
          values={draft.disabledSkills}
        />
      </div>
    </FormSection>
  );
}
