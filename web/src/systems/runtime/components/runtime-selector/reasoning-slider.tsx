import { type ReasoningEffort } from "@/lib/api-contract";
import { cn } from "@agh/ui";
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
  /** id of the footer's visible "Reasoning effort" label. */
  labelId?: string;
  onSelect: (effort: ReasoningEffort) => void;
}

function stopLeft(index: number, last: number): string {
  const fraction = last > 0 ? index / last : 0;
  return `calc(${fraction} * (100% - ${REASONING_SLIDER_TRACK_PX}px) + ${REASONING_SLIDER_TRACK_PX / 2}px)`;
}

/**
 * Discrete reasoning range selector (design ref: provider-model-reasoning-selector.html).
 * One stop per advertised level — there is no "Default" or "None" stop: the
 * model's default level renders pre-selected while the wire value stays "",
 * and any interaction commits an explicit canonical effort. Drag the thumb,
 * click a stop label, or use arrow keys on the `role="slider"` track.
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

  return (
    <div
      className="relative touch-none select-none"
      data-testid="runtime-selector-reasoning-slider"
      data-unset={slider.unset ? "true" : "false"}
      data-dragging={slider.dragging ? "true" : "false"}
    >
      <div className="relative mb-1 h-11">
        {slider.stops.map((effort, index) => {
          const on = index === slider.markedIndex;
          const edge = index === 0 ? "start" : index === slider.last ? "end" : undefined;
          return (
            <button
              key={effort}
              type="button"
              tabIndex={-1}
              data-rz={effort}
              data-on={on ? "true" : "false"}
              data-edge={edge}
              className={cn(
                "absolute top-0 flex h-11 min-w-11 items-center justify-center rounded-xs px-1 text-badge font-medium whitespace-nowrap text-muted transition-colors hover:text-fg",
                edge === "start" && "left-0",
                edge === "end" && "right-0",
                edge === undefined && "-translate-x-1/2",
                on && "text-fg-strong"
              )}
              style={edge === undefined ? { left: stopLeft(index, slider.last) } : undefined}
              onClick={() => {
                slider.commit(index);
                trackRef.current?.focus();
              }}
            >
              {reasoningEffortLabel(effort)}
            </button>
          );
        })}
      </div>
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
        className="relative h-11 cursor-pointer rounded-md outline-none focus-visible:shadow-focus-ring"
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
          className="absolute inset-x-0 top-1/2 h-4 -translate-y-1/2 rounded-full bg-canvas ring-1 ring-line-soft ring-inset"
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
      </div>
    </div>
  );
}
