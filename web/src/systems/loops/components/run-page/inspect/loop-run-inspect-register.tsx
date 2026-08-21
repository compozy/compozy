import type { ReactNode } from "react";
import { PlugZap, Search } from "lucide-react";

import { LaneTabs, Pill, PillDot } from "@compozy/ui";

import type { LoopRosterReach } from "../../../lib/loop-run-registers-view";
import { LoopSection } from "../../loop-section";

export type LoopInspectLane = "graph" | "nodes" | "generations" | "events";

interface LoopRunInspectRegisterProps {
  lane: LoopInspectLane;
  onLaneChange: (lane: LoopInspectLane) => void;
  /**
   * Rows and entries actually read, not the run's totals — the roster and the
   * timeline are both paged, so neither number can claim to be a denominator.
   * `reach` is what says whether the roster number is the whole story.
   */
  loadedNodeCount: number;
  loadedEventCount: number;
  /** Generations arrive whole with the run detail, so this one is a total. */
  generationCount: number;
  /** How much of the roster these counts were drawn from. */
  reach: LoopRosterReach;
  /** True while the run is live and the stream is attached. */
  isLive: boolean;
  /** True when the stream dropped; the lane keeps its last reconciled read. */
  isReconnecting: boolean;
  /**
   * Required, not optional: the page owns whether Inspect is open, and an
   * uncontrolled fallback would let the register's disclosure drift away from
   * the page state that decides which node the reads are about.
   */
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
  /** The lane's own foot: what it centred on, how old the read is. */
  foot?: ReactNode;
}

/**
 * The operator register: one disclosure, four lanes, one read model.
 *
 * There is no simple/advanced toggle — a "simple mode" that is the cockpit minus
 * panels is two artifacts that drift. This is depth, one step down, over exactly
 * the reads the default register above is already showing.
 *
 * Closed, its gist still says what is inside, so nobody has to open it to find
 * out whether opening it is worth the click.
 */
export function LoopRunInspectRegister({
  lane,
  onLaneChange,
  loadedNodeCount,
  generationCount,
  loadedEventCount,
  reach,
  isLive,
  isReconnecting,
  open,
  onOpenChange,
  children,
  foot,
}: LoopRunInspectRegisterProps) {
  // The gist is the only line a reader sees before deciding whether to open
  // this, so it is where the loaded counts have to admit what they are. A
  // truncated roster reads `200+ steps`; one still arriving says so in words.
  // The lane tabs keep the bare numbers — they label what is in the lane below.
  const steps = reach.isTruncated
    ? `${loadedNodeCount}+ steps`
    : reach.isComplete
      ? `${loadedNodeCount} ${loadedNodeCount === 1 ? "step" : "steps"}`
      : `reading steps…`;
  const gist = [
    "graph",
    steps,
    `${generationCount} ${generationCount === 1 ? "round" : "rounds"}`,
    `${loadedEventCount} events`,
  ].join(" · ");
  return (
    <LoopSection
      className="mb-0"
      data-testid="loop-run-inspect"
      defaultOpen={false}
      gist={gist}
      icon={<Search aria-hidden="true" />}
      onOpenChange={onOpenChange}
      open={open}
      right={
        // Stale state is never painted as live. When the stream drops the badge
        // leaves with it and a reconnecting chip takes its place, so the lane can
        // keep showing the last reconciled read without pretending it is current.
        isReconnecting ? (
          <Pill data-testid="loop-run-inspect-reconnecting" tone="warning">
            <PlugZap aria-hidden="true" className="size-3" />
            reconnecting
          </Pill>
        ) : isLive ? (
          <Pill data-testid="loop-run-inspect-live" tone="accent">
            <PillDot pulse tone="accent" />
            Live
          </Pill>
        ) : null
      }
      title="Inspect"
    >
      <div
        className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
        data-testid="loop-run-inspect-panel"
      >
        <LaneTabs
          ariaLabel="Operator register"
          items={[
            { value: "graph", label: "Graph", testId: "loop-lane-graph" },
            { value: "nodes", label: "Nodes", count: loadedNodeCount, testId: "loop-lane-nodes" },
            {
              value: "generations",
              label: "Generations",
              count: generationCount,
              testId: "loop-lane-generations",
            },
            {
              value: "events",
              label: "Events",
              count: loadedEventCount,
              testId: "loop-lane-events",
            },
          ]}
          listClassName="border-b border-line-soft px-2"
          onChange={onLaneChange}
          value={lane}
        />
        <div data-lane={lane} data-testid={`loop-run-inspect-lane-${lane}`}>
          {children}
        </div>
        {foot ? (
          <div
            className="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-line-soft px-4 py-2.5 text-form-hint text-subtle"
            data-testid="loop-run-inspect-foot"
          >
            {foot}
          </div>
        ) : null}
      </div>
    </LoopSection>
  );
}
