import { Radar, X } from "@compozy/ui/icons";

import {
  Button,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  StateGlyph,
  Time,
} from "@compozy/ui";

import type { SessionMonitorPayload } from "../adapters/session-monitor-api";
import {
  monitorControlLabel,
  monitorIsWaking,
  monitorWatchLabel,
} from "../lib/session-monitor-format";
import { ComposerPopoverRowsSkeleton } from "./composer-popover-rows-skeleton";

export interface MonitorComposerControlProps {
  /** Active monitors the session reports; the trigger shows only while it is positive. */
  active: number;
  monitors: readonly SessionMonitorPayload[];
  loading?: boolean;
  failed?: boolean;
  /** The list read found no such session; shown without a retry. */
  missing?: boolean;
  onRetry?: () => void;
  onCancel?: (monitorId: string) => void;
  cancelingId?: string | null;
  /** The popover opened or closed; the owner reads the list while it is open. */
  onOpenChange?: (open: boolean) => void;
}

// The row's geometry, shared by the live rows and their placeholders so the two cannot drift.
const MONITOR_ROW_FRAME = "group/monitor-row flex items-start gap-2 rounded-md px-0.5 py-1";
const MONITOR_ROW_TEXT_FRAME = "flex min-w-0 flex-1 flex-col gap-0.5";

const DESCRIPTION_WIDTHS = ["w-48", "w-40", "w-52"] as const;
const WATCH_WIDTHS = ["w-32", "w-28", "w-36"] as const;

function MonitorRow({
  monitor,
  canceling,
  onCancel,
}: {
  monitor: SessionMonitorPayload;
  canceling: boolean;
  onCancel?: (monitorId: string) => void;
}) {
  const waking = monitorIsWaking(monitor);
  return (
    <li
      className={MONITOR_ROW_FRAME}
      data-testid={`composer-monitor-${monitor.monitor_id}`}
      data-status={monitor.status}
    >
      <StateGlyph size="sm" state={waking ? "running" : "monitoring"} className="mt-0.75" />
      <span className={MONITOR_ROW_TEXT_FRAME}>
        <span className="line-clamp-2 text-transcript-meta text-pretty text-fg">
          {monitor.description}
        </span>
        <span className="flex min-w-0 items-center gap-1 text-transcript-caption text-muted-foreground">
          <span className="truncate">{monitorWatchLabel(monitor)}</span>
          <span aria-hidden="true">·</span>
          {waking ? (
            <span className="shrink-0">Waking the session…</span>
          ) : (
            <span className="shrink-0">
              {monitor.kind === "timer" ? "fires " : "gives up "}
              <Time iso={monitor.deadline_at} />
            </span>
          )}
        </span>
      </span>
      {onCancel && !waking ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={`Cancel monitor: ${monitor.description}`}
          disabled={canceling}
          onClick={() => onCancel(monitor.monitor_id)}
          className="text-muted-foreground"
        >
          {canceling ? <StateGlyph size="sm" state="running" /> : <X aria-hidden="true" />}
        </Button>
      ) : null}
    </li>
  );
}

function MonitorLoadError({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-0.5">
      <p className="text-transcript-caption text-muted-foreground">{message}</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

function MonitorComposerBody({
  active,
  monitors,
  loading = false,
  failed = false,
  missing = false,
  onRetry,
  onCancel,
  cancelingId = null,
}: Omit<MonitorComposerControlProps, "onOpenChange">) {
  // A gone session outranks rows cached from an earlier read.
  if (missing) {
    return <MonitorLoadError message="This session is no longer available." />;
  }
  if (monitors.length > 0) {
    return (
      <div className="flex flex-col gap-1">
        <ul className="flex flex-col gap-0.5">
          {monitors.map(monitor => (
            <MonitorRow
              key={monitor.monitor_id}
              monitor={monitor}
              canceling={cancelingId === monitor.monitor_id}
              onCancel={onCancel}
            />
          ))}
        </ul>
        {failed ? (
          <MonitorLoadError message="Couldn't refresh monitors." onRetry={onRetry} />
        ) : null}
      </div>
    );
  }
  if (failed) {
    return <MonitorLoadError message="Couldn't load monitors." onRetry={onRetry} />;
  }
  if (loading) {
    // One placeholder row per active monitor the session's summary reports (capped).
    return (
      <ComposerPopoverRowsSkeleton
        label="Loading monitors"
        data-testid="composer-monitors-loading"
        count={active}
        rowClassName={MONITOR_ROW_FRAME}
        textClassName={MONITOR_ROW_TEXT_FRAME}
        titleWidths={DESCRIPTION_WIDTHS}
        metaWidths={WATCH_WIDTHS}
      />
    );
  }
  return (
    <p className="px-0.5 text-transcript-caption text-muted-foreground">
      Nothing is being monitored.
    </p>
  );
}

/**
 * The composer's monitor control: shown only while the session has active
 * monitors, so a session whose turn ended but still waits on one never reads
 * as done. Its popover lists what each monitor watches and when it fires or
 * gives up, with a cancel action that never wakes the session.
 */
export function MonitorComposerControl({ onOpenChange, ...body }: MonitorComposerControlProps) {
  const { active } = body;
  if (active <= 0) return null;
  return (
    <Popover onOpenChange={open => onOpenChange?.(open)}>
      <PopoverTrigger
        render={
          <InputGroupButton
            type="button"
            size="tool"
            aria-label={monitorControlLabel(active)}
            data-testid="composer-monitors-button"
            className="shrink-0 text-warning data-[popup-open]:bg-surface-2"
          />
        }
      >
        <Radar aria-hidden="true" className="size-4" />
        <span className="font-mono text-mono-id tabular-nums">{active}</span>
      </PopoverTrigger>
      <PopoverContent side="top" align="start" className="w-80 max-w-[calc(100vw-2rem)] gap-2">
        <div className="flex items-baseline gap-2 px-0.5">
          <PopoverTitle className="text-transcript-meta font-semibold text-fg-strong">
            Monitoring
          </PopoverTitle>
          <span className="ml-auto text-transcript-caption text-faint">
            Wakes this session when it fires
          </span>
        </div>
        <MonitorComposerBody {...body} />
      </PopoverContent>
    </Popover>
  );
}
