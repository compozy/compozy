import { useRef, useState, type KeyboardEvent, type PointerEvent } from "react";

import { type ReasoningEffort } from "@/lib/api-contract";

import { reasoningEffortLabel, reasoningEffortPosition } from "./types";

/** Track height / thumb diameter in px — the geometry the calc() offsets derive from. */
export const REASONING_SLIDER_TRACK_PX = 16;
export const REASONING_SLIDER_THUMB_PX = 20;
/** Pointer travel before a press becomes a drag instead of a click-to-commit. */
const DRAG_THRESHOLD_PX = 3;

/** Continuous 0..1 track position for a pointer event (padded by the round caps). */
function positionFrom(event: PointerEvent<HTMLDivElement>): number {
  const rect = event.currentTarget.getBoundingClientRect();
  const pad = rect.height / 2;
  const usable = Math.max(1, rect.width - 2 * pad);
  return Math.min(1, Math.max(0, (event.clientX - rect.left - pad) / usable));
}

export interface UseReasoningSliderArgs {
  /** Selectable levels (already `none`-free via `resolveReasoningState`). */
  levels: ReasoningEffort[];
  /** Controlled wire value — "" means the provider default is in effect. */
  value: ReasoningEffort | "";
  /** Model default shown as the selected stop while the wire value is "". */
  defaultEffort: ReasoningEffort | "";
  onSelect: (effort: ReasoningEffort) => void;
}

export interface ReasoningSliderModel {
  stops: ReasoningEffort[];
  last: number;
  /** Continuous 0..1 thumb position (live while dragging, resting otherwise). */
  p: number;
  /** Stop index the labels/aria mark as current (-1 = nothing marked). */
  markedIndex: number;
  /** No explicit value and no known default. */
  unset: boolean;
  dragging: boolean;
  valueNow: number;
  valueText: string;
  commit: (index: number) => void;
  handlePointerDown: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  handlePointerCancel: () => void;
  handleKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
}

/**
 * Behavior for the discrete reasoning range selector: stop resolution (the
 * model default renders selected while the wire value stays ""), pointer
 * drag-vs-click, and the WAI-ARIA slider keyboard contract. Every commit emits
 * an explicit canonical effort — there is no "Default"/"None" stop to return to.
 */
export function useReasoningSlider({
  levels,
  value,
  defaultEffort,
  onSelect,
}: UseReasoningSliderArgs): ReasoningSliderModel {
  const dragRef = useRef<{ startX: number; engaged: boolean } | null>(null);
  // Continuous 0..1 position while dragging; null when resting on a stop.
  const [dragP, setDragP] = useState<number | null>(null);

  const stops = [...levels].sort((a, b) => reasoningEffortPosition(a) - reasoningEffortPosition(b));
  const last = stops.length - 1;
  const active = value !== "" && stops.includes(value) ? value : defaultEffort;
  const activeIndex = active === "" ? -1 : stops.indexOf(active);
  // No explicit value and no known default: rest at the first stop with a
  // neutral fill instead of claiming a level the runtime never advertised.
  const unset = activeIndex < 0;
  const restingIndex = unset ? 0 : activeIndex;
  const p = dragP ?? (last > 0 ? restingIndex / last : 0);
  const markedIndex =
    dragP == null ? activeIndex : Math.max(0, Math.min(last, Math.round(dragP * last)));
  const dragging = dragP != null;

  const commit = (index: number) => {
    const effort = stops[Math.max(0, Math.min(last, index))];
    if (effort && effort !== value) onSelect(effort);
  };

  const handlePointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    dragRef.current = { startX: event.clientX, engaged: false };
    if (typeof event.currentTarget.setPointerCapture === "function") {
      event.currentTarget.setPointerCapture(event.pointerId);
    }
  };

  const handlePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    if (!drag.engaged) {
      if (Math.abs(event.clientX - drag.startX) < DRAG_THRESHOLD_PX) return;
      drag.engaged = true;
    }
    setDragP(positionFrom(event));
  };

  const handlePointerUp = (event: PointerEvent<HTMLDivElement>) => {
    if (!dragRef.current) return;
    dragRef.current = null;
    setDragP(null);
    commit(Math.round(positionFrom(event) * last));
  };

  const handlePointerCancel = () => {
    dragRef.current = null;
    setDragP(null);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    // From the unset state any directional key selects the first real level.
    const base = unset ? -1 : activeIndex;
    if (event.key === "ArrowRight" || event.key === "ArrowUp") {
      event.preventDefault();
      commit(base < 0 ? 0 : base + 1);
    } else if (event.key === "ArrowLeft" || event.key === "ArrowDown") {
      event.preventDefault();
      commit(base < 0 ? 0 : base - 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      commit(0);
    } else if (event.key === "End") {
      event.preventDefault();
      commit(last);
    }
  };

  const valueNow = markedIndex < 0 ? restingIndex : markedIndex;
  const valueText =
    markedIndex < 0
      ? "Provider default"
      : `${reasoningEffortLabel(stops[valueNow])}${value === "" ? " (model default)" : ""}`;

  return {
    stops,
    last,
    p,
    markedIndex,
    unset,
    dragging,
    valueNow,
    valueText,
    commit,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp,
    handlePointerCancel,
    handleKeyDown,
  };
}
