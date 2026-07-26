import type * as React from "react";

import { Bot } from "lucide-react";

import {
  Field,
  FieldError,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
  RequiredMark,
  Textarea,
} from "@agh/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";

export interface AgentCreateDefinitionSectionProps {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  /** Runtime + catalog pair, which belongs to naming the agent rather than to a section of its own. */
  children?: React.ReactNode;
}

/**
 * Simple tier: who this agent is and what it is responsible for.
 *
 * Name and prompt are the only fields the create contract rejects when empty,
 * so they stay in Simple — Advanced never hides a required field.
 */
export function AgentCreateDefinitionSection({
  draft,
  errors,
  onDraftChange,
  children,
}: AgentCreateDefinitionSectionProps) {
  return (
    <FormSection
      data-testid="agent-create-definition"
      help="Name and instructions are the only fields the daemon rejects when empty. Everything else falls back to the project defaults."
      icon={Bot}
      title="The definition"
    >
      <Field data-invalid={Boolean(errors.name)}>
        <FieldLabel htmlFor="agent-create-name">
          Agent name
          <RequiredMark />
        </FieldLabel>
        <Input
          aria-invalid={Boolean(errors.name)}
          autoFocus
          className="font-mono"
          data-testid="agent-create-name"
          id="agent-create-name"
          onChange={event => onDraftChange({ ...draft, name: event.target.value })}
          placeholder="release-captain"
          value={draft.name}
        />
        <FieldError data-testid="agent-create-name-error">{errors.name}</FieldError>
      </Field>

      <Field data-invalid={Boolean(errors.prompt)}>
        <FieldHeader>
          <FieldLabel htmlFor="agent-create-prompt">
            Instructions
            <RequiredMark />
          </FieldLabel>
          <HelpTip label="About instructions">
            The system prompt every session inherits — the agent's responsibility, its boundaries,
            and when it should escalate to a person.
          </HelpTip>
        </FieldHeader>
        <Textarea
          aria-invalid={Boolean(errors.prompt)}
          className="min-h-40"
          data-testid="agent-create-prompt"
          id="agent-create-prompt"
          onChange={event => onDraftChange({ ...draft, prompt: event.target.value })}
          placeholder="You are responsible for release readiness..."
          value={draft.prompt}
        />
        <FieldError data-testid="agent-create-prompt-error">{errors.prompt}</FieldError>
      </Field>

      {children}
    </FormSection>
  );
}
