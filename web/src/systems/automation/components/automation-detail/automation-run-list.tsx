import { useState, type ComponentProps } from "react";
import type { LucideIcon } from "lucide-react";
import {
  AlertCircle,
  ArrowRight,
  Bot,
  ChevronRight,
  Clock,
  History,
  MessageSquare,
  Play,
  Repeat2,
  RotateCcw,
  SkipForward,
  SquareCheck,
} from "lucide-react";
import { Link } from "@tanstack/react-router";

import { Button, Empty, SkeletonRows, StateGlyph, Time, cn } from "@compozy/ui";

import type { AutomationEntity } from "../../lib/automation-entity";
import { buildAutomationRunView } from "../../lib/automation-run-model";
import type { AutomationRunIcon, AutomationRunView } from "../../lib/automation-run-model";
import type { AutomationRun } from "../../types";
import { AutomationDetailSection } from "./automation-detail-section";

const RUN_ICONS = {
  agent: Bot,
  session: MessageSquare,
  loop: Repeat2,
  task: SquareCheck,
  manual: Play,
  skip: SkipForward,
  clock: Clock,
} as const satisfies Record<AutomationRunIcon, LucideIcon>;

const PANEL_SHELL = "overflow-hidden rounded-lg bg-card shadow-card";

function runLinkProps(link: NonNullable<AutomationRunView["link"]>) {
  switch (link.kind) {
    case "loop-run":
      return {
        to: "/loop-runs/$runId",
        params: { runId: link.id },
        search: link.workspaceId ? { workspace: link.workspaceId } : {},
      } as const;
    case "task":
      return { to: "/tasks/$id", params: { id: link.id } } as const;
    case "session":
      return { to: "/session/$id", params: { id: link.id } } as const;
  }
}

interface RunDrawerActionsProps extends Omit<ComponentProps<"div">, "children"> {
  view: AutomationRunView;
  onSetUpRetries?: () => void;
}

/**
 * The drawer's next step: open what the run produced (only when the daemon
 * recorded it) and, for a failure with no retries, the fix.
 */
function RunDrawerActions({ view, onSetUpRetries, className, ...props }: RunDrawerActionsProps) {
  const retries = view.offerRetries && onSetUpRetries;
  if (!view.link && !retries) return null;
  return (
    <div className={cn("mt-2.5 flex items-center gap-2", className)} {...props}>
      {view.link ? (
        <Button
          data-testid={`automation-run-open-${view.id}`}
          nativeButton={false}
          render={<Link {...runLinkProps(view.link)} />}
          size="xs"
          variant="neutral"
        >
          <ArrowRight />
          {view.link.label}
        </Button>
      ) : null}
      {retries ? (
        <Button
          data-testid={`automation-run-retries-${view.id}`}
          onClick={onSetUpRetries}
          size="xs"
          type="button"
          variant="neutral"
        >
          <RotateCcw />
          Set up retries
        </Button>
      ) : null}
    </div>
  );
}

interface RunRowProps extends Omit<ComponentProps<"li">, "children"> {
  view: AutomationRunView;
  open: boolean;
  onOpenToggle: () => void;
  onSetUpRetries?: () => void;
}

function RunRow({ view, open, onOpenToggle, onSetUpRetries, className, ...props }: RunRowProps) {
  const MetaIcon = RUN_ICONS[view.icon];
  const drawerId = `automation-run-drawer-${view.id}`;
  return (
    <li className={cn("border-t border-line-soft first:border-t-0", className)} {...props}>
      <button
        aria-controls={drawerId}
        aria-expanded={open}
        className={cn(
          "grid w-full grid-cols-[6.5rem_minmax(0,1fr)_18px] items-center gap-3 px-4 py-2.75 text-left transition-colors duration-base ease-out md:grid-cols-[6.75rem_minmax(0,1fr)_4.5rem_5.5rem_18px]",
          "hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none",
          open && "bg-selected"
        )}
        data-testid={`automation-run-${view.id}`}
        onClick={onOpenToggle}
        type="button"
      >
        <span
          className="inline-flex min-w-0 items-center gap-1.5 text-small-body text-fg-2"
          data-state={view.glyph}
        >
          <StateGlyph state={view.glyph} />
          <span className="truncate">{view.statusLabel}</span>
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-small-body text-muted">
          <MetaIcon aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
          <span className="min-w-0 truncate">{view.meta.text}</span>
          {view.meta.monoId ? (
            <span className="shrink-0 font-mono text-mono-id text-subtle">{view.meta.monoId}</span>
          ) : null}
        </span>
        <span className="hidden text-right font-mono text-form-hint tabular-nums text-subtle md:block">
          {view.duration}
        </span>
        <span className="hidden text-right text-form-hint text-subtle md:block">
          {view.at ? <Time iso={view.at} /> : "—"}
        </span>
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3.5 text-subtle transition-transform duration-base ease-out",
            open && "rotate-90"
          )}
        />
      </button>
      <div
        className="bg-sunken px-4 pt-2.5 pb-3.5"
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
            key={line.id}
          >
            {line.text}
          </p>
        ))}
        <RunDrawerActions onSetUpRetries={onSetUpRetries} view={view} />
      </div>
    </li>
  );
}

interface AutomationRunListProps {
  entity: AutomationEntity;
  runs: readonly AutomationRun[];
  error: Error | null;
  isLoading: boolean;
  /** Schedules that are on name their next run in the empty state. */
  nextRunAt?: string;
  onRetry: () => void;
  /** Offered in a failed run's drawer when retries are off and the automation is editable. */
  onSetUpRetries?: () => void;
}

/**
 * Runs — the last activations the daemon kept, newest first, as a single-open
 * accordion so one run can be read without losing the list. One list for
 * every kind; the glyph carries the tone and the word stays quiet.
 */
export function AutomationRunList({
  entity,
  runs,
  error,
  isLoading,
  nextRunAt,
  onRetry,
  onSetUpRetries,
}: AutomationRunListProps) {
  const [openRunId, setOpenRunId] = useState<string | null>(null);
  const settled = !isLoading && !error;
  const views = settled ? runs.map(run => buildAutomationRunView(run, entity)) : [];

  return (
    <AutomationDetailSection
      count={settled ? runs.length : undefined}
      data-testid="automation-run-list"
      gist={settled ? (runs.length === 0 ? "no runs yet" : "last 10 recorded") : undefined}
      icon={History}
      label="Runs"
    >
      <div className={PANEL_SHELL}>
        {isLoading ? (
          <div className="px-4 py-3" data-testid="automation-run-list-loading">
            <SkeletonRows count={3} />
          </div>
        ) : error ? (
          <Empty
            action={
              <Button onClick={onRetry} size="sm" type="button" variant="neutral">
                Try again
              </Button>
            }
            className="px-4 py-7"
            data-testid="automation-run-list-error"
            description={error.message || "Failed to load automation runs."}
            fill={false}
            icon={AlertCircle}
            title="Unable to load runs"
          />
        ) : views.length === 0 ? (
          <Empty
            className="px-4 py-7"
            data-testid="automation-run-list-empty"
            description={
              nextRunAt ? (
                <>
                  Next run <Time iso={nextRunAt} />
                </>
              ) : undefined
            }
            fill={false}
            icon={History}
            title="No runs yet"
          />
        ) : (
          <ul data-testid="automation-run-list-rows">
            {views.map(view => (
              <RunRow
                key={view.id}
                onOpenToggle={() =>
                  setOpenRunId(previous => (previous === view.id ? null : view.id))
                }
                onSetUpRetries={onSetUpRetries}
                open={openRunId === view.id}
                view={view}
              />
            ))}
          </ul>
        )}
      </div>
    </AutomationDetailSection>
  );
}
