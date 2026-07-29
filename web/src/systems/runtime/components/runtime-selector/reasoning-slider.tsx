import { type ReasoningEffort } from "@/lib/api-contract";
import { cn } from "@compozy/ui";
import { useRef } from "react";

import { reasoningEffortLabel } from "./types";
import {
  REASONING_SLIDER_THUMB_PX,
  REASONING_SLIDER_TRACK_PX,
  useReasoningSlider,
} from "./use-reasoning-slider";

export interface ReasoningSliderProps {
  /** Selectable levels (already `none`-free via `resolveReasoningState`). */
  levels: ReasoningEffort[];
  /** Controlled wire value — "" means the provider default is in effect. */
  value: ReasoningEffort | "";
  /** Model default shown as the selected stop while the wire value is "". */
  defaultEffort: ReasoningEffort | "";
  modelName: string;
  /** id of the footer's visible "Reasoning" label. */
  labelId?: string;
  onSelect: (effort: ReasoningEffort) => void;
}

function stopLeft(index: number, last: number): string {
  const fraction = last > 0 ? index / last : 0;
  return `calc(${fraction} * (100% - ${REASONING_SLIDER_TRACK_PX}px) + ${REASONING_SLIDER_TRACK_PX / 2}px)`;
}

/**
 * Compact discrete reasoning selector (design ref: runtime-selector-variations.html,
 * variation A). One 24px-tall track carries the whole control — the per-stop
 * label row is gone; the current level lives in a tooltip that rides the thumb
 * (shown on hover, focus, and drag) and in `aria-valuetext`. There is no
 * "Default"/"None" stop: the model default renders pre-selected while the wire
 * value stays "", and any interaction commits an explicit canonical effort.
 * Drag the thumb, click the track, or use arrow keys on the `role="slider"`.
 */
export function ReasoningSlider({
  levels,
  value,
  defaultEffort,
  modelName,
  labelId,
  onSelect,
}: ReasoningSliderProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const slider = useReasoningSlider({ levels, value, defaultEffort, onSelect });
  const tipLabel =
    slider.markedIndex < 0
      ? "Provider default"
      : reasoningEffortLabel(slider.stops[slider.valueNow]);
  const thumbCenter = `calc(${slider.p} * (100% - ${REASONING_SLIDER_TRACK_PX}px) + ${REASONING_SLIDER_TRACK_PX / 2}px)`;

  return (
    <div
      className="relative min-w-0 flex-1 touch-none select-none"
      data-testid="runtime-selector-reasoning-slider"
      data-unset={slider.unset ? "true" : "false"}
      data-dragging={slider.dragging ? "true" : "false"}
    >
      <div
        ref={trackRef}
        role="slider"
        tabIndex={0}
        aria-orientation="horizontal"
        aria-labelledby={labelId}
        aria-label={labelId ? undefined : `Reasoning effort for ${modelName}`}
        aria-valuemin={0}
        aria-valuemax={slider.last}
        aria-valuenow={slider.valueNow}
        aria-valuetext={slider.valueText}
        data-testid="runtime-selector-reasoning-track"
        className="group/track relative h-6 cursor-pointer rounded-md outline-none focus-visible:shadow-focus-ring"
        onPointerDown={slider.handlePointerDown}
        onPointerMove={slider.handlePointerMove}
        onPointerUp={event => {
          slider.handlePointerUp(event);
          trackRef.current?.focus();
        }}
        onPointerCancel={slider.handlePointerCancel}
        onKeyDown={slider.handleKeyDown}
      >
        <div
          aria-hidden="true"
          className="absolute inset-x-0 top-1/2 h-4 -translate-y-1/2 rounded-full bg-canvas-soft ring-1 ring-line-soft ring-inset"
        >
          <div
            className={cn(
              "absolute inset-y-0.5 left-0.5 rounded-full transition-[width] duration-300 ease-spring motion-reduce:transition-none",
              slider.unset && !slider.dragging ? "bg-btn-default-fill" : "bg-accent",
              slider.dragging && "transition-none"
            )}
            style={{
              width: `calc(${slider.p} * (100% - ${REASONING_SLIDER_TRACK_PX}px) + ${REASONING_SLIDER_TRACK_PX - 4}px)`,
            }}
          />
          {slider.stops.map((effort, index) => (
            <span
              key={effort}
              className="absolute top-1/2 size-[3px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-faint"
              style={{ left: stopLeft(index, slider.last) }}
            />
          ))}
          <div
            className={cn(
              "absolute top-1/2 size-5 -translate-y-1/2 rounded-full bg-fg-strong ring-2 ring-canvas transition-[left] duration-300 ease-spring motion-reduce:transition-none",
              slider.dragging && "scale-105 transition-none"
            )}
            style={{
              left: `calc(${slider.p} * (100% - ${REASONING_SLIDER_TRACK_PX}px) - ${(REASONING_SLIDER_THUMB_PX - REASONING_SLIDER_TRACK_PX) / 2}px)`,
            }}
          />
        </div>
        {/* Current-level tip rides the thumb; aria-valuetext is the AT channel. */}
        <span
          aria-hidden="true"
          data-testid="runtime-selector-reasoning-tip"
          className={cn(
            "pointer-events-none absolute bottom-[calc(100%+3px)] -translate-x-1/2 rounded-xs bg-elevated px-1.5 py-px font-mono text-micro whitespace-nowrap text-fg ring-1 ring-line-strong",
            "opacity-0 group-hover/track:opacity-100 group-focus-visible/track:opacity-100",
            // Fade fast, glide with the thumb's spring; while dragging the tip
            // tracks the pointer directly so `left` must not transition.
            slider.dragging
              ? "opacity-100 [transition:opacity_var(--duration-fast)_ease-out] motion-reduce:transition-none"
              : "[transition:opacity_var(--duration-fast)_ease-out,left_300ms_var(--ease-spring)] motion-reduce:transition-none"
          )}
          style={{ left: thumbCenter }}
        >
          {tipLabel}
        </span>
      </div>
    </div>
  );
}
