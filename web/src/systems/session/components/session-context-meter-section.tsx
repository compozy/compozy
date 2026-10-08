import type { ComponentProps } from "react";
import { Gauge } from "lucide-react";
import { Pill, StackedProgress, StatusBreakdown, cn } from "@compozy/ui";
import type { SessionContextView } from "../lib/session-context";
import { formatContextTokens } from "../lib/context-format";
import type { SessionPayload } from "../types";
import {
  describeSessionContextMeter,
  type SessionContextMeterView,
  type SessionContextTiersView,
} from "../lib/session-context-view";
import { SessionContextCompactAction } from "./session-context-compact-action";
import { SessionInspectorEmpty, SessionInspectorSection } from "./session-inspector-section";

type StatusBreakdownItem = ComponentProps<typeof StatusBreakdown>["items"][number];

/** Tier swatches are magnitude keys: 8px squares, never signal dots. */
function TierSwatch({ className, ...props }: ComponentProps<"span">) {
  return (
    <span
      aria-hidden="true"
      className={cn("size-2 shrink-0 rounded-[2px]", className)}
      {...props}
    />
  );
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

/** The three-tier bar and its legend: magnitude colours only. */
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
      swatch: <TierSwatch className="bg-surface-2 shadow-hairline-inset" />,
      value: tiers.free,
      formattedValue: formatContextTokens(tiers.free),
      tone: "neutral",
      showBar: false,
    },
  ];
  return (
    <>
      <StackedProgress
        className={tiers.stale ? "[&_[data-tone=accent]]:bg-accent-dim" : undefined}
        ariaLabel={`Context window: ${formatContextTokens(tiers.used)} of ${formatContextTokens(tiers.total)} used`}
        total={tiers.total}
        segments={[
          { value: tiers.compozy?.value ?? 0, tone: "accent", label: "CompozyOS context" },
          { value: tiers.agent, tone: "neutral", label: "Agent & conversation" },
        ]}
      />
      <StatusBreakdown total={tiers.total} items={items} />
    </>
  );
}

function SessionContextMeterBody({ view }: { view: SessionContextMeterView }) {
  if (view.kind === "empty") {
    return <SessionInspectorEmpty icon={Gauge} title={view.title} description={view.description} />;
  }
  if (view.kind === "unknown") {
    return (
      <>
        <p className="text-small-body font-medium text-fg">Context usage unknown</p>
        <p className="text-micro leading-4 text-subtle">{view.sentence}</p>
      </>
    );
  }
  return (
    <>
      <div className="flex flex-wrap items-baseline gap-2 tabular-nums">
        <span className="text-kpi-compact leading-none font-semibold tracking-tight text-fg">
          {view.value}
        </span>
        <span className="font-mono text-mono-id text-muted">{view.amount}</span>
        {view.chip ? (
          <span className="ml-auto self-center">
            <Pill size="xs" form={view.chip.form} tone={view.chip.tone}>
              {view.chip.label}
            </Pill>
          </span>
        ) : null}
      </div>
      {view.tiers ? <SessionContextTiers tiers={view.tiers} /> : null}
    </>
  );
}

export function SessionContextMeterSection({
  context,
  session,
}: {
  context: SessionContextView;
  /** The inspected session; its advertised commands decide whether Compact now is offered. */
  session?: SessionPayload;
}) {
  const view = describeSessionContextMeter(context);
  return (
    <SessionInspectorSection data-testid="session-context-meter">
      <SessionContextMeterBody view={view} />
      {session ? <SessionContextCompactAction session={session} /> : null}
    </SessionInspectorSection>
  );
}
