import type { LoopBudgetBar, LoopBudgetTone } from "../../lib/loop-runs-view";

const FILL_CLASS: Record<LoopBudgetTone, string> = {
  neutral: "bg-bar-fill",
  warn: "bg-warning",
  danger: "bg-danger",
};

interface LoopBudgetMiniBarProps {
  bar: LoopBudgetBar;
}

/**
 * A run's token budget mini-bar: the token count, a percent when a budget is set,
 * and a clamped fill that warns/danger-tints near the ceiling. Uncapped runs
 * (opt-in budgets off) show the count with an empty track (truthful — no fake %).
 */
export function LoopBudgetMiniBar({ bar }: LoopBudgetMiniBarProps) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5" data-testid="loop-budget-bar">
      <div className="flex justify-between gap-2 font-mono text-mono-id text-subtle">
        <span>{bar.tokensLabel}</span>
        {bar.percent !== null ? (
          <span>{bar.percent}%</span>
        ) : (
          <span className="text-faint">no cap</span>
        )}
      </div>
      <div className="h-0.75 overflow-hidden rounded-full bg-white/[0.06]">
        {bar.hasCap && bar.percent !== null ? (
          <div
            className={`h-full rounded-full ${FILL_CLASS[bar.tone]}`}
            style={{ width: `${bar.percent}%` }}
          />
        ) : null}
      </div>
    </div>
  );
}
