import type { useNavigate } from "@tanstack/react-router";
import { Pill, useTopbarSlot } from "@compozy/ui";
import { LoopRunControls, LoopRunOverflowMenu, LoopStatusPill } from "@/systems/loops";
import { loopRunsTrail } from "../loop-window-crumbs";
import type { useLoopRunDetail } from "../use-loop-run-detail";

interface LoopRunTopbarNavigation {
  runId: string;
  openLoops: () => void;
  openRuns: () => void;
  navigate: ReturnType<typeof useNavigate>;
}

/** Publishes run navigation, status, and controls into the owning window's top bar. */
export function useLoopRunTopbar(
  { page, dialogs }: ReturnType<typeof useLoopRunDetail>,
  { runId, openLoops, openRuns, navigate }: LoopRunTopbarNavigation
) {
  const loopName = page.run?.loop_name;
  useTopbarSlot({
    ...loopRunsTrail({
      level: "run",
      loopName,
      onBack: openRuns,
      openLoop:
        loopName === undefined
          ? undefined
          : () => {
              void navigate({ to: "/loops/$name", params: { name: loopName } });
            },
      openLoops,
      openRuns,
      runId,
    }),
    status: page.run ? (
      <span className="flex items-center gap-2">
        <LoopStatusPill status={page.run.status} data-testid="loop-run-status-pill" />
        {page.run.historical ? (
          <Pill data-testid="loop-run-history-pill" size="xs" tone="neutral">
            History
          </Pill>
        ) : null}
      </span>
    ) : undefined,
    actions:
      page.run && !page.run.historical ? (
        <div className="flex items-center gap-2">
          <LoopRunControls
            status={page.run.status}
            pauseRequested={page.run.pause_requested}
            pendingVerb={page.pendingRunVerb}
            onPause={page.handlePause}
            onResume={page.handleResume}
            onCancel={() => dialogs.openRunControl("cancel")}
          />
          <LoopRunOverflowMenu loopName={page.run.loop_name} />
        </div>
      ) : undefined,
  });
}
