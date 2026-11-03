import { Settings2, ShieldCheck } from "@compozy/ui/icons";

import { Field, FormSection, Pill, RadioCard } from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";
import {
  AGENT_PERMISSION_LEVELS,
  FOLLOW_SETTINGS_LABEL,
  STAYS_ON_DEFAULT_PERMISSION,
} from "../lib/agent-permissions";

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
  const shown = staysOn && unset ? STAYS_ON_DEFAULT_PERMISSION : draft.permissions;
  return (
    <FormSection
      data-testid="agent-create-permissions-section"
      help="Applies to new sessions with this agent."
      icon={ShieldCheck}
      title="What can it do on its own?"
    >
      <Field>
        <div
          aria-label="Permission level"
          className="flex flex-col gap-2"
          data-testid="agent-create-permissions"
          role="radiogroup"
        >
          {staysOn ? null : (
            <RadioCard
              data-testid="agent-create-permissions-inherit"
              description="Uses the level in your Settings."
              icon={Settings2}
              onSelect={() => onDraftChange({ ...draft, permissions: "" })}
              selected={unset}
              title={FOLLOW_SETTINGS_LABEL}
            />
          )}
          {AGENT_PERMISSION_LEVELS.map(option => (
            <RadioCard
              badge={
                staysOn && unset && option.value === STAYS_ON_DEFAULT_PERMISSION ? (
                  <Pill size="sm" tone="neutral">
                    Default
                  </Pill>
                ) : undefined
              }
              data-testid={`agent-create-permissions-${option.value}`}
              description={option.description}
              icon={option.icon}
              key={option.value}
              onSelect={() => onDraftChange({ ...draft, permissions: option.value })}
              selected={shown === option.value}
              title={option.label}
            />
          ))}
        </div>
      </Field>
    </FormSection>
  );
}
