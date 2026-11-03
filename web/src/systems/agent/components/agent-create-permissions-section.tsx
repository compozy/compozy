import { Settings2, ShieldCheck } from "@compozy/ui/icons";

import { Field, FormSection, Pill, RadioCardGroup, type RadioCardGroupOption } from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";
import {
  AGENT_PERMISSION_LEVELS,
  FOLLOW_SETTINGS_LABEL,
  isAgentPermissionLevel,
  STAYS_ON_DEFAULT_PERMISSION,
} from "../lib/agent-permissions";

/** The radio value for "Follow Settings", which stores no `permissions`. */
const INHERIT = "inherit";

export interface AgentCreatePermissionsSectionProps {
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/**
 * Advanced tier: the permission level every session inherits. An on-demand agent can leave it to
 * Settings by omitting `permissions`; a stays-on agent with nothing set runs at Edit on its own.
 */
export function AgentCreatePermissionsSection({
  draft,
  onDraftChange,
}: AgentCreatePermissionsSectionProps) {
  const staysOn = draft.kind === "stays_on";
  const unset = draft.permissions === "";
  const shown = unset ? (staysOn ? STAYS_ON_DEFAULT_PERMISSION : INHERIT) : draft.permissions;
  const options: RadioCardGroupOption<string>[] = [
    ...(staysOn
      ? []
      : [
          {
            value: INHERIT,
            title: FOLLOW_SETTINGS_LABEL,
            description: "Uses the level in your Settings.",
            icon: Settings2,
            "data-testid": "agent-create-permissions-inherit",
          },
        ]),
    ...AGENT_PERMISSION_LEVELS.map(option => ({
      value: option.value,
      title: option.label,
      description: option.description,
      icon: option.icon,
      "data-testid": `agent-create-permissions-${option.value}`,
      badge:
        staysOn && unset && option.value === STAYS_ON_DEFAULT_PERMISSION ? (
          <Pill size="sm" tone="neutral">
            Default
          </Pill>
        ) : undefined,
    })),
  ];
  return (
    <FormSection
      data-testid="agent-create-permissions-section"
      help="Applies to new sessions with this agent."
      icon={ShieldCheck}
      title="What can it do on its own?"
    >
      <Field>
        <RadioCardGroup
          aria-label="Permission level"
          className="flex flex-col"
          data-testid="agent-create-permissions"
          onValueChange={next =>
            onDraftChange({ ...draft, permissions: isAgentPermissionLevel(next) ? next : "" })
          }
          options={options}
          value={shown}
        />
      </Field>
    </FormSection>
  );
}
