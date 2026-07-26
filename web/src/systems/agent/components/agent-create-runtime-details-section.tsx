import { Terminal } from "lucide-react";

import { Field, FieldHeader, FieldLabel, FormSection, HelpTip, Input } from "@agh/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";

export interface AgentCreateRuntimeDetailsSectionProps {
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/** Advanced tier: overrides for how the provider subprocess launches. */
export function AgentCreateRuntimeDetailsSection({
  draft,
  onDraftChange,
}: AgentCreateRuntimeDetailsSectionProps) {
  return (
    <FormSection
      data-testid="agent-create-runtime-details"
      help="Applies only to this agent. Everything here falls back to the provider's own configuration when left empty."
      icon={Terminal}
      title="Launch overrides"
    >
      <Field>
        <FieldHeader>
          <FieldLabel htmlFor="agent-create-command">Runtime command</FieldLabel>
          <HelpTip label="About runtime command">
            Overrides the executable this agent's provider launches. Leave it empty and the
            provider's own command is used.
          </HelpTip>
        </FieldHeader>
        <Input
          className="font-mono"
          data-testid="agent-create-command"
          id="agent-create-command"
          onChange={event => onDraftChange({ ...draft, command: event.target.value })}
          placeholder="provider default"
          value={draft.command}
        />
      </Field>
    </FormSection>
  );
}
