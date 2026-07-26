import { useRef, type Dispatch, type SetStateAction } from "react";

import { Popover, PopoverContent, PopoverTrigger, cn } from "@agh/ui";

import type { WindowManagerConfig } from "@/systems/os";

import { dragValueAria, useDragValue } from "../../hooks/use-drag-value";
import {
  WINDOW_MANAGER_RANGES,
  snapMapPercent,
  snapMapScale,
} from "../../lib/window-manager-snap-geometry";

type BindingKey = keyof WindowManagerConfig["bindings"];
type BindingValue = WindowManagerConfig["bindings"][BindingKey];

const BINDING_OPTIONS: ReadonlyArray<{ value: BindingValue; label: string; hint: string }> = [
  { value: "none", label: "Free", hint: "The centre snaps like any other edge." },
  { value: "reserved", label: "Reserved", hint: "Kept clear — nothing claims this centre." },
  { value: "zoom", label: "Zoom", hint: "Dropping here zooms the window to the desktop." },
];

/**
 * Where a dragged window is caught, drawn where it is caught. Each control sits
 * inside the region it governs, so nothing here needs a coordinate field: the
 * band is the band.
 */
export function WindowManagerSnapMap({
  draft,
  setDraft,
}: {
  draft: WindowManagerConfig;
  setDraft: Dispatch<SetStateAction<WindowManagerConfig>>;
}) {
  const mapRef = useRef<HTMLDivElement>(null);
  const snap = draft.snap;
  const scale = () => {
    const width = mapRef.current?.getBoundingClientRect().width ?? 0;
    return 1 / snapMapScale(width);
  };
  const setSnap = (key: "edgeBand" | "cornerReach" | "exitSlack") => (value: number) =>
    setDraft(current => ({ ...current, snap: { ...current.snap, [key]: value } }));
  const setBinding = (key: BindingKey) => (value: BindingValue) =>
    setDraft(current => ({ ...current, bindings: { ...current.bindings, [key]: value } }));

  const band = snapMapPercent(snap.edgeBand, "x");
  const bandY = snapMapPercent(snap.edgeBand, "y");
  const corner = snapMapPercent(snap.cornerReach, "x");
  const cornerY = snapMapPercent(snap.cornerReach, "y");
  const inset = snapMapPercent(snap.edgeBand + snap.exitSlack, "x");
  const insetY = snapMapPercent(snap.edgeBand + snap.exitSlack, "y");

  return (
    <div className="flex flex-col gap-3">
      <div
        className="relative h-54 overflow-hidden rounded-md border border-line-strong bg-canvas select-none"
        data-testid="window-manager-snap-map"
        ref={mapRef}
      >
        <span
          className="pointer-events-none absolute inset-y-0 left-0 bg-accent-tint"
          style={{ width: band }}
        />
        <span
          className="pointer-events-none absolute inset-y-0 right-0 bg-accent-tint"
          style={{ width: band }}
        />
        <span
          className="pointer-events-none absolute top-0 bg-accent-tint"
          style={{ left: band, right: band, height: bandY }}
        />
        <span
          className="pointer-events-none absolute bottom-0 bg-accent-tint"
          style={{ left: band, right: band, height: bandY }}
        />
        {(["left", "right"] as const).map(x =>
          (["top", "bottom"] as const).map(y => (
            <span
              className="pointer-events-none absolute bg-accent-tint-strong"
              key={`${x}-${y}`}
              style={{ [x]: 0, [y]: 0, width: corner, height: cornerY }}
            />
          ))
        )}
        <span
          className="pointer-events-none absolute rounded-xxs border border-dashed border-accent-dim"
          style={{ left: inset, right: inset, top: insetY, bottom: insetY }}
        />

        <SnapGrip
          axis="x"
          label="Edge band"
          range={WINDOW_MANAGER_RANGES.edgeBand}
          scale={scale}
          style={{ left: `calc(${band} - 5px)`, top: "calc(50% - 13px)", height: "26px" }}
          value={snap.edgeBand}
          onChange={setSnap("edgeBand")}
        />
        <SnapGrip
          axis="y"
          label="Corner reach"
          range={WINDOW_MANAGER_RANGES.cornerReach}
          scale={scale}
          style={{ top: `calc(${cornerY} - 5px)`, left: "5px", width: "26px" }}
          value={snap.cornerReach}
          onChange={setSnap("cornerReach")}
        />
        <SnapGrip
          axis="x"
          label="Exit slack"
          range={WINDOW_MANAGER_RANGES.exitSlack}
          scale={scale}
          style={{ left: `calc(${inset} - 5px)`, bottom: "12px", height: "26px" }}
          value={snap.exitSlack}
          onChange={setSnap("exitSlack")}
        />

        <CentreZone
          label="Top centre zone"
          side="top"
          value={draft.bindings.topCenter}
          onChange={setBinding("topCenter")}
        />
        <CentreZone
          label="Bottom centre zone"
          side="bottom"
          value={draft.bindings.bottomCenter}
          onChange={setBinding("bottomCenter")}
        />
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-form-hint text-subtle">
        <Readout label="Edge band" value={snap.edgeBand} />
        <Readout label="Corner reach" value={snap.cornerReach} />
        <Readout label="Exit slack" value={snap.exitSlack} />
      </div>
    </div>
  );
}

function SnapGrip({
  axis,
  label,
  value,
  range,
  scale,
  style,
  onChange,
}: {
  axis: "x" | "y";
  label: string;
  value: number;
  range: { min: number; max: number };
  scale: () => number;
  style: React.CSSProperties;
  onChange: (next: number) => void;
}) {
  const drag = useDragValue({ axis, max: range.max, min: range.min, scale, value, onChange });

  return (
    <div
      {...dragValueAria(label, value, range.min, range.max)}
      className={cn(
        "group/grip absolute z-6 flex items-center justify-center",
        axis === "x" ? "w-2.5 cursor-ew-resize" : "h-2.5 cursor-ns-resize",
        "focus-visible:outline-none focus-visible:shadow-focus-ring"
      )}
      data-dragging={drag.dragging ? "true" : undefined}
      data-testid={`window-manager-snap-grip-${label.toLowerCase().replace(/\s+/g, "-")}`}
      style={style}
      onKeyDown={drag.onKeyDown}
      onPointerDown={drag.onPointerDown}
    >
      <span
        aria-hidden="true"
        className={cn(
          "rounded-pill bg-line-focus transition-colors duration-base ease-out",
          "group-hover/grip:bg-accent group-focus-visible/grip:bg-accent",
          axis === "x" ? "h-6.5 w-0.5" : "h-0.5 w-6.5",
          drag.dragging && "bg-accent"
        )}
      />
    </div>
  );
}

function CentreZone({
  label,
  side,
  value,
  onChange,
}: {
  label: string;
  side: "top" | "bottom";
  value: BindingValue;
  onChange: (next: BindingValue) => void;
}) {
  const current = BINDING_OPTIONS.find(option => option.value === value) ?? BINDING_OPTIONS[0];

  return (
    <Popover>
      <PopoverTrigger
        aria-label={label}
        className={cn(
          "eyebrow absolute z-7 left-1/2 flex h-5 -translate-x-1/2 items-center rounded-xs border px-2",
          "border-line-strong bg-elevated text-muted transition-colors duration-base ease-out",
          "hover:bg-btn-default-hover hover:text-fg-strong",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          value === "zoom" && "border-accent-dim bg-accent-tint text-accent-strong",
          value === "reserved" && "border-info/30 bg-info-tint text-info"
        )}
        data-testid={`window-manager-centre-zone-${side}`}
        render={<button type="button" />}
        style={{ [side]: "6px" }}
      >
        {current?.label}
      </PopoverTrigger>
      <PopoverContent align="center" className="w-56 p-1" role="menu">
        {BINDING_OPTIONS.map(option => (
          <button
            aria-checked={option.value === value}
            className={cn(
              "flex w-full flex-col items-start gap-0.5 rounded-sm px-2 py-1.5 text-left",
              "text-small-body text-fg transition-colors duration-base ease-out hover:bg-row-hover",
              "focus-visible:outline-none focus-visible:shadow-focus-ring",
              option.value === value && "text-accent-strong"
            )}
            key={option.value}
            role="menuitemradio"
            type="button"
            onClick={() => onChange(option.value)}
          >
            {option.label}
            <span className="text-form-hint text-subtle">{option.hint}</span>
          </button>
        ))}
      </PopoverContent>
    </Popover>
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
