import { AlertCircle, Repeat2, X } from "lucide-react";

import { Button, Empty, Spinner, useTopbarSlot } from "@compozy/ui";

import { useLoopRunFormPage } from "./use-loop-run-form-page";
import { LoopRunForm } from "@/systems/loops";

/**
 * Run-form entry for a Loop (design §4.3): the auto-generated typed input form, the
 * per-run limits fold, the live contract preview, and Dry run / Start run. On a
 * successful start the run's id routes to its live run page.
 */
export function LoopRunFormLocation({ name }: { name: string }) {
  const page = useLoopRunFormPage(name);
  const { configQuery, loopQuery, openLoop, openLoops, workspaceId } = page;
  useTopbarSlot({
    onBack: openLoop,
    crumbs: [
      { id: "loops", label: "Loops", onSelect: openLoops },
      { id: "loop", label: name, onSelect: openLoop },
    ],
    crumb: "Run",
    // Leaving without starting is a route action, so it sits in the window chrome
    // beside the crumbs rather than as a third button next to Dry run and Start run.
    actions: (
      <Button
        data-testid="loop-run-form-close"
        onClick={openLoop}
        size="sm"
        type="button"
        variant="ghost"
      >
        <X aria-hidden="true" className="size-3.5" />
        Close
      </Button>
    ),
  });

  if (workspaceId === "") {
    return (
      <RunFormState
        description="Select a workspace to run this Loop."
        testId="loop-run-form-no-workspace"
        title="No workspace selected"
      />
    );
  }
  if (loopQuery.isLoading || configQuery.isLoading) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center"
        data-testid="loop-run-form-loading"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }
  if (loopQuery.error || configQuery.error || !loopQuery.data) {
    return (
      <RunFormState
        description={
          loopQuery.error?.message ?? configQuery.error?.message ?? `Loop ${name} not found.`
        }
        icon={AlertCircle}
        testId="loop-run-form-error"
        title="Unable to load loop"
      />
    );
  }

  if (!configQuery.effectiveConfig) {
    return (
      <RunFormState
        description="Couldn't load the effective loop configuration."
        icon={AlertCircle}
        testId="loop-run-form-effective-error"
        title="Unable to load loop configuration"
      />
    );
  }

  return (
    <LoopRunForm
      key={loopQuery.data.name}
      workspaceId={workspaceId}
      loop={loopQuery.data}
      activeRun={page.activeRun}
      effectiveConfig={configQuery.effectiveConfig}
      onRunStarted={page.openRun}
    />
  );
}

interface RunFormStateProps {
  title: string;
  description: string;
  testId: string;
  icon?: typeof Repeat2;
}

function RunFormState({ title, description, testId, icon = Repeat2 }: RunFormStateProps) {
  return (
    <div
      className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
      data-testid={testId}
    >
      <Empty className="max-w-md" description={description} icon={icon} title={title} />
    </div>
  );
}
