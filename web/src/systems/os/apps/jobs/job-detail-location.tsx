import { AutomationDetailPanel, AutomationEditorDialog } from "@/systems/automation";
import { useAutomationJobDetailPage } from "../automation/use-automation-page";

export function JobDetailLocation({ jobId }: { jobId: string }) {
  const page = useAutomationJobDetailPage(jobId);

  return (
    <>
      <AutomationDetailPanel
        error={page.error}
        item={page.job}
        kind="jobs"
        onDelete={page.handleDelete}
        onEdit={page.handleEdit}
        onToggleEnabled={page.handleToggleEnabled}
        onTriggerNow={page.handleTriggerNow}
        runs={page.runs}
        runsError={page.runsError}
        runsLoading={page.runsLoading}
        state={{
          isDeleting: page.isDeleting,
          isLoading: page.isLoading,
          isTogglePending: page.isTogglePending,
          isTriggerDisabled: page.isTriggerDisabled,
          isTriggerPending: page.isTriggerPending,
        }}
      />

      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
