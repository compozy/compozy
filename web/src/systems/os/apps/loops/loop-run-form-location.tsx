import { AlertCircle, Repeat2, X } from "lucide-react";

import { Button, Empty, useTopbarSlot } from "@compozy/ui";

import { LoopUnboundState } from "./loop-unbound-state";
import { useLoopRunFormPage } from "./use-loop-run-form-page";
import { LoopRunForm, LoopRunFormSkeleton } from "@/systems/loops";

/**
 * Run-form entry for a Loop (design §4.3): the auto-generated typed input form, the
 * per-run limits fold, the live contract preview, and Dry run / Start run. On a
 * successful start the run's id routes to its live run page.
 */
export function LoopRunFormLocation({ name }: { name: string }) {
  const page = useLoopRunFormPage(name);
  const { configQuery, liveRunPending, loopQuery, openLoop, openLoops, scope, workspaceId } = page;
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
      <LoopUnboundState
        description="Open this Loop from the Loops catalog to run it."
        scope={scope}
        testId="loop-run-form-no-workspace"
      />
    );
  }
  // "No data yet", not "fetching": a window outside the live-data budget has not started its
  // reads and must not read as a missing Loop.
  if (loopQuery.isPending || configQuery.isPending || liveRunPending) {
    return <LoopRunFormSkeleton name={name} />;
  }
  if (loopQuery.error || configQuery.error || !loopQuery.data) {
    return (
      <RunFormState
        description={
          loopQuery.error?.message ?? configQuery.error?.message ?? `Loop ${name} not found.`
        }
        icon={AlertCircle}
        testId="loop-run-form-error"
        title="Couldn't open this Loop"
      />
    );
  }

  if (!configQuery.effectiveConfig) {
    return (
      <RunFormState
        description="Couldn't load this Loop's settings. Try again in a moment."
        icon={AlertCircle}
        testId="loop-run-form-effective-error"
        title="Couldn't load settings"
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
