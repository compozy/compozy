import { PillGroup, type PillGroupItem } from "@agh/ui";

import type { LoopOutcomeSegment } from "../../lib/loop-runs-view";
import type { LoopRunStatus } from "../../types";

export type LoopOutcomeValue = "all" | LoopRunStatus;

interface LoopRunsOutcomeFilterProps {
  segments: readonly LoopOutcomeSegment[];
  value: LoopOutcomeValue;
  onChange: (value: LoopOutcomeValue) => void;
}

/**
 * Data-driven outcome filter: `All` plus one segment per status present in the
 * window, each carrying its count. Paused/Queued only appear once such runs
 * exist, so the filter never advertises an outcome the workspace has none of.
 */
export function LoopRunsOutcomeFilter({ segments, value, onChange }: LoopRunsOutcomeFilterProps) {
  const items: PillGroupItem<LoopOutcomeValue>[] = segments.map(segment => ({
    value: segment.value,
    label: segment.label,
    badge: segment.count,
    testId: `loop-outcome-${segment.value}`,
  }));
  return (
    <PillGroup
      aria-label="Filter by outcome"
      data-testid="loop-runs-outcome-filter"
      items={items}
      value={value}
      onChange={onChange}
      size="sm"
    />
  );
}
