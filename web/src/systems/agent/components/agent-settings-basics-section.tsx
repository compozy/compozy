import { Bot } from "lucide-react";

import { Field, FieldDescription, FieldError, FieldLabel, FormSection, Input } from "@agh/ui";

import type { AgentSettingsDraft, AgentSettingsValidation } from "../lib/agent-settings-draft";

export interface AgentSettingsBasicsSectionProps {
  draft: AgentSettingsDraft;
  errors: AgentSettingsValidation["fields"];
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
}

export function AgentSettingsBasicsSection({
  draft,
  errors,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsBasicsSectionProps) {
  return (
    <FormSection
      data-testid="agent-settings-basics"
      icon={Bot}
      title="Basics"
      description="Identity fields for this agent definition."
    >
      <Field>
        <FieldLabel htmlFor="agent-settings-name">Name</FieldLabel>
        <Input
          id="agent-settings-name"
          data-testid="agent-settings-name"
          value={draft.name}
          readOnly
          aria-readonly="true"
          className="font-mono"
        />
        <FieldDescription>Renaming is not supported.</FieldDescription>
      </Field>
      <Field data-invalid={Boolean(errors.categoryPath)}>
        <FieldLabel htmlFor="agent-settings-category">Category path</FieldLabel>
        <Input
          id="agent-settings-category"
          data-testid="agent-settings-category"
          value={draft.categoryPath}
          disabled={disabled}
          readOnly={readOnly}
          aria-readonly={readOnly || undefined}
          onChange={event => onPatch({ categoryPath: event.target.value })}
          placeholder="ops/release"
        />
        <FieldDescription>
          Slash-separated segments. Leave blank for uncategorized.
        </FieldDescription>
        <FieldError data-testid="agent-settings-category-error">{errors.categoryPath}</FieldError>
      </Field>
    </FormSection>
  );
}
