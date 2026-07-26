import type { Dispatch, SetStateAction } from "react";

import { cn } from "@agh/ui";

import type { WindowManagerConfig } from "@/systems/os";

import { dragValueAria, useDragValue } from "../../hooks/use-drag-value";
import { WINDOW_MANAGER_RANGES } from "../../lib/window-manager-snap-geometry";

type GapKey = keyof WindowManagerConfig["gaps"];

const OUTER: ReadonlyArray<{ key: Exclude<GapKey, "inner">; label: string; axis: "x" | "y" }> = [
  { key: "top", label: "Top gap", axis: "y" },
  { key: "right", label: "Right gap", axis: "x" },
  { key: "bottom", label: "Bottom gap", axis: "y" },
  { key: "left", label: "Left gap", axis: "x" },
];

/**
 * The box model at true scale: one pixel of travel is one pixel of gap, so the
 * value never has to be typed and never has to be converted in someone's head.
 * The inner guide moves at half speed because a seam widens from its centre.
 */
export function WindowManagerGapEditor({
  draft,
  setDraft,
}: {
  draft: WindowManagerConfig;
  setDraft: Dispatch<SetStateAction<WindowManagerConfig>>;
}) {
  const gaps = draft.gaps;
  const setGap = (key: GapKey) => (value: number) =>
    setDraft(current => ({ ...current, gaps: { ...current.gaps, [key]: value } }));

  return (
    <div className="flex flex-col gap-3">
      <div
        className="relative h-54 overflow-hidden rounded-md border border-line-strong bg-canvas select-none"
        data-testid="window-manager-gap-editor"
      >
        <div
          className="absolute grid grid-cols-2 grid-rows-2"
          style={{
            top: `${gaps.top}px`,
            right: `${gaps.right}px`,
            bottom: `${gaps.bottom}px`,
            left: `${gaps.left}px`,
            gap: `${gaps.inner}px`,
          }}
        >
          {[0, 1, 2, 3].map(index => (
            <div className="rounded-xxs border border-line-strong bg-surface-glaze" key={index} />
          ))}
        </div>

        {OUTER.map(edge => (
          <GapGuide
            axis={edge.axis}
            invert={edge.key === "right" || edge.key === "bottom"}
            key={edge.key}
            label={edge.label}
            style={
              edge.axis === "y"
                ? { [edge.key]: `${gaps[edge.key] - 5}px`, left: 0, right: 0 }
                : { [edge.key]: `${gaps[edge.key] - 5}px`, top: 0, bottom: 0 }
            }
            value={gaps[edge.key]}
            onChange={setGap(edge.key)}
          />
        ))}

        <GapGuide
          axis="x"
          label="Gap between tiles"
          scale={() => 2}
          style={{
            left: "calc(50% - 5px)",
            top: `${gaps.top + 16}px`,
            bottom: `${gaps.bottom + 16}px`,
          }}
          value={gaps.inner}
          onChange={setGap("inner")}
        />
        <GapGuide
          axis="y"
          label="Gap between tiles"
          scale={() => 2}
          style={{
            top: "calc(50% - 5px)",
            left: `${gaps.left + 16}px`,
            right: `${gaps.right + 16}px`,
          }}
          value={gaps.inner}
          onChange={setGap("inner")}
        />
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-form-hint text-subtle">
        <Readout label="Between tiles" value={gaps.inner} />
        <Readout label="Top" value={gaps.top} />
        <Readout label="Right" value={gaps.right} />
        <Readout label="Bottom" value={gaps.bottom} />
        <Readout label="Left" value={gaps.left} />
        <span className="flex-1" />
        <span className="flex items-center gap-2 text-faint">
          Shown at actual size
          <span aria-hidden="true" className="block h-px w-16 bg-line-strong" />
          <span className="font-mono text-badge">64 px</span>
        </span>
      </div>
    </div>
  );
}

function GapGuide({
  axis,
  label,
  value,
  style,
  scale,
  invert,
  onChange,
}: {
  axis: "x" | "y";
  label: string;
  value: number;
  style: React.CSSProperties;
  scale?: () => number;
  invert?: boolean;
  onChange: (next: number) => void;
}) {
  const range = WINDOW_MANAGER_RANGES.gap;
  const drag = useDragValue({
    axis,
    invert,
    max: range.max,
    min: range.min,
    scale,
    value,
    onChange,
  });

  return (
    <div
      {...dragValueAria(label, value, range.min, range.max)}
      className={cn(
        "group/guide absolute z-5 flex items-center justify-center",
        axis === "x" ? "w-2.5 cursor-ew-resize" : "h-2.5 cursor-ns-resize",
        "focus-visible:outline-none focus-visible:shadow-focus-ring"
      )}
      data-dragging={drag.dragging ? "true" : undefined}
      style={style}
      onKeyDown={drag.onKeyDown}
      onPointerDown={drag.onPointerDown}
    >
      <span
        aria-hidden="true"
        className={cn(
          "rounded-pill bg-line-focus transition-colors duration-base ease-out",
          "group-hover/guide:bg-accent group-focus-visible/guide:bg-accent",
          axis === "x" ? "h-[38%] w-0.5" : "h-0.5 w-[38%]",
          drag.dragging && "bg-accent"
        )}
      />
    </div>
  );
}

function Readout({ label, value }: { label: string; value: number }) {
  return (
    <span className="inline-flex items-baseline gap-1">
      {label}
      <b className="font-mono text-form-hint font-medium tabular-nums text-fg">{value}</b>px
    </span>
  );
}
