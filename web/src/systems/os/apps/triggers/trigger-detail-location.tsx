import { useAutomationTriggerDetailPage } from "../automation/use-automation-page";
import { AutomationEditorDialog, TriggerDetailPanel } from "@/systems/automation";

export function TriggerDetailLocation({ triggerId }: { triggerId: string }) {
  const page = useAutomationTriggerDetailPage(triggerId);

  return (
    <>
      <TriggerDetailPanel
        error={page.error}
        loopWorkspaceName={page.loopWorkspaceName}
        onBack={page.handleBack}
        onDelete={page.handleDelete}
        onEdit={page.handleEdit}
        onRetryRuns={page.handleRetryRuns}
        onToggleEnabled={page.handleToggleEnabled}
        runs={page.runs}
        runsError={page.runsError}
        runsLoading={page.runsLoading}
        state={{
          isDeleting: page.isDeleting,
          isLoading: page.isLoading,
          isTogglePending: page.isTogglePending,
        }}
        trigger={page.trigger}
        workspaceName={page.workspaceName}
      />

      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
