import {
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Field,
  FieldError,
  FieldLabel,
  Input,
} from "@compozy/ui";

import type {
  BranchCandidate,
  WorktreeCreateDialogModel,
} from "../hooks/use-worktree-create-dialog";

export interface WorktreeCreateAdvancedFieldsProps {
  model: WorktreeCreateDialogModel;
  branchPickerOpen: boolean;
  onBranchPickerOpenChange: (open: boolean) => void;
}

/**
 * Branch, base ref, and the existing-branch picker. Branch prefill is derived
 * from the name alone — Compozy forces no prefix and exposes no prefix setting
 * (BR-15), so any namespace here is something the user typed.
 */
export function WorktreeCreateAdvancedFields({
  model,
  branchPickerOpen,
  onBranchPickerOpenChange,
}: WorktreeCreateAdvancedFieldsProps) {
  const { draft, setDraft, fieldError, refusal, isSubmitting, branchCandidates } = model;
  const free = branchCandidates.filter(candidate => candidate.heldBy === null);
  const held = branchCandidates.filter(candidate => candidate.heldBy !== null);

  return (
    <div className="flex flex-col gap-4">
      <Field>
        <FieldLabel htmlFor="worktree-branch">Branch</FieldLabel>
        <Input
          id="worktree-branch"
          data-testid="worktree-create-branch"
          value={draft.branch}
          disabled={isSubmitting}
          placeholder={model.preview?.branch ?? ""}
          aria-invalid={fieldError === "branch" || undefined}
          onChange={event => setDraft({ branch: event.target.value })}
        />
        {fieldError === "branch" ? (
          <FieldError data-testid="worktree-create-branch-error">{refusal?.message}</FieldError>
        ) : null}
      </Field>

      <Field>
        <FieldLabel htmlFor="worktree-base-ref">Base ref</FieldLabel>
        <Input
          id="worktree-base-ref"
          data-testid="worktree-create-base-ref"
          value={draft.baseRef}
          disabled={isSubmitting}
          placeholder="main"
          aria-invalid={fieldError === "baseRef" || undefined}
          onChange={event => setDraft({ baseRef: event.target.value })}
        />
        {fieldError === "baseRef" ? (
          <FieldError data-testid="worktree-create-base-ref-error">{refusal?.message}</FieldError>
        ) : null}
      </Field>

      <Field>
        <FieldLabel htmlFor="worktree-existing-branch">Use existing branch</FieldLabel>
        <CommandSelect open={branchPickerOpen} onOpenChange={onBranchPickerOpenChange}>
          <CommandSelectTrigger
            id="worktree-existing-branch"
            data-testid="worktree-create-existing-branch"
            disabled={isSubmitting}
            selected={draft.existingBranch !== ""}
          >
            {draft.existingBranch === "" ? "None" : draft.existingBranch}
          </CommandSelectTrigger>
          <CommandSelectShell
            inputPlaceholder="Search branches…"
            inputProps={{ "aria-label": "Search branches" }}
          >
            <CommandList>
              <CommandEmpty>No branches match your search.</CommandEmpty>
              <BranchGroup
                heading="Branches"
                candidates={free}
                onSelect={branch => {
                  setDraft({ existingBranch: branch });
                  onBranchPickerOpenChange(false);
                }}
              />
              {/* Held branches stay visible and badged rather than hidden: the
                  user needs to see that the branch exists and who holds it. */}
              <BranchGroup
                heading="Already in a worktree"
                candidates={held}
                onSelect={branch => {
                  setDraft({ existingBranch: branch });
                  onBranchPickerOpenChange(false);
                }}
              />
            </CommandList>
          </CommandSelectShell>
        </CommandSelect>
      </Field>
    </div>
  );
}

function BranchGroup({
  heading,
  candidates,
  onSelect,
}: {
  heading: string;
  candidates: BranchCandidate[];
  onSelect: (branch: string) => void;
}) {
  if (candidates.length === 0) return null;

  return (
    <CommandSelectGroup heading={heading}>
      {candidates.map(candidate => (
        <CommandItem
          key={candidate.branch}
          value={candidate.branch}
          onSelect={() => onSelect(candidate.branch)}
          data-testid={`worktree-create-branch-option-${candidate.branch}`}
        >
          <span className="min-w-0 flex-1 truncate font-mono text-mono-id">{candidate.branch}</span>
          {candidate.heldBy ? (
            <span className="shrink-0 text-badge text-muted">{candidate.heldBy}</span>
          ) : null}
        </CommandItem>
      ))}
    </CommandSelectGroup>
  );
}
