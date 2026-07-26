import { Activity, ChevronRight, FileCode, Gauge } from "lucide-react";

import { Button, Empty, Eyebrow, Metric, Pill, ScrollArea, type PillTone } from "@agh/ui";
import { describeCost } from "@/lib/cost-provenance";

import { formatMessageTimestamp } from "../lib/format-timestamp";
import { hasReportableUsage } from "./session-inspector.logic";
import type {
  InspectorFileEntry,
  InspectorTraceEvent,
  InspectorTraceKind,
  InspectorTraceStatus,
} from "./session-inspector.logic";
import type { InspectorUsage } from "./session-inspector-types";

const TRACE_STATUS_TONE: Record<InspectorTraceStatus, PillTone> = {
  ok: "success",
  warn: "warning",
  error: "danger",
  pending: "accent",
};

const TRACE_KIND_LABEL: Record<InspectorTraceKind, string> = {
  start: "START",
  user: "USER",
  agent: "AGENT",
  tool: "TOOL",
  diff: "DIFF",
  system: "SYSTEM",
  approval: "APPROVAL",
};

function formatNumber(value?: number): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—";
  return value.toLocaleString();
}

interface TraceSectionProps {
  events: InspectorTraceEvent[];
  total: number;
  limit: number;
  onViewAll?: () => void;
}

export function SessionInspectorTraceSection({
  events,
  total,
  limit,
  onViewAll,
}: TraceSectionProps) {
  return (
    <div className="flex min-h-full flex-col" data-testid="session-inspector-trace">
      {events.length === 0 ? (
        <Empty
          data-testid="session-inspector-trace-empty"
          description="Trace rows appear as the agent sends prompts, runs tools, and receives responses."
          icon={Activity}
          title="No trace events yet"
        />
      ) : (
        <ol className="flex flex-col gap-3" data-testid="session-inspector-trace-list">
          {events.map(event => (
            <TraceRow event={event} key={event.id} />
          ))}
        </ol>
      )}
      {total > limit && onViewAll ? (
        <Button
          className="mt-3 h-7 gap-1 self-start px-1 text-muted hover:text-fg"
          data-testid="session-inspector-trace-view-all"
          onClick={onViewAll}
          size="sm"
          type="button"
          variant="ghost"
        >
          View all
          <ChevronRight className="size-3" />
        </Button>
      ) : null}
    </div>
  );
}

function TraceRow({ event }: { event: InspectorTraceEvent }) {
  const tone = TRACE_STATUS_TONE[event.status];
  return (
    <li
      className="flex items-start gap-2"
      data-kind={event.kind}
      data-status={event.status}
      data-testid="session-inspector-trace-row"
    >
      <Pill.Dot
        className="mt-1 shrink-0"
        data-testid="session-inspector-trace-dot"
        pulse={event.status === "pending"}
        size="md"
        tone={tone}
      />
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <Eyebrow className="text-subtle shrink-0" data-testid="session-inspector-trace-timestamp">
          {formatMessageTimestamp(event.timestamp)}
        </Eyebrow>
        <Pill
          className="shrink-0"
          data-testid="session-inspector-trace-kind"
          mono
          tone={tone === "danger" ? "danger" : tone === "warning" ? "warning" : "neutral"}
        >
          {TRACE_KIND_LABEL[event.kind]}
        </Pill>
        <span
          className="min-w-0 flex-1 truncate text-small-body text-fg"
          data-testid="session-inspector-trace-label"
        >
          {event.label}
        </span>
      </div>
    </li>
  );
}

export function SessionInspectorUsageSection({
  usage,
}: {
  usage: InspectorUsage | null | undefined;
}) {
  const cost = describeCost({
    status: usage?.costStatus,
    source: usage?.costSource,
    amount: usage?.costUsd,
    currency: usage?.costCurrency,
  });
  const turnCount = usage?.turnCount ?? 0;
  const hasUsage = usage != null && hasReportableUsage(usage, cost);

  return (
    <div className="flex min-h-full flex-col gap-3" data-testid="session-inspector-usage">
      {hasUsage ? (
        <>
          <div className="grid grid-cols-2 gap-2" data-testid="session-inspector-usage-grid">
            <Metric
              className="p-3"
              data-testid="session-inspector-usage-tokens-in"
              label="Tokens in"
              value={formatNumber(usage?.tokensIn)}
            />
            <Metric
              className="p-3"
              data-testid="session-inspector-usage-tokens-out"
              label="Tokens out"
              value={formatNumber(usage?.tokensOut)}
            />
            <Metric
              className="col-span-2 p-3"
              data-testid="session-inspector-usage-total-tokens"
              label="Total tokens"
              value={formatNumber(usage?.totalTokens)}
            />
            <Metric
              className="col-span-2 p-3"
              data-testid="session-inspector-usage-cost"
              label="Total cost"
              subtext={cost.note ?? undefined}
              value={cost.value}
            />
          </div>
          {turnCount > 0 ? (
            <Eyebrow className="text-subtle self-start" data-testid="session-inspector-usage-turns">
              {`Across ${turnCount.toLocaleString()} turn${turnCount === 1 ? "" : "s"}`}
            </Eyebrow>
          ) : null}
        </>
      ) : (
        <Empty
          data-testid="session-inspector-usage-empty"
          description="Token counts and cost land here once the agent reports its first turn."
          icon={Gauge}
          title="No usage yet"
        />
      )}
    </div>
  );
}

export function SessionInspectorFilesSection({ files }: { files: InspectorFileEntry[] }) {
  return (
    <div className="flex min-h-full flex-col" data-testid="session-inspector-files">
      {files.length === 0 ? (
        <Empty
          data-testid="session-inspector-files-empty"
          description="Files the agent reads during this session appear here."
          icon={FileCode}
          title="No files read"
        />
      ) : (
        <ScrollArea
          className="max-h-60 rounded-md border border-line bg-canvas-soft"
          data-testid="session-inspector-files-scroll"
        >
          <ul
            className="flex flex-col divide-y divide-line"
            data-testid="session-inspector-files-list"
          >
            {files.map(file => (
              <li
                className="flex items-center gap-2 px-2 py-1.5"
                data-testid="session-inspector-files-row"
                key={file.path}
              >
                <FileCode aria-hidden="true" className="size-3 shrink-0 text-subtle" />
                <span
                  className="min-w-0 flex-1 truncate font-mono text-eyebrow text-fg"
                  data-testid="session-inspector-files-path"
                >
                  {file.path}
                </span>
                <span
                  className="shrink-0 font-mono text-badge text-subtle"
                  data-testid="session-inspector-files-count"
                >
                  ×{file.readCount}
                </span>
              </li>
            ))}
          </ul>
        </ScrollArea>
      )}
    </div>
  );
}
