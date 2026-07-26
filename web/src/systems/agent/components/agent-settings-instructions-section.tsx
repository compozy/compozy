import { Bot } from "lucide-react";

import { Field, FieldError, FieldLabel, FormSection, Textarea } from "@agh/ui";

import type { AgentSettingsDraft, AgentSettingsValidation } from "../lib/agent-settings-draft";

export interface AgentSettingsInstructionsSectionProps {
  draft: AgentSettingsDraft;
  errors: AgentSettingsValidation["fields"];
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
}

export function AgentSettingsInstructionsSection({
  draft,
  errors,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsInstructionsSectionProps) {
  return (
    <FormSection
      data-testid="agent-settings-instructions"
      icon={Bot}
      title="Instructions"
      description="AGENT.md body used when starting sessions from this agent."
    >
      <Field data-invalid={Boolean(errors.prompt)}>
        <FieldLabel htmlFor="agent-settings-prompt">Prompt</FieldLabel>
        <Textarea
          id="agent-settings-prompt"
          data-testid="agent-settings-prompt"
          variant="mono"
          className="min-h-form-textarea"
          value={draft.prompt}
          disabled={disabled}
          readOnly={readOnly}
          aria-readonly={readOnly || undefined}
          onChange={event => onPatch({ prompt: event.target.value })}
        />
        <FieldError data-testid="agent-settings-prompt-error">{errors.prompt}</FieldError>
      </Field>
    </FormSection>
  );
}
