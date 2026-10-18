import { Terminal } from "lucide-react";

import { Field, FieldHeader, FieldLabel, FormSection, HelpTip, Input } from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";

export interface AgentCreateRuntimeDetailsSectionProps {
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/** Advanced tier: a custom command for starting this agent's provider. */
export function AgentCreateRuntimeDetailsSection({
  draft,
  onDraftChange,
}: AgentCreateRuntimeDetailsSectionProps) {
  return (
    <FormSection
      data-testid="agent-create-runtime-details"
      help="Applies only to this agent. Leave it empty to use the provider's own command."
      icon={Terminal}
      title="Advanced: custom command"
    >
      <Field>
        <FieldHeader>
          <FieldLabel htmlFor="agent-create-command">Command</FieldLabel>
          <HelpTip label="About command">
            The program CompozyOS starts for this agent's provider. Leave it empty to use the
            default.
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
