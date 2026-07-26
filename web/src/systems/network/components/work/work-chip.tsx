import { Pill, type PillTone } from "@agh/ui";

import { cn } from "@/lib/utils";

import {
  formatNetworkWorkStateLabel,
  isNetworkWorkState,
  shouldRenderNetworkWorkChip,
  type NetworkWorkState,
} from "../../lib/network-formatters";
import { formatElapsedSeconds, useElapsedSeconds } from "../../lib/use-elapsed";

export interface WorkChipProps {
  /** Network work state - silent for `submitted` / `completed` per `_design.md` §6.6. */
  state: string | null | undefined;
  /** When set + state is `working`, the chip ticks with elapsed seconds. */
  startedAt?: string | null;
  className?: string;
  onClick?: () => void;
  /** Optional `aria-label` override (e.g. "open work · working · 12s"). */
  ariaLabel?: string;
}

const STATE_TONE: Record<NetworkWorkState, PillTone | null> = {
  submitted: null,
  working: "warning",
  needs_input: "warning",
  completed: null,
  failed: "danger",
  canceled: "neutral",
};

export function WorkChip({ state, startedAt, className, onClick, ariaLabel }: WorkChipProps) {
  const elapsed = useElapsedSeconds(startedAt, {
    enabled: state === "working",
  });

  if (!shouldRenderNetworkWorkChip(state)) {
    return null;
  }
  if (!isNetworkWorkState(state)) {
    return null;
  }

  const label = formatNetworkWorkStateLabel(state);
  const elapsedText = state === "working" && startedAt ? formatElapsedSeconds(elapsed) : "";
  const text = elapsedText ? `${label} · ${elapsedText}` : label;
  const tone = STATE_TONE[state] ?? "neutral";
  const stateClassName = cn(
    state === "needs_input" && "motion-safe:animate-pulse",
    state === "canceled" && "bg-transparent text-subtle"
  );

  if (onClick) {
    return (
      <Pill
        aria-label={ariaLabel ?? text}
        className={cn(stateClassName, className)}
        data-testid="network-work-chip"
        data-state={state}
        mono
        onClick={onClick}
        render={<button type="button" />}
        size="xs"
        tone={tone}
      >
        {text}
      </Pill>
    );
  }

  return (
    <Pill
      aria-label={ariaLabel ?? text}
      className={cn(stateClassName, className)}
      data-testid="network-work-chip"
      data-state={state}
      mono
      size="xs"
      tone={tone}
    >
      {text}
    </Pill>
  );
}
