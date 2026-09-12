import { Empty, Eyebrow, PillDot, StackedProgress, StatusBreakdown } from "@compozy/ui";
import { Gauge } from "lucide-react";
import type { SessionContextView } from "../lib/session-context";
import {
  formatContextPercent,
  formatContextTokens,
  formatContextTurn,
} from "../lib/context-format";
import { SessionContextStateChip } from "./session-context-control";

export function SessionContextMeterSection({ context }: { context: SessionContextView }) {
  const { display, used, size } = context;
  return (
    <section className="flex flex-col gap-3" data-testid="session-context-meter">
      {used == null ? (
        <div className="flex items-center justify-between gap-2">
          <Eyebrow>Context window</Eyebrow>
          <SessionContextStateChip context={context} />
        </div>
      ) : null}
      {used != null ? (
        <>
          <div className="flex items-center gap-2 tabular-nums">
            <span
              className={context.warning ? "text-kpi-compact text-warning" : "text-kpi-compact"}
              style={{ fontWeight: "var(--font-weight-display)" }}
            >
              {context.ratio != null
                ? formatContextPercent(context.ratio)
                : formatContextTokens(used)}
            </span>
            <span className="font-mono text-mono-id text-muted">
              {context.ratio != null ? formatContextTokens(used) : "used"}
              {size != null ? ` / ${formatContextTokens(size)}` : ""}
            </span>
            <div className="ml-auto shrink-0">
              <SessionContextStateChip context={context} />
            </div>
          </div>
          {display ? (
            <>
              <div className="relative py-1">
                <StackedProgress
                  className={
                    context.injected?.stale ? "[&_[data-tone=accent]]:bg-accent-dim" : undefined
                  }
                  ariaLabel={`Context window: ${formatContextTokens(used)} of ${formatContextTokens(display.total)} used`}
                  total={display.total}
                  segments={[
                    { value: display.compozy, tone: "accent", label: "CompozyOS context" },
                    { value: display.agent, tone: "neutral", label: "Agent & conversation" },
                  ]}
                />
                {context.pressure_threshold != null && context.size_source === "agent" ? (
                  <span
                    aria-hidden="true"
                    className="absolute inset-y-0 w-px bg-warning"
                    style={{
                      left: `${Math.min(1, Math.max(0, context.pressure_threshold)) * 100}%`,
                    }}
                  />
                ) : null}
              </div>
              <StatusBreakdown
                total={display.total}
                items={[
                  ...(context.injected
                    ? [
                        {
                          label: "CompozyOS context ≈",
                          swatch: <PillDot tone="accent" size="md" />,
                          value: context.injected.tokens,
                          formattedValue: formatContextTokens(context.injected.tokens),
                          tone: "accent" as const,
                          showBar: false,
                        },
                      ]
                    : []),
                  {
                    label: "Agent & conversation",
                    swatch: <PillDot tone="neutral" size="md" />,
                    value: Math.max(0, used - (context.injected?.tokens ?? 0)),
                    formattedValue: formatContextTokens(
                      Math.max(0, used - (context.injected?.tokens ?? 0))
                    ),
                    tone: "neutral",
                    showBar: false,
                  },
                  {
                    label: "Free",
                    swatch: <PillDot color="var(--color-canvas-tint)" size="md" />,
                    value: display.free,
                    formattedValue: formatContextTokens(display.free),
                    tone: "neutral",
                    showBar: false,
                  },
                ]}
              />
            </>
          ) : null}
          {context.estimateExceedsReported ? (
            <p className="text-eyebrow text-warning">estimate exceeds reported</p>
          ) : null}
          {context.state === "unavailable" ? (
            <p className="text-eyebrow text-muted">Usage unavailable</p>
          ) : null}
          {context.reported_turn_id ||
          (context.pressure_threshold != null && context.size_source === "agent") ? (
            <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-eyebrow text-subtle">
              {context.reported_turn_id ? (
                <span>
                  as of turn {formatContextTurn(context.reported_turn_id)}
                  {context.size_source === "catalog" ? " · window from model catalog" : ""}
                </span>
              ) : null}
              {context.pressure_threshold != null && context.size_source === "agent" ? (
                <span className={context.warning ? "text-warning" : undefined}>
                  Compaction runs at {formatContextPercent(context.pressure_threshold)}
                </span>
              ) : null}
            </p>
          ) : null}
        </>
      ) : (
        <Empty
          icon={Gauge}
          title={
            context.loading
              ? "Loading context"
              : context.state === "unavailable"
                ? "Usage unavailable"
                : "No context report yet"
          }
          description={
            context.state === "unavailable" || context.loading
              ? undefined
              : "The meter fills once the agent reports its first turn."
          }
        />
      )}
    </section>
  );
}
