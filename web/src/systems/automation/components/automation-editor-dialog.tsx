import type { LucideIcon } from "lucide-react";
import { CalendarClock, Zap } from "lucide-react";
import type { ReactNode } from "react";

import { Dialog, DialogContent, EntityDialogHeader, dialogShellClass } from "@agh/ui";

import type { AutomationDialogHandle } from "../lib/dialog-handle";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type { CreateAutomationJobRequest, CreateAutomationTriggerRequest } from "../types";
import type { AgentPayload } from "@/systems/agent";

import { AutomationJobForm } from "./automation-job-form";
import { AutomationTriggerForm } from "./automation-trigger-form";

type AutomationDialogEditorState =
  | {
      draft: CreateAutomationJobRequest;
      isPending: boolean;
      kind: "jobs";
      mode: "create" | "edit";
      onCancel: () => void;
      onChange: (draft: CreateAutomationJobRequest) => void;
      onSubmit: () => void;
    }
  | {
      draft: CreateAutomationTriggerRequest;
      isPending: boolean;
      kind: "triggers";
      mode: "create" | "edit";
      onCancel: () => void;
      onChange: (draft: CreateAutomationTriggerRequest) => void;
      onSubmit: () => void;
      submitError?: string | null;
    };

interface AutomationEditorDialogProps {
  activeWorkspaceId?: string | null;
  /** Agent catalog for the target selector. */
  agents?: AgentPayload[];
  /** Initial loading state for the agent target catalog. */
  agentsLoading?: boolean;
  /** Agent target catalog failure, preserved by the selector. */
  agentsError?: string | null;
  editor: AutomationDialogEditorState | null;
  handle?: AutomationDialogHandle;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

interface AutomationHeaderCopy {
  icon: LucideIcon;
  eyebrow: string;
  title: string;
  description: ReactNode;
}

function jobHeaderCopy(mode: "create" | "edit"): AutomationHeaderCopy {
  return {
    icon: CalendarClock,
    eyebrow: "Automation · Job",
    title: mode === "create" ? "Create job" : "Edit job",
    description: (
      <>
        A job runs an agent, materializes a task, or starts a Loop on a schedule.{" "}
        <b className="font-medium text-muted">Choose the target and when it should run.</b>
      </>
    ),
  };
}

function triggerHeaderCopy(mode: "create" | "edit"): AutomationHeaderCopy {
  return {
    icon: Zap,
    eyebrow: "Automation · Trigger",
    title: mode === "create" ? "Create trigger" : "Edit trigger",
    description: (
      <>
        A trigger watches for a runtime event and, when it matches, runs an agent or starts a Loop.{" "}
        <b className="font-medium text-muted">When this happens → run that.</b>
      </>
    ),
  };
}

const WIDE_CONTENT_CLASS = `text-fg grid-rows-[auto_minmax(0,1fr)] ${dialogShellClass("lg", {
  fill: true,
})}`;

export function AutomationEditorDialog({
  activeWorkspaceId,
  agents,
  agentsLoading,
  agentsError,
  editor,
  handle,
  workspaces,
}: AutomationEditorDialogProps) {
  const isEditorOpen = editor !== null;

  return (
    <Dialog
      disablePointerDismissal
      handle={handle}
      open={isEditorOpen}
      onOpenChange={open => {
        if (!open) editor?.onCancel();
      }}
    >
      {editor ? (
        <DialogContent
          unframed
          className={WIDE_CONTENT_CLASS}
          data-testid="automation-editor-dialog"
        >
          <EntityDialogHeader
            {...(editor.kind === "jobs"
              ? jobHeaderCopy(editor.mode)
              : triggerHeaderCopy(editor.mode))}
          />
          {editor.kind === "jobs" ? (
            <AutomationJobForm
              activeWorkspaceId={activeWorkspaceId}
              agents={agents}
              agentsError={agentsError}
              agentsLoading={agentsLoading}
              draft={editor.draft}
              isPending={editor.isPending}
              mode={editor.mode}
              onCancel={editor.onCancel}
              onChange={editor.onChange}
              onSubmit={editor.onSubmit}
              workspaces={workspaces}
            />
          ) : (
            <AutomationTriggerForm
              activeWorkspaceId={activeWorkspaceId}
              agents={agents}
              agentsError={agentsError}
              agentsLoading={agentsLoading}
              draft={editor.draft}
              isPending={editor.isPending}
              mode={editor.mode}
              onCancel={editor.onCancel}
              onChange={editor.onChange}
              onSubmit={editor.onSubmit}
              submitError={editor.submitError}
              workspaces={workspaces}
            />
          )}
        </DialogContent>
      ) : null}
    </Dialog>
  );
}
