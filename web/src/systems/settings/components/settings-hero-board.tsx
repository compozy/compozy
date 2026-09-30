import type { ReactNode } from "react";

import { cn, Pill, type PillTone } from "@compozy/ui";

export interface SettingsHeroStat {
  key: string;
  label: ReactNode;
  value: ReactNode;
  /** Quiet second line under the value (e.g. "1,231 received · 27 rejected"). */
  sub?: ReactNode;
}

export interface SettingsHeroBoardProps {
  /** State sentence next to the dot, e.g. "Automation running" / "Mesh ready". */
  state: ReactNode;
  tone: PillTone;
  /** Pulse the state dot while the engine is live. */
  pulse?: boolean;
  /** Optional status pill on the right edge. Omit it when the state sentence already says it. */
  pill?: ReactNode;
  /** Optional live line after the state, e.g. "Next run in 12 min". */
  sub?: ReactNode;
  /** Optional stat cells; omit them when another row on the page already owns the counts. */
  stats?: ReadonlyArray<SettingsHeroStat>;
  "data-testid"?: string;
}

/**
 * Page-top status board (prototype `.board` / `.mesh`): live state line with
 * a pulse dot, optionally over a status pill and big stat cells. Values must come
 * from runtime data — the board renders state, never invents it.
 */
export function SettingsHeroBoard({
  state,
  tone,
  pulse = false,
  pill,
  sub,
  stats = [],
  "data-testid": testId,
}: SettingsHeroBoardProps) {
  return (
    <section
      className="flex flex-col gap-4 rounded-lg bg-canvas shadow-card p-4"
      data-testid={testId ?? "settings-hero-board"}
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <Pill.Dot pulse={pulse} tone={tone} />
        <span className="min-w-0 truncate text-ws-name font-semibold text-fg">{state}</span>
        {sub ? <span className="text-form-label tabular-nums text-subtle">{sub}</span> : null}
        {pill ? (
          <span className="ml-auto shrink-0">
            <Pill tone={tone}>{pill}</Pill>
          </span>
        ) : null}
      </div>
      {stats.length > 0 ? (
        <div
          className={cn(
            "grid gap-4 border-t border-line-soft pt-3.5",
            stats.length >= 3 ? "grid-cols-3" : "grid-cols-2"
          )}
        >
          {stats.map(stat => (
            <div className="flex min-w-0 flex-col gap-0.5" key={stat.key}>
              <span className="truncate text-metric-value font-semibold tabular-nums text-fg">
                {stat.value}
              </span>
              <span className="truncate text-form-label text-muted">{stat.label}</span>
              {stat.sub ? (
                <span className="truncate text-form-hint text-subtle">{stat.sub}</span>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
