import { useRef } from "react";

import { Disclosure, Field, FieldDescription, FieldError, FieldLabel, Input } from "@compozy/ui";

import { agentSlugFor, type AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { AgentCreateOrganizeSection } from "./agent-create-organize-section";
import { AgentCreatePermissionsSection } from "./agent-create-permissions-section";
import { AgentCreateRuntimeDetailsSection } from "./agent-create-runtime-details-section";
import { AgentCreateToolsSection } from "./agent-create-tools-section";

/** The one quiet fold: the Short name, and what the agent may use. */
export function AgentCreateAdvanced({
  draft,
  errors,
  open,
  onOpenChange,
  onDraftChange,
}: {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}) {
  const fold = useRef<HTMLDivElement>(null);
  return (
    <Disclosure
      label="Advanced"
      onOpenChange={next => {
        onOpenChange(next);
        if (next) requestAnimationFrame(() => fold.current?.scrollIntoView({ block: "nearest" }));
      }}
      open={open}
      ref={fold}
      size="md"
    >
      <div className="flex flex-col gap-6 pt-3">
        <Field className="max-w-80" data-invalid={Boolean(errors.name)}>
          <FieldLabel htmlFor="agent-create-short-name">Short name</FieldLabel>
          <Input
            className="font-mono"
            data-testid="agent-create-short-name"
            id="agent-create-short-name"
            onChange={event => onDraftChange({ ...draft, name: event.target.value })}
            placeholder={agentSlugFor(draft.displayName) || "reviewer"}
            value={draft.name}
          />
          <FieldDescription>Used to mention it in a message.</FieldDescription>
          <FieldError data-testid="agent-create-short-name-error">{errors.name}</FieldError>
        </Field>
        <AgentCreateOrganizeSection draft={draft} errors={errors} onDraftChange={onDraftChange} />
        <AgentCreatePermissionsSection draft={draft} onDraftChange={onDraftChange} />
        <AgentCreateRuntimeDetailsSection draft={draft} onDraftChange={onDraftChange} />
        <AgentCreateToolsSection draft={draft} errors={errors} onDraftChange={onDraftChange} />
      </div>
    </Disclosure>
  );
}
