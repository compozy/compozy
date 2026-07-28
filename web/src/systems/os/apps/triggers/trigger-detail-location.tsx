import { AutomationDetailPanel, AutomationEditorDialog } from "@/systems/automation";
import { useAutomationTriggerDetailPage } from "../automation/use-automation-page";

export function TriggerDetailLocation({ triggerId }: { triggerId: string }) {
  const page = useAutomationTriggerDetailPage(triggerId);

  return (
    <>
      <AutomationDetailPanel
        error={page.error}
        item={page.trigger}
        kind="triggers"
        onDelete={page.handleDelete}
        onEdit={page.handleEdit}
        onToggleEnabled={page.handleToggleEnabled}
        runs={page.runs}
        runsError={page.runsError}
        runsLoading={page.runsLoading}
        state={{
          isDeleting: page.isDeleting,
          isLoading: page.isLoading,
          isTogglePending: page.isTogglePending,
          isTriggerPending: false,
        }}
      />

      <AutomationEditorDialog {...page.editorDialogProps} />
    </>
  );
}
