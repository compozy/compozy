import { ArrowRight, ChevronDown, FolderGit2 } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldError,
  FieldLabel,
  Icon,
  Input,
  MonoId,
  Spinner,
} from "@compozy/ui";

import type { WorktreeCreateDialogModel } from "../hooks/use-worktree-create-dialog";
import { WorktreeCreateAdvancedFields } from "./worktree-create-advanced-fields";
import { WorktreePath } from "./worktree-path";

export interface WorktreeCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceName: string;
  model: WorktreeCreateDialogModel;
  /** Jump to the worktree already holding a refused branch (US-002 EC-2). */
  onSelectHoldingWorktree?: (worktreeId: string) => void;
}

export function WorktreeCreateDialog({
  open,
  onOpenChange,
  workspaceName,
  model,
  onSelectHoldingWorktree,
}: WorktreeCreateDialogProps) {
  const [branchPickerOpen, setBranchPickerOpen] = useState(false);
  const { draft, setDraft, fieldError, refusal, isSubmitting, heldByWorktree } = model;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("sm")}`}
        data-testid="worktree-create-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          icon={FolderGit2}
          eyebrow={`Workspace · ${workspaceName}`}
          title="New worktree"
          description={`A separate checkout of ${workspaceName} on its own branch and directory.`}
          // Closing never aborts the create: the daemon materializes in the
          // background and the row is already pending in every list.
          onClose={() => onOpenChange(false)}
        />

        <form
          className="contents"
          onSubmit={event => {
            event.preventDefault();
            model.submit();
          }}
        >
          <EntityDialogBody data-testid="worktree-create-dialog-body">
            <Field>
              <FieldLabel htmlFor="worktree-name">Name</FieldLabel>
              <Input
                id="worktree-name"
                data-testid="worktree-create-name"
                value={draft.name}
                disabled={isSubmitting}
                // The generated suggestion lives in the placeholder only —
                // prefilling it would claim the user chose this name.
                placeholder={model.generatedName}
                aria-invalid={fieldError === "name" || undefined}
                onChange={event => setDraft({ name: event.target.value })}
              />
              {fieldError === "name" ? (
                <FieldError data-testid="worktree-create-name-error">{refusal?.message}</FieldError>
              ) : null}
              {draft.name.trim() === "" ? (
                <p className="text-form-hint text-subtle">
                  Leave empty to accept {model.generatedName}.
                </p>
              ) : null}
            </Field>

            <Collapsible
              open={model.advancedOpen}
              onOpenChange={model.setAdvancedOpen}
              className="mt-4"
            >
              <CollapsibleTrigger
                data-testid="worktree-create-advanced-toggle"
                disabled={isSubmitting}
                className="inline-flex items-center gap-1.5 text-form-label text-subtle hover:text-fg"
              >
                <Icon as={ChevronDown} size="sm" />
                Advanced
              </CollapsibleTrigger>
              <CollapsibleContent className="pt-4">
                <WorktreeCreateAdvancedFields
                  model={model}
                  branchPickerOpen={branchPickerOpen}
                  onBranchPickerOpenChange={setBranchPickerOpen}
                />
              </CollapsibleContent>
            </Collapsible>

            {model.pendingWorktree ? (
              <div
                data-testid="worktree-create-pending"
                className="mt-4 flex items-center gap-3 rounded-md border border-line bg-canvas-soft p-3"
              >
                <Spinner className="size-3.5 shrink-0 text-warning" />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-small-body text-fg-strong">
                    {model.pendingWorktree.name}
                  </span>
                  <span className="font-mono text-micro text-faint">
                    {model.pendingWorktree.pending_phase
                      ? `${model.pendingWorktree.pending_phase}…`
                      : "materializing…"}
                  </span>
                </span>
                {/* Cancels the accepted creation daemon-side and unwinds its
                    artifacts — closing the dialog would leave it running. */}
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  disabled={model.isCancelling}
                  data-testid="worktree-create-cancel-creation"
                  onClick={model.cancelCreate}
                >
                  Cancel creation
                </Button>
              </div>
            ) : null}

            {model.cancelError ? (
              <p
                className="mt-2 text-form-hint text-danger"
                data-testid="worktree-create-cancel-error"
              >
                {model.cancelError}
              </p>
            ) : null}

            {model.materializationError ? (
              <p className="mt-2 text-form-hint text-danger" data-testid="worktree-create-error">
                {model.materializationError}
              </p>
            ) : null}

            {heldByWorktree && onSelectHoldingWorktree ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="mt-4"
                data-testid="worktree-create-select-holder"
                onClick={() => {
                  onSelectHoldingWorktree(heldByWorktree.id);
                  onOpenChange(false);
                }}
              >
                <Icon as={ArrowRight} size="sm" />
                Select that worktree instead
              </Button>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            // Cancel stays live while submitting: dismissing the dialog is not
            // an abort, so disabling it would imply a rollback that never runs.
            cancelDisabled={false}
            cancelTestId="worktree-create-cancel"
            hint={<WorktreeCreatePreviewLine model={model} />}
            hintTestId="worktree-create-preview"
            isSaving={isSubmitting}
            onCancel={() => onOpenChange(false)}
            primaryDisabled={!model.canSubmit}
            primaryLabel={isSubmitting ? "Creating…" : "Create worktree"}
            primaryTestId="worktree-create-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

/**
 * `branch → path`. The path is omitted when the placement root is unknown,
 * because the root is operator-configurable and guessing it would preview a
 * directory the daemon will not create.
 */
function WorktreeCreatePreviewLine({ model }: { model: WorktreeCreateDialogModel }) {
  // A refused existing-branch pick leaves no valid derivation to show.
  if (!model.preview || model.fieldError === "branch") return null;

  return (
    <span className="flex min-w-0 w-full items-center gap-1.5 overflow-hidden">
      <MonoId className="shrink-0" value={model.preview.branch} preserveCase size="sm" />
      {model.preview.path ? (
        <>
          <Icon as={ArrowRight} size="xs" className="shrink-0 text-faint" />
          <WorktreePath className="min-w-0 flex-1" path={model.preview.path} />
        </>
      ) : null}
      {model.isSubmitting ? <Spinner className="size-3" /> : null}
    </span>
  );
}
