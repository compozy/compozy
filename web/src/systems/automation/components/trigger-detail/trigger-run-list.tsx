import { useState } from "react";
import type { LucideIcon } from "lucide-react";
import {
  AlertCircle,
  ArrowRight,
  Ban,
  Bot,
  BotOff,
  ChevronRight,
  Clock,
  History,
  Workflow,
} from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, Empty, Pill, SkeletonRows, cn } from "@compozy/ui";

import { buildTriggerRunView } from "../../lib/trigger-run-model";
import type { TriggerRunIcon, TriggerRunView } from "../../lib/trigger-run-model";
import type { AutomationRun, AutomationTrigger } from "../../types";
import { TriggerDetailSection } from "./trigger-detail-section";

const RUN_ICONS: Record<TriggerRunIcon, LucideIcon> = {
  agent: Bot,
  "agent-off": BotOff,
  loop: Workflow,
  clock: Clock,
  ban: Ban,
};

const PANEL_SHELL = "overflow-hidden rounded-lg border border-line bg-canvas-soft";

/**
 * The drawer's one action, and only when the daemon recorded something to open.
 * A disabled placeholder would promise a destination that does not exist yet.
 */
function TriggerRunOpenLink({ view }: { view: TriggerRunView }) {
  if (!view.link) return null;
  const linkProps =
    view.link.kind === "loop-run"
      ? ({ to: "/loop-runs/$runId", params: { runId: view.link.id } } as const)
      : ({ to: "/session/$id", params: { id: view.link.id } } as const);
  return (
    <div className="mt-2.5 flex items-center gap-2">
      <Button
        data-testid={`trigger-run-open-${view.id}`}
        nativeButton={false}
        render={<Link {...linkProps} />}
        size="xs"
        variant="neutral"
      >
        <ArrowRight className="size-3" />
        {view.link.label}
      </Button>
    </div>
  );
}

function TriggerRunRow({
  view,
  open,
  onToggle,
}: {
  view: TriggerRunView;
  open: boolean;
  onToggle: () => void;
}) {
  const MetaIcon = RUN_ICONS[view.icon];
  const drawerId = `trigger-run-drawer-${view.id}`;
  return (
    <li className="border-t border-line-soft first:border-t-0">
      <button
        aria-controls={drawerId}
        aria-expanded={open}
        className={cn(
          "grid w-full grid-cols-[6rem_minmax(0,1fr)_18px] items-center gap-3 px-4 py-2.75 text-left transition-colors duration-base ease-out md:grid-cols-[6.75rem_minmax(0,1fr)_4.5rem_18px]",
          "hover:bg-row-hover focus-visible:shadow-focus-inset focus-visible:outline-none",
          open && "bg-row-selected"
        )}
        data-testid={`automation-run-${view.id}`}
        onClick={onToggle}
        type="button"
      >
        <Pill size="sm" tone={view.tone}>
          <Pill.Dot pulse={view.pulse} tone={view.tone} />
          {view.statusLabel}
        </Pill>
        <span className="flex min-w-0 items-center gap-1.5 text-small-body text-muted">
          <MetaIcon aria-hidden="true" className="size-3 shrink-0 text-faint" strokeWidth={1.75} />
          <span className="min-w-0 truncate">{view.meta.text}</span>
          {view.meta.monoId ? (
            <span className="shrink-0 font-mono text-mono-id text-subtle">{view.meta.monoId}</span>
          ) : null}
        </span>
        <span className="hidden text-right font-mono text-form-hint tabular-nums text-subtle md:block">
          {view.duration}
        </span>
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3.5 text-subtle transition-transform duration-base ease-out",
            open && "rotate-90"
          )}
          strokeWidth={1.75}
        />
      </button>
      <div
        className="bg-input-fill px-4 pt-2.5 pb-3.5"
        data-testid={drawerId}
        hidden={!open}
        id={drawerId}
      >
        {view.drawerLines.map(line => (
          <p
            className={cn(
              "text-small-body leading-relaxed",
              line.tone === "danger" ? "text-danger" : "text-muted"
            )}
            key={line.text}
          >
            {line.text}
          </p>
        ))}
        <TriggerRunOpenLink view={view} />
      </div>
    </li>
  );
}

interface TriggerRunListProps {
  trigger: AutomationTrigger;
  runs: AutomationRun[];
  error: Error | null;
  isLoading: boolean;
  onRetry: () => void;
}

/**
 * Recent runs — the last activations the daemon kept, as a single-open
 * accordion so one run can be read without losing the list.
 */
export function TriggerRunList({ trigger, runs, error, isLoading, onRetry }: TriggerRunListProps) {
  const [openRunId, setOpenRunId] = useState<string | null>(null);
  const settled = !isLoading && !error;
  const views = settled ? runs.map(run => buildTriggerRunView(run, trigger)) : [];

  return (
    <TriggerDetailSection
      count={settled ? runs.length : undefined}
      data-testid="automation-run-history"
      gist={settled ? (runs.length === 0 ? "no runs yet" : "last 10 from the daemon") : undefined}
      icon={History}
      label="Recent runs"
    >
      <div className={PANEL_SHELL}>
        {isLoading ? (
          <div className="px-4 py-3" data-testid="automation-run-history-loading">
            <SkeletonRows count={3} />
          </div>
        ) : error ? (
          <Empty
            action={
              <Button onClick={onRetry} size="sm" type="button" variant="neutral">
                Retry
              </Button>
            }
            className="px-4 py-7"
            data-testid="automation-run-history-error"
            description={error.message || "Failed to load automation runs."}
            fill={false}
            icon={AlertCircle}
            title="Unable to load runs"
          />
        ) : views.length === 0 ? (
          <Empty
            className="px-4 py-7"
            data-testid="automation-run-history-empty"
            description="Runs will appear here after the first matching activation."
            fill={false}
            icon={History}
            title="No runs recorded yet"
          />
        ) : (
          <ul data-testid="automation-run-history-rows">
            {views.map(view => (
              <TriggerRunRow
                key={view.id}
                onToggle={() => setOpenRunId(previous => (previous === view.id ? null : view.id))}
                open={openRunId === view.id}
                view={view}
              />
            ))}
          </ul>
        )}
      </div>
    </TriggerDetailSection>
  );
}
