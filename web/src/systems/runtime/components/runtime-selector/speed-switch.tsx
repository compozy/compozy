import { Zap } from "lucide-react";
import { useState } from "react";

import { type RuntimeSpeed } from "@/lib/api-contract";
import { cn } from "@compozy/ui";

export interface RuntimeSpeedSwitchProps {
  speed: RuntimeSpeed;
  disabled?: boolean;
  onSpeedChange: (next: RuntimeSpeed) => void;
}

/**
 * ACP speed request toggle (design ref: runtime-selector-variations.html, PR
 * #267). The bolt rides the thumb across the track; enabling it replays a
 * short "zap" flourish. This is intent, not a guarantee: the daemon resolves
 * the request at prompt dispatch (`speed_resolution`), and no pre-session
 * capability signal exists — so the switch never fakes a per-model disabled
 * state. A native button carries the `switch` pattern (Enter/Space for free);
 * its visible "Fast" text is the accessible name.
 */
export function RuntimeSpeedSwitch({
  speed,
  disabled = false,
  onSpeedChange,
}: RuntimeSpeedSwitchProps) {
  const fast = speed === "fast";
  // Remount key for the bolt so toggling fast on replays the zap animation.
  const [zap, setZap] = useState(0);

  const toggle = () => {
    const next: RuntimeSpeed = fast ? "normal" : "fast";
    if (next === "fast") setZap(count => count + 1);
    onSpeedChange(next);
  };

  return (
    <button
      type="button"
      role="switch"
      aria-checked={fast}
      disabled={disabled}
      title="Runtime speed. Fast applies only when the agent supports it."
      data-testid="runtime-selector-speed"
      data-speed={speed}
      className="group/speed flex h-6 shrink-0 select-none items-center gap-2 rounded-md outline-none transition-opacity focus-visible:shadow-focus-ring disabled:cursor-not-allowed disabled:opacity-40"
      onClick={toggle}
    >
      <span
        className={cn(
          "text-badge font-medium transition-colors",
          fast ? "text-fg-strong" : "text-subtle group-hover/speed:text-fg"
        )}
      >
        Fast
      </span>
      <span
        aria-hidden="true"
        className={cn(
          "relative h-[22px] w-10 shrink-0 rounded-full transition-colors duration-200",
          fast
            ? "bg-accent-tint-strong ring-1 ring-accent-dim ring-inset"
            : "bg-canvas-soft ring-1 ring-line-soft ring-inset"
        )}
      >
        <span
          className={cn(
            "absolute top-0.5 left-0.5 grid h-[18px] w-[18px] place-items-center rounded-full ring-1 shadow-highlight transition-[translate,width,background-color,color] duration-300 ease-spring motion-reduce:transition-none",
            // iOS-style press stretch: the thumb elongates toward where it will travel.
            "group-active/speed:w-[22px]",
            fast
              ? "translate-x-[18px] bg-fg-strong text-accent ring-line-strong group-active/speed:translate-x-[14px]"
              : "translate-x-0 bg-elevated text-faint ring-line-strong"
          )}
        >
          <Zap
            key={zap}
            aria-hidden="true"
            className={cn("size-3 fill-current", zap > 0 && fast && "runtime-speed-bolt")}
          />
        </span>
      </span>
    </button>
  );
}
