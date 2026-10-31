import { AlertTriangle } from "@compozy/ui/icons";

import { Alert, AlertDescription, Field, FieldLabel, RadioCard } from "@compozy/ui";

import { AGENT_WORKSPACE_ACCESS_COPY } from "../lib/agent-access-copy";
import {
  agentWorkspaceAccessEditable,
  type AgentSettingsDraft,
  type AgentWorkspaceAccess,
} from "../lib/agent-settings-draft";

export interface AgentSettingsWorkspaceAccessFieldProps {
  draft: AgentSettingsDraft;
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
}

const OPTIONS: { value: AgentWorkspaceAccess; copy: { title: string; description: string } }[] = [
  { value: "", copy: AGENT_WORKSPACE_ACCESS_COPY.default },
  { value: "unrestricted", copy: AGENT_WORKSPACE_ACCESS_COPY.unrestricted },
];

/**
 * The per-agent workspace grant (USW S6). Unrestricted shows an inline warning
 * and saves without an extra dialog; definitions from a marketplace or an
 * extension never carry the grant, so the field is read-only with a note.
 */
export function AgentSettingsWorkspaceAccessField({
  draft,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsWorkspaceAccessFieldProps) {
  const editable = agentWorkspaceAccessEditable(draft);
  const locked = readOnly || !editable;
  return (
    <Field data-testid="agent-settings-workspace-access">
      <FieldLabel id="agent-settings-workspace-access-label">
        {AGENT_WORKSPACE_ACCESS_COPY.label}
      </FieldLabel>
      <div
        aria-labelledby="agent-settings-workspace-access-label"
        className="grid gap-2 sm:grid-cols-2"
        role="radiogroup"
      >
        {OPTIONS.map(option => (
          <RadioCard
            key={option.value || "default"}
            aria-disabled={locked || undefined}
            data-testid={`agent-settings-workspace-access-${option.value || "default"}`}
            description={option.copy.description}
            disabled={disabled}
            onSelect={() => {
              if (!locked) onPatch({ workspaceAccess: option.value });
            }}
            selected={draft.workspaceAccess === option.value}
            title={option.copy.title}
          />
        ))}
      </div>
      {!editable ? (
        <p
          className="text-small-body text-muted"
          data-testid="agent-settings-workspace-access-readonly"
        >
          {AGENT_WORKSPACE_ACCESS_COPY.readOnlyNote}
        </p>
      ) : draft.workspaceAccess === "unrestricted" ? (
        <Alert data-testid="agent-settings-workspace-access-warning" variant="warning">
          <AlertTriangle aria-hidden="true" className="size-4" />
          <AlertDescription>{AGENT_WORKSPACE_ACCESS_COPY.warning}</AlertDescription>
        </Alert>
      ) : null}
    </Field>
  );
}
