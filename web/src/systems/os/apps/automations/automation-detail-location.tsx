import {
  useAutomationJobDetailPage,
  useAutomationTriggerDetailPage,
} from "../automation/use-automation-page";
import {
  AutomationDetailPanel,
  AutomationEditorDialog,
  TriggerDetailPanel,
} from "@/systems/automation";

/** Detail of one automation; routes keep the daemon entity (`jobs` | `triggers`). */
export function AutomationDetailLocation({ id, kind }: { id: string; kind: "job" | "trigger" }) {
  return kind === "job" ? <JobDetail jobId={id} /> : <TriggerDetail triggerId={id} />;
}

function JobDetail({ jobId }: { jobId: string }) {
  const page = useAutomationJobDetailPage(jobId);

  return (
    <>
      <AutomationDetailPanel
        error={page.error}
        item={page.job}
        onBack={page.handleBack}
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

function TriggerDetail({ triggerId }: { triggerId: string }) {
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
