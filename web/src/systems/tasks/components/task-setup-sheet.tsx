import { Lock, Settings2 } from "lucide-react";
import { useState } from "react";

import {
  Button,
  ConfirmDialog,
  JsonViewer,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@agh/ui";

import type { TaskProfileEditor } from "../hooks/use-profile-editor";
import type { TaskExecutionProfile } from "../types";
import { TaskSetupForm, type TaskSetupFormProps } from "./task-setup-form";
import { TaskSetupProfileView } from "./task-setup-profile-view";

export interface TaskSetupSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: TaskExecutionProfile | null;
  profileLoading?: boolean;
  profileErrorMessage?: string | null;
  hasActiveRun: boolean;
  isSetPending?: boolean;
  isDeletePending?: boolean;
  clearOpen: boolean;
  onClearOpenChange: (open: boolean) => void;
  onClearProfile: () => Promise<void>;
  editor: TaskProfileEditor;
  runtime: TaskSetupFormProps["runtime"];
}

/**
 * Setup sheet for who works on this task and where it runs. Read view is
 * form-shaped; editing is locked while a run is active. The raw profile stays
 * one toggle away for operators while writes use the typed runtime contract.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.7
 */
export function TaskSetupSheet({
  open,
  onOpenChange,
  profile,
  profileLoading = false,
  profileErrorMessage = null,
  hasActiveRun,
  isSetPending = false,
  isDeletePending = false,
  clearOpen,
  onClearOpenChange,
  onClearProfile,
  editor,
  runtime,
}: TaskSetupSheetProps) {
  const [showJson, setShowJson] = useState(false);

  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent className="w-full sm:max-w-xl" data-testid="tasks-setup-sheet" side="right">
        <SheetHeader>
          <div className="flex items-start gap-3">
            <span
              aria-hidden="true"
              className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-tint text-accent-strong"
            >
              <Settings2 className="size-4" />
            </span>
            <div className="min-w-0">
              <span className="eyebrow text-subtle">Execution profile</span>
              <SheetTitle>Task setup</SheetTitle>
              <SheetDescription>Who works on this task and where it runs.</SheetDescription>
            </div>
          </div>
        </SheetHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          {hasActiveRun ? (
            <div
              className="mb-4 flex items-start gap-2.5 rounded-md border border-line-soft bg-canvas-soft px-3.5 py-3 text-small-body leading-relaxed text-muted"
              data-testid="tasks-setup-locked"
            >
              <Lock aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
              Editing is locked while a run is active. Cancel the current run before changing the
              setup.
            </div>
          ) : null}

          {profileLoading && !profile ? (
            <p className="text-small-body text-muted">Loading setup…</p>
          ) : profileErrorMessage && !profile ? (
            <p className="text-small-body text-danger">{profileErrorMessage}</p>
          ) : profile ? (
            <TaskSetupProfileView profile={profile} />
          ) : (
            <p className="text-small-body text-muted" data-testid="tasks-setup-empty">
              No custom setup. Runs inherit the workspace defaults for worker, model, and sandbox.
            </p>
          )}

          {editor.open && editor.value ? (
            <div className="mt-2 flex flex-col gap-2" data-testid="tasks-setup-editor">
              <TaskSetupForm onChange={editor.setValue} runtime={runtime} value={editor.value} />
              <div className="flex items-center justify-end gap-2">
                <Button
                  aria-busy={isSetPending || undefined}
                  disabled={isSetPending}
                  onClick={() => editor.setOpen(false)}
                  size="sm"
                  type="button"
                  variant="neutral"
                >
                  Cancel
                </Button>
                <Button
                  data-testid="tasks-setup-editor-save"
                  disabled={isSetPending}
                  onClick={() => void editor.submit()}
                  size="sm"
                  type="button"
                >
                  {isSetPending ? "Saving…" : "Save setup"}
                </Button>
              </div>
            </div>
          ) : showJson && profile ? (
            <div className="mt-2" data-testid="tasks-setup-json">
              <JsonViewer value={profile} />
            </div>
          ) : null}
        </div>

        <SheetFooter className="flex-row items-center justify-between gap-2 border-t border-line px-4 py-3">
          <div className="flex items-center gap-2">
            {profile && !editor.open ? (
              <Button
                data-testid="tasks-setup-toggle-json"
                onClick={() => setShowJson(current => !current)}
                size="sm"
                type="button"
                variant="ghost"
              >
                {showJson ? "Hide JSON" : "View JSON"}
              </Button>
            ) : null}
            {profile && !hasActiveRun && !editor.open ? (
              <Button
                data-testid="tasks-setup-clear"
                disabled={isDeletePending}
                onClick={() => onClearOpenChange(true)}
                size="sm"
                type="button"
                variant="ghost"
              >
                Clear setup
              </Button>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            {!hasActiveRun && !editor.open ? (
              <Button
                data-testid="tasks-setup-edit"
                onClick={() => editor.setOpen(true)}
                size="sm"
                type="button"
                variant="neutral"
              >
                {profile ? "Edit setup" : "Create setup"}
              </Button>
            ) : null}
            <Button
              data-testid="tasks-setup-close"
              onClick={() => onOpenChange(false)}
              size="sm"
              type="button"
              variant={hasActiveRun || editor.open ? "neutral" : "ghost"}
            >
              Close
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
      <ConfirmDialog
        cancelLabel="Keep setup"
        confirmButtonProps={{
          "aria-busy": isDeletePending || undefined,
          "data-testid": "tasks-setup-clear-confirm",
        }}
        confirmLabel={isDeletePending ? "Clearing…" : "Clear setup"}
        description="This removes the task-specific execution profile. Future runs inherit workspace defaults."
        isPending={isDeletePending}
        onConfirm={onClearProfile}
        onOpenChange={onClearOpenChange}
        open={clearOpen}
        title="Clear task setup?"
        tone="danger"
      />
    </Sheet>
  );
}
