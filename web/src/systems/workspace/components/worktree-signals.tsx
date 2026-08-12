import { cn } from "@compozy/ui";

import {
  WorktreeAgentSignal,
  WorktreeAheadBehindSignal,
  WorktreeDirtySignal,
  WorktreeMergedSignal,
  WorktreeOriginSignal,
  WorktreeSetupFlagSignal,
  WorktreeStaleSignal,
} from "./worktree-signal-parts";

export interface WorktreeSignalsProps extends Omit<React.ComponentProps<"span">, "children"> {
  dirty: boolean | null;
  /** Status-only facts. List payloads do not carry them, so they stay optional. */
  insertions?: number | null;
  deletions?: number | null;
  changedFiles?: number | null;
  ahead: number | null;
  behind: number | null;
  agentActivity: string;
  agentTitle?: string;
  origin: string;
  setupState: string;
  setupError?: string;
  merged?: boolean | null;
  /** Relative stamp for remote-derived values; absent means fresh-or-unknown. */
  staleLabel?: string | null;
  /**
   * Nest density: at most one trailing signal, companion badges suppressed and
   * the setup flag reduced to its icon (locked nest density rule).
   */
  density?: "full" | "nest";
}

const NEST_SIGNAL_BUDGET = 1;
const FULL_SIGNAL_BUDGET = 2;

/**
 * Composes the row signal lane under a hard budget so a busy worktree cannot
 * crowd out its own name. Order is priority order: the most action-relevant
 * fact survives truncation.
 */
export function WorktreeSignals({
  dirty,
  insertions,
  deletions,
  changedFiles,
  ahead,
  behind,
  agentActivity,
  agentTitle,
  origin,
  setupState,
  setupError,
  merged,
  staleLabel,
  density = "full",
  className,
  ...props
}: WorktreeSignalsProps) {
  const isNest = density === "nest";

  // Visibility is decided here rather than by invoking the child components, so
  // the budget never depends on calling a component outside React's render.
  const candidates = [
    {
      key: "agent",
      visible: agentActivity === "running" || agentActivity === "awaiting_input",
      node: <WorktreeAgentSignal activity={agentActivity} title={agentTitle} showLabel={!isNest} />,
    },
    {
      key: "dirty",
      visible: dirty === true,
      node: (
        <WorktreeDirtySignal
          dirty={dirty}
          insertions={insertions}
          deletions={deletions}
          changedFiles={isNest ? null : changedFiles}
        />
      ),
    },
    {
      key: "ahead-behind",
      visible:
        (typeof ahead === "number" && ahead > 0) || (typeof behind === "number" && behind > 0),
      node: <WorktreeAheadBehindSignal ahead={ahead} behind={behind} />,
    },
    { key: "merged", visible: merged === true, node: <WorktreeMergedSignal merged={merged} /> },
  ];

  const budget = isNest ? NEST_SIGNAL_BUDGET : FULL_SIGNAL_BUDGET;
  const visible = candidates
    .filter(candidate => candidate.visible)
    .slice(0, budget)
    .map(candidate => (
      <span key={candidate.key} className="contents">
        {candidate.node}
      </span>
    ));

  return (
    <span
      data-slot="worktree-signals"
      data-density={density}
      className={cn("inline-flex min-w-0 items-center gap-2.5", className)}
      {...props}
    >
      {visible}
      <WorktreeSetupFlagSignal setupState={setupState} setupError={setupError} iconOnly={isNest} />
      {isNest ? null : <WorktreeOriginSignal origin={origin} />}
      {isNest ? null : <WorktreeStaleSignal label={staleLabel} />}
    </span>
  );
}
