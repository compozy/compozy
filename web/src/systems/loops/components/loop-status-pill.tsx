import { Pill, PillDot, type PillProps, type PillSize } from "@agh/ui";

import { loopStatusLabel, loopStatusPulse, loopStatusTone } from "../lib/loop-formatters";

export interface LoopStatusPillProps extends Omit<PillProps, "tone" | "pulse" | "children"> {
  /** Raw `loop_run.status` from the daemon; unknown values render a neutral, label-only pill. */
  status?: string | null;
  size?: PillSize;
}

/**
 * Renders a Loop run status as the design's `.pill--*` chip (tint bg + saturated
 * text + a state dot), pulsing only for live `running`/`watching` and gated by
 * `prefers-reduced-motion` inside `Pill`. Tone/pulse/label come from the single
 * canonical mapping in `loop-formatters` (truthful UI — never a coerced pill).
 */
export function LoopStatusPill({ status, size = "sm", ...props }: LoopStatusPillProps) {
  return (
    <Pill tone={loopStatusTone(status)} pulse={loopStatusPulse(status)} size={size} {...props}>
      <PillDot />
      {loopStatusLabel(status)}
    </Pill>
  );
}
