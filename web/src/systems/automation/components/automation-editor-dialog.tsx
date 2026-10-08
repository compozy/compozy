import { Zap } from "lucide-react";

import { Dialog, DialogContent, EntityDialogHeader, dialogShellClass } from "@compozy/ui";

import type { AutomationDialogHandle } from "../lib/dialog-handle";
import type { AutomationEditorSection, AutomationFormDraft } from "../lib/automation-form-draft";
import type { WorkspaceOption } from "../lib/trigger-preview";
import { AutomationForm } from "./automation-form/automation-form";
import type { AgentPayload } from "@/systems/agent";
import { useAggregateDestination } from "@/systems/profiles";

export interface AutomationEditorDialogState {
  draft: AutomationFormDraft;
  isPending: boolean;
  lockedLoop?: string;
  mode: "create" | "edit";
  onCancel: () => void;
  onChange: (draft: AutomationFormDraft) => void;
  onSubmit: () => void;
  section?: AutomationEditorSection;
  submitError?: string | null;
  submitErrorField?: "name" | null;
}

interface AutomationEditorDialogProps {
  activeWorkspaceId?: string | null;
  /** Agent catalog for the target selector. */
  agents?: AgentPayload[];
  /** Initial loading state for the agent target catalog. */
  agentsLoading?: boolean;
  /** Agent target catalog failure, preserved by the selector. */
  agentsError?: string | null;
  editor: AutomationEditorDialogState | null;
  handle?: AutomationDialogHandle;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

const WIDE_CONTENT_CLASS = `text-fg grid-rows-[auto_minmax(0,1fr)] ${dialogShellClass("lg", {
  fill: true,
})}`;

/** One create/edit dialog for every automation; Starts decides job vs trigger. */
export function AutomationEditorDialog({
  activeWorkspaceId,
  agents,
  agentsLoading,
  agentsError,
  editor,
  handle,
  workspaces,
}: AutomationEditorDialogProps) {
  const aggregateDestination = useAggregateDestination();
  const isEdit = editor?.mode === "edit";

  return (
    <Dialog
      disablePointerDismissal
      handle={handle}
      open={editor !== null}
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
            description={
              isEdit ? (
                "Changes apply from the next run."
              ) : (
                <>
                  Choose <b className="font-medium text-fg">when it starts</b> and{" "}
                  <b className="font-medium text-fg">what it does</b>. You can turn it off any time.
                </>
              )
            }
            eyebrow="Automation"
            icon={Zap}
            title={isEdit ? "Edit automation" : "New automation"}
          />
          <AutomationForm
            activeWorkspaceId={activeWorkspaceId}
            agents={agents}
            agentsError={agentsError}
            agentsLoading={agentsLoading}
            draft={editor.draft}
            isPending={editor.isPending}
            lockedLoop={editor.lockedLoop}
            mode={editor.mode}
            onCancel={editor.onCancel}
            onChange={editor.onChange}
            onSubmit={editor.onSubmit}
            profileDestination={isEdit ? null : aggregateDestination}
            section={editor.section}
            submitError={editor.submitError}
            submitErrorField={editor.submitErrorField}
            workspaces={workspaces}
          />
        </DialogContent>
      ) : null}
    </Dialog>
  );
}
