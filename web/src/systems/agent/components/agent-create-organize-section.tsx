import { FolderTree } from "lucide-react";

import {
  Field,
  FieldError,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
} from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";

export interface AgentCreateOrganizeSectionProps {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
}

/**
 * Advanced tier: where the agent files in the catalog. It changes nothing about
 * how the agent runs, so Simple leaves it out.
 */
export function AgentCreateOrganizeSection({
  draft,
  errors,
  onDraftChange,
}: AgentCreateOrganizeSectionProps) {
  return (
    <FormSection data-testid="agent-create-organize" icon={FolderTree} title="Organize">
      <Field data-invalid={Boolean(errors.categoryPath)}>
        <FieldHeader>
          <FieldLabel htmlFor="agent-create-category-path">Group</FieldLabel>
          <HelpTip label="About group">
            Use / to nest, e.g. operations/incident. It only sorts the agent list.
          </HelpTip>
        </FieldHeader>
        <Input
          aria-invalid={Boolean(errors.categoryPath)}
          data-testid="agent-create-category-path"
          id="agent-create-category-path"
          onChange={event => onDraftChange({ ...draft, categoryPath: event.target.value })}
          placeholder="operations/incident"
          value={draft.categoryPath}
        />
        <FieldError data-testid="agent-create-category-path-error">
          {errors.categoryPath}
        </FieldError>
      </Field>
    </FormSection>
  );
}
