import { AlertCircle, Repeat2, X } from "lucide-react";

import {
  Button,
  Empty,
  PAGE_CONTENT_GUTTER,
  Skeleton,
  SkeletonRows,
  cn,
  useTopbarSlot,
} from "@compozy/ui";

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
        description="Select a project to run this Loop."
        testId="loop-run-form-no-workspace"
        title="No project selected"
      />
    );
  }
  if (loopQuery.isLoading || configQuery.isLoading) {
    return (
      <div
        aria-busy="true"
        className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col gap-6 pt-6")}
        data-testid="loop-run-form-loading"
      >
        <Skeleton className="h-7 w-48" />
        <SkeletonRows count={3} rowClassName="border-b border-line-soft py-3" />
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
