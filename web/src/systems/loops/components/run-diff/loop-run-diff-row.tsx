import { ArrowRight, Gavel, Minus, Pencil, RotateCcw, type LucideIcon } from "lucide-react";

import { Pill } from "@compozy/ui";

import type { LoopDiffChange } from "../../lib/loop-request-vocabulary";
import type { LoopDiffRowView, LoopDiffValueView } from "../../lib/loop-run-diff-model";

export interface LoopRunDiffRowProps {
  row: LoopDiffRowView;
}

interface LoopRunDiffValueProps {
  value: LoopDiffValueView;
}

type LoopDiffSide = "base" | "against";

const CHANGE_ICON: Record<LoopDiffChange, LucideIcon> = {
  changed: Pencil,
  rerun: RotateCcw,
  skipped: Minus,
  carried: ArrowRight,
  verdict: Gavel,
};

const SIDE_LABEL: Record<LoopDiffSide, string> = { base: "Base", against: "Against" };

const SIDES: readonly LoopDiffSide[] = ["base", "against"];

function valueText(value: LoopDiffValueView): string {
  if (value.isSummarized) return value.summary;
  if (value.isAbsent) return "absent";
  return value.text === "" ? "—" : value.text;
}

export function LoopRunDiffValue({ value }: LoopRunDiffValueProps) {
  return (
    <span className="font-mono text-mono-id tabular-nums break-all text-subtle">
      {valueText(value)}
    </span>
  );
}

function LoopRunDiffSummary({ row }: LoopRunDiffRowProps) {
  return (
    <div
      className="mt-1.5 flex flex-col gap-1.5 rounded-sm border border-line-soft bg-input-fill px-2.5 py-2"
      data-testid="loop-diff-value-summary"
    >
      {SIDES.map(side => (
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1" key={side}>
          <span className="font-mono text-mono-id text-faint">{SIDE_LABEL[side]}</span>
          <LoopRunDiffValue value={row[side]} />
        </div>
      ))}
    </div>
  );
}

function LoopRunDiffInline({ row }: { row: LoopDiffRowView }) {
  const base = valueText(row.base);
  const against = valueText(row.against);
  if (base === against) {
    return <p className="mt-1 text-form-hint leading-relaxed break-words text-muted">{against}</p>;
  }
  return (
    <p className="mt-1 text-form-hint leading-relaxed break-words text-muted">
      <span className={row.base.isAbsent ? "text-subtle" : "text-danger line-through"}>{base}</span>
      <ArrowRight aria-hidden="true" className="mx-1.5 inline size-3 align-[-1px] text-faint" />
      <span className="text-fg">{against}</span>
    </p>
  );
}

export function LoopRunDiffRow({ row }: LoopRunDiffRowProps) {
  const Glyph = CHANGE_ICON[row.change];
  const summarized = row.base.isSummarized || row.against.isSummarized;
  const micro = [row.itemIndex === null ? null : `item ${row.itemIndex}`, row.cause || null]
    .filter(Boolean)
    .join(" · ");
  return (
    <li
      className="flex items-start gap-2.5 border-t border-line-soft px-3 py-2.5 first:border-t-0 hover:bg-row-hover"
      data-change={row.change}
      data-testid={`loop-diff-row-${row.nodeId}`}
    >
      <Glyph aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
      <div className="min-w-0 flex-1">
        <p className="text-small-body font-medium text-fg-strong">{row.nodeId}</p>
        {summarized ? <LoopRunDiffSummary row={row} /> : <LoopRunDiffInline row={row} />}
        {micro ? (
          <p className="mt-1 font-mono text-mono-id tabular-nums text-faint">{micro}</p>
        ) : null}
      </div>
      <Pill
        className="mt-0.5 shrink-0"
        data-testid="loop-diff-row-change"
        size="sm"
        tone={row.tone}
      >
        {row.changeLabel}
      </Pill>
    </li>
  );
}
