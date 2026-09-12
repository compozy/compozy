import { Fragment, type ComponentProps } from "react";
import { Clock, Gauge, Minimize2 } from "lucide-react";
import { Pill, StackedProgress, StatusBreakdown, cn } from "@compozy/ui";
import type { SessionContextView } from "../lib/session-context";
import { formatContextTokens } from "../lib/context-format";
import {
  describeSessionContextChip,
  describeSessionContextMeter,
  type SessionContextMeterView,
  type SessionContextTiersView,
} from "../lib/session-context-view";
import { SessionInspectorEmpty, SessionInspectorSection } from "./session-inspector-section";

type StatusBreakdownItem = ComponentProps<typeof StatusBreakdown>["items"][number];

/** Meter chip: reported · stale · estimated size · near compaction · unavailable (hollow). */
export function SessionContextStateChip({ context }: { context: SessionContextView }) {
  const chip = describeSessionContextChip(context);
  return (
    <Pill size="xs" form={chip.form} tone={chip.tone}>
      {chip.label}
    </Pill>
  );
}

/** Tier swatches are magnitude keys: 8px squares, never signal dots. */
function TierSwatch({ className }: { className: string }) {
  return <span aria-hidden="true" className={cn("size-2 shrink-0 rounded-[2px]", className)} />;
}

function TierLabel({ children, approx }: { children: string; approx?: boolean }) {
  return (
    <>
      <span className="truncate font-medium text-fg">{children}</span>
      {approx ? (
        <em aria-label="approximately" className="text-micro not-italic text-faint">
          ≈
        </em>
      ) : null}
    </>
  );
}

function compozyTierItem(tiers: SessionContextTiersView): StatusBreakdownItem[] {
  const { compozy } = tiers;
  if (!compozy) return [];
  const caveat = compozy.exceeds
    ? "estimate exceeds reported"
    : tiers.stale
      ? "may have been summarized"
      : undefined;
  return [
    {
      id: "compozy",
      label: <TierLabel approx>CompozyOS context</TierLabel>,
      swatch: <TierSwatch className={tiers.stale ? "bg-accent-dim" : "bg-accent"} />,
      value: compozy.value,
      formattedValue: formatContextTokens(compozy.value),
      detail: compozy.exceeds ? `of ≈ ${formatContextTokens(compozy.raw)}` : undefined,
      note: caveat ? <span className="text-warning">{caveat}</span> : undefined,
      tone: "accent",
      showBar: false,
    },
  ];
}

/** The three-tier bar and its legend: magnitude colours only, the tick is the one signal. */
function SessionContextTiers({ tiers }: { tiers: SessionContextTiersView }) {
  const items: StatusBreakdownItem[] = [
    ...compozyTierItem(tiers),
    {
      id: "agent",
      label: <TierLabel>Agent & conversation</TierLabel>,
      swatch: <TierSwatch className="bg-muted" />,
      value: tiers.agent,
      formattedValue: formatContextTokens(tiers.agent),
      tone: "neutral",
      showBar: false,
    },
    {
      id: "free",
      label: <TierLabel>Free</TierLabel>,
      swatch: <TierSwatch className="bg-canvas-tint shadow-hairline-inset" />,
      value: tiers.free,
      formattedValue: formatContextTokens(tiers.free),
      tone: "neutral",
      showBar: false,
    },
  ];
  return (
    <>
      <div className="relative">
        <StackedProgress
          className={tiers.stale ? "[&_[data-tone=accent]]:bg-accent-dim" : undefined}
          ariaLabel={`Context window: ${formatContextTokens(tiers.used)} of ${formatContextTokens(tiers.total)} used`}
          total={tiers.total}
          segments={[
            { value: tiers.compozy?.value ?? 0, tone: "accent", label: "CompozyOS context" },
            { value: tiers.agent, tone: "neutral", label: "Agent & conversation" },
          ]}
        />
        {tiers.tick != null ? (
          <span
            aria-hidden="true"
            className="absolute -inset-y-0.75 w-px bg-warning opacity-90"
            style={{ left: `${tiers.tick * 100}%` }}
          />
        ) : null}
      </div>
      <StatusBreakdown total={tiers.total} items={items} />
    </>
  );
}

function SessionContextMeterLine({ parts, warning }: { parts: string[]; warning: boolean }) {
  const Icon = warning ? Minimize2 : Clock;
  return (
    <p
      className={cn(
        "flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-micro leading-4 text-subtle",
        warning && "text-warning"
      )}
    >
      <Icon aria-hidden="true" className="size-2.75 shrink-0" />
      {parts.map((part, index) => (
        <Fragment key={part}>
          {index > 0 ? (
            <span aria-hidden="true" className="text-faint">
              ·
            </span>
          ) : null}
          <span>{part}</span>
        </Fragment>
      ))}
    </p>
  );
}

function SessionContextMeterBody({
  view,
  context,
}: {
  view: SessionContextMeterView;
  context: SessionContextView;
}) {
  if (view.kind === "empty") {
    return <SessionInspectorEmpty icon={Gauge} title={view.title} description={view.description} />;
  }
  if (view.kind === "unknown") {
    return (
      <>
        <p className="text-small-body font-medium text-fg">Context usage unknown</p>
        <p className="text-micro leading-4 text-subtle">
          This agent hasn't reported context usage.
        </p>
      </>
    );
  }
  return (
    <>
      <div className="flex flex-wrap items-baseline gap-2 tabular-nums">
        <span
          className={cn(
            "text-kpi-compact leading-none tracking-tight text-fg-strong",
            view.warning && "text-warning"
          )}
          style={{ fontWeight: "var(--font-weight-display)" }}
        >
          {view.value}
        </span>
        <span className="font-mono text-mono-id text-muted">{view.amount}</span>
        <span className="ml-auto self-center">
          <SessionContextStateChip context={context} />
        </span>
      </div>
      {view.tiers ? <SessionContextTiers tiers={view.tiers} /> : null}
      {view.line.length > 0 ? (
        <SessionContextMeterLine parts={view.line} warning={view.warning} />
      ) : null}
    </>
  );
}

export function SessionContextMeterSection({ context }: { context: SessionContextView }) {
  const view = describeSessionContextMeter(context);
  return (
    <SessionInspectorSection data-testid="session-context-meter">
      <SessionContextMeterBody view={view} context={context} />
    </SessionInspectorSection>
  );
}
