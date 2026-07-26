"use client";

import { AlertCircle, ClipboardCheck } from "lucide-react";

import {
  BlockLoading,
  Dialog,
  DialogContent,
  Empty,
  EntityDialogHeader,
  dialogShellClass,
} from "@agh/ui";

import {
  TASK_DESCRIPTION,
  TaskEditorSurface,
  type TaskEditorSurfaceMode,
  type TaskEditorSurfaceProps,
} from "./task-editor-surface";

export type TaskEditorModalMode = TaskEditorSurfaceMode;

/**
 * Readiness of the record the editor binds. `new` is always `ready`; `edit`
 * resolves a task before the form can be trusted, and a half-filled form would
 * invite edits that the submit path would then overwrite.
 */
export type TaskEditorModalStatus = "ready" | "loading" | "unavailable";

export interface TaskEditorModalProps extends Omit<TaskEditorSurfaceProps, "onCancel" | "header"> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  status?: TaskEditorModalStatus;
}

/** One grid row: the child shell owns its own header/body/footer rows. */
const MODAL_CONTENT_CLASS = `text-fg grid-rows-[minmax(0,1fr)] ${dialogShellClass("md", {
  fill: true,
})}`;

const SHELL_CLASS = "flex min-h-0 flex-1 flex-col bg-canvas text-fg";

export function TaskEditorModal({
  open,
  onOpenChange,
  status = "ready",
  ...surfaceProps
}: TaskEditorModalProps) {
  const close = () => onOpenChange(false);
  const header = (
    <EntityDialogHeader
      description={TASK_DESCRIPTION}
      eyebrow="Autonomy · Task"
      icon={ClipboardCheck}
      onClose={close}
      title={surfaceProps.mode === "new" ? "Create task" : "Edit task"}
    />
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        className={MODAL_CONTENT_CLASS}
        data-mode={surfaceProps.mode}
        data-status={status}
        data-testid="task-editor-modal"
        showCloseButton={false}
      >
        {status === "ready" ? (
          <TaskEditorSurface {...surfaceProps} header={header} onCancel={close} />
        ) : (
          <section className={SHELL_CLASS} data-mode={surfaceProps.mode}>
            {header}
            {status === "loading" ? (
              <BlockLoading
                data-testid="task-editor-modal-loading"
                label="Loading task"
                size="md"
                surface="bare"
              />
            ) : (
              <div
                className="flex min-h-0 flex-1 items-center justify-center p-6"
                data-testid="task-editor-modal-unavailable"
              >
                <Empty
                  description="The task this editor binds is no longer readable. Close the editor to return to it."
                  icon={AlertCircle}
                  title="Task unavailable"
                />
              </div>
            )}
          </section>
        )}
      </DialogContent>
    </Dialog>
  );
}
