import type { ReactNode } from "react";
import { PlugZap, Search } from "lucide-react";

import { LaneTabs, Pill, PillDot } from "@compozy/ui";

import { LoopSection } from "../../loop-section";

export type LoopInspectLane = "graph" | "nodes" | "generations" | "events";

interface LoopRunInspectRegisterProps {
  lane: LoopInspectLane;
  onLaneChange: (lane: LoopInspectLane) => void;
  nodeCount: number;
  generationCount: number;
  eventCount: number;
  /** True while the run is live and the stream is attached. */
  isLive: boolean;
  /** True when the stream dropped; the lane keeps its last reconciled read. */
  isReconnecting: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
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
  nodeCount,
  generationCount,
  eventCount,
  isLive,
  isReconnecting,
  open,
  onOpenChange,
  children,
  foot,
}: LoopRunInspectRegisterProps) {
  const gist = [
    "graph",
    `${nodeCount} ${nodeCount === 1 ? "step" : "steps"}`,
    `${generationCount} ${generationCount === 1 ? "round" : "rounds"}`,
    `${eventCount} events`,
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
            { value: "nodes", label: "Nodes", count: nodeCount, testId: "loop-lane-nodes" },
            {
              value: "generations",
              label: "Generations",
              count: generationCount,
              testId: "loop-lane-generations",
            },
            { value: "events", label: "Events", count: eventCount, testId: "loop-lane-events" },
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
