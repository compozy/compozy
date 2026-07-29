import { Button, Eyebrow, Section, Spinner, Switch } from "@compozy/ui";

import type { ExtensionLogsModel, ExtensionLogStreamStatus } from "../hooks/use-extension-logs";

const CLOCK_FORMATTER = new Intl.DateTimeFormat(undefined, {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

const STATUS_TEXT: Record<ExtensionLogStreamStatus, string> = {
  connecting: "Connecting to the log stream",
  idle: "Not following",
  live: "Following live",
  paused: "Paused · showing retained lines",
  reconnecting: "Reconnecting · showing retained lines",
};

function formatLogClock(timestamp: string): string {
  const parsed = Date.parse(timestamp);
  return Number.isFinite(parsed) ? CLOCK_FORMATTER.format(parsed) : "--:--:--";
}

/**
 * Mirrors the daemon's redacted per-instance ring. Follow is operator-controlled, retained lines
 * survive a reconnect, and only the connection status is announced — never individual lines.
 */
export function ExtensionLogPanel({ logs, name }: { logs: ExtensionLogsModel; name: string }) {
  const followLabelId = `extension-logs-follow-${name}`;
  return (
    <Section label="Logs">
      <div className="overflow-hidden rounded-lg bg-canvas-soft" data-testid="extension-logs-panel">
        <div className="flex items-center justify-between gap-3 border-b border-line-soft px-4 py-2.5">
          <span
            className="flex min-w-0 items-center gap-2"
            data-testid="extension-logs-status"
            role="status"
          >
            {logs.status === "connecting" ? (
              <Spinner aria-hidden="true" className="size-3" />
            ) : null}
            <span className="truncate text-xs text-muted">{STATUS_TEXT[logs.status]}</span>
          </span>
          <span className="flex shrink-0 items-center gap-2">
            <Eyebrow className="text-muted" id={followLabelId}>
              Follow
            </Eyebrow>
            <Switch
              aria-labelledby={followLabelId}
              checked={logs.follow}
              data-testid="extension-logs-follow"
              onCheckedChange={logs.setFollow}
            />
          </span>
        </div>
        <ExtensionLogBody logs={logs} />
      </div>
    </Section>
  );
}

function ExtensionLogBody({ logs }: { logs: ExtensionLogsModel }) {
  if (logs.isLoading && logs.entries.length === 0) {
    return (
      <p className="px-4 py-3 text-small-body text-muted" role="status">
        Loading logs…
      </p>
    );
  }
  if (logs.error && logs.entries.length === 0) {
    return (
      <div className="space-y-2 px-4 py-3">
        <p className="text-small-body font-medium text-danger">Logs could not be loaded</p>
        <p className="text-xs text-muted">{logs.error.message}</p>
        <Button onClick={logs.refetch} size="sm" type="button" variant="outline">
          Retry logs
        </Button>
      </div>
    );
  }
  if (logs.entries.length === 0) {
    return (
      <p className="px-4 py-3 text-small-body text-muted">
        No log lines have been recorded for this instance.
      </p>
    );
  }
  return (
    <ol
      aria-live="off"
      className="max-h-64 divide-y divide-line-soft overflow-y-auto"
      data-testid="extension-logs-lines"
    >
      {logs.entries.map(entry => (
        <li className="grid gap-1 px-4 py-2" key={entry.sequence}>
          <span className="flex items-center gap-2 font-mono text-mono-id text-muted tabular-nums">
            <span>{formatLogClock(entry.timestamp)}</span>
            <span aria-hidden="true">·</span>
            <span>{entry.sequence}</span>
          </span>
          <span className="break-words font-mono text-xs text-fg">{entry.message}</span>
        </li>
      ))}
    </ol>
  );
}
