import { Bot, ChevronRight, CornerDownRight } from "lucide-react";

import {
  Icon,
  SEGMENT_PILL_TARGET,
  SegmentPill,
  StateGlyph,
  cn,
  type StateGlyphState,
} from "@compozy/ui";

import type { ChildSessionSignalState } from "../../lib/session-hierarchy";
import { isSessionRunning } from "../../lib/session-running";
import type { SessionSubagentCounts } from "../../lib/session-subagent-summary";
import {
  nextSessionThreadSegment,
  type SessionThreadSegment,
} from "../../lib/session-thread-branch";
import type { SessionPayload } from "../../types";
import { subagentChipState } from "../subagents/subagent-format";

export interface SessionListBranchToggleProps {
  session: SessionPayload;
  /** Child sessions that fold under the row (a revealed subagent never counts). */
  childCount: number;
  /** The most urgent child state while the child list is not showing. */
  childSignal: ChildSessionSignalState | null;
  subagentCounts: SessionSubagentCounts | null;
  segment: SessionThreadSegment | null;
  onChange: (segment: SessionThreadSegment | null) => void;
  /** The branch region the toggle shows and folds. */
  branchId: string;
  testIdPrefix: string;
}

interface Indicator {
  segment: SessionThreadSegment;
  icon: typeof Bot;
  text: string;
  glyph: StateGlyphState | null;
  label: string;
}

const pluralize = (count: number, noun: string) => `${count} ${noun}${count === 1 ? "" : "s"}`;

/**
 * The meta line's branch control: one discreet indicator per kind of
 * descendant (child sessions, subagents) and one chevron for the fold. With
 * one kind the indicator and chevron are a single toggle; with both, the
 * indicators become segments a rubber pill travels between, and the selected
 * segment folds the branch when pressed again.
 */
export function SessionListBranchToggle({
  session,
  childCount,
  childSignal,
  subagentCounts,
  segment,
  onChange,
  branchId,
  testIdPrefix,
}: SessionListBranchToggleProps) {
  const indicators: Indicator[] = [];
  if (childCount > 0) {
    indicators.push({
      segment: "children",
      icon: CornerDownRight,
      text: String(childCount),
      glyph: childSignal,
      label: pluralize(childCount, "child session"),
    });
  }
  if (subagentCounts !== null) {
    const chip = subagentChipState(subagentCounts, isSessionRunning(session));
    indicators.push({
      segment: "subagents",
      icon: Bot,
      text: chip?.text ?? String(subagentCounts.total),
      glyph: chip?.glyph ?? null,
      label: chip?.ariaLabel ?? pluralize(subagentCounts.total, "subagent"),
    });
  }
  const press = (clicked: SessionThreadSegment | "chevron") =>
    onChange(nextSessionThreadSegment(segment, clicked, childCount > 0));
  const toggleTestId = `${testIdPrefix}-thread-toggle-${session.id}`;
  const only = indicators.length === 1 ? indicators[0] : undefined;
  if (indicators.length === 0) return null;

  if (only) {
    return (
      <button
        type="button"
        className={cn(INDICATOR, "hover:bg-surface-2")}
        aria-expanded={segment !== null}
        aria-controls={branchId}
        aria-label={only.segment === "children" ? `Toggle ${only.label}` : only.label}
        data-slot="session-branch-toggle"
        data-segment={only.segment}
        data-state={only.glyph ?? undefined}
        data-testid={toggleTestId}
        onClick={() => press("chevron")}
      >
        <IndicatorBody indicator={only} />
        <Chevron open={segment !== null} />
      </button>
    );
  }

  return (
    <span className="inline-flex items-center" data-slot="session-branch-toggle">
      <span
        role="group"
        aria-label="Child sessions and subagents"
        className="relative inline-flex items-center"
      >
        {indicators.map(indicator => (
          <button
            key={indicator.segment}
            type="button"
            {...{ [SEGMENT_PILL_TARGET]: "" }}
            className={cn(INDICATOR, "relative z-10", segment === indicator.segment && "text-fg")}
            aria-expanded={segment === indicator.segment}
            aria-controls={branchId}
            aria-label={indicator.label}
            data-segment={indicator.segment}
            data-state={indicator.glyph ?? undefined}
            data-testid={`${testIdPrefix}-branch-${indicator.segment}-${session.id}`}
            onClick={() => press(indicator.segment)}
          >
            <IndicatorBody indicator={indicator} />
          </button>
        ))}
        {/* The row's own hover and selected plates are surface-2/selected: the pill steps above both. */}
        <SegmentPill
          index={segment === null ? null : segment === "children" ? 0 : 1}
          className="bg-line-strong"
        />
      </span>
      <button
        type="button"
        className={cn(INDICATOR, "px-1 hover:bg-surface-2")}
        aria-expanded={segment !== null}
        aria-controls={branchId}
        aria-label="Toggle child sessions and subagents"
        data-testid={toggleTestId}
        onClick={() => press("chevron")}
      >
        <Chevron open={segment !== null} />
      </button>
    </span>
  );
}

const INDICATOR =
  "inline-flex h-5 shrink-0 items-center gap-1 rounded-pill px-1.5 font-mono text-micro text-subtle tabular-nums outline-none transition-colors duration-fast ease-out hover:text-fg focus-visible:shadow-focus-ring motion-reduce:transition-none";

function IndicatorBody({ indicator }: { indicator: Indicator }) {
  return (
    <>
      <Icon as={indicator.icon} size="xs" aria-hidden="true" className="text-subtle" />
      {indicator.text}
      {indicator.glyph !== null ? <StateGlyph size="sm" state={indicator.glyph} /> : null}
    </>
  );
}

function Chevron({ open }: { open: boolean }) {
  return (
    <Icon
      as={ChevronRight}
      size="sm"
      aria-hidden="true"
      className={cn(
        "transition-transform duration-base ease-out motion-reduce:transition-none",
        open && "rotate-90"
      )}
    />
  );
}
