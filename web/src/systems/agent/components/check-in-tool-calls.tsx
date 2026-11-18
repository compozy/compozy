/** What a check-in's tools did: one line of counts, and the calls behind it on request. */
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { Disclosure, Pill } from "@compozy/ui";

import type { AgentInstanceRef } from "../adapters/agent-instance-api";
import {
  summarizeToolCalls,
  toolCallLine,
  toolCallRows,
  type CheckInToolCall,
  type CheckInToolSummary,
} from "../lib/agent-check-in-tool-calls";
import { agentInstanceOccurrenceCheckInOptions } from "../lib/query-options";

/** Each call: tool and server, read or change, allowed or why not, and how long it took. */
export function CheckInToolCallRows({ calls }: { calls: readonly CheckInToolCall[] }) {
  return (
    <ul aria-label="Tool calls" className="flex flex-col divide-y divide-line-soft">
      {toolCallRows(calls).map(row => (
        <li className="flex flex-wrap items-center gap-x-2.5 gap-y-1 py-1.5" key={row.key}>
          <span className="flex min-w-0 flex-1 items-baseline gap-1.5">
            <span className="truncate text-small-body font-medium text-fg">{row.title}</span>
            {row.server ? (
              <span className="truncate text-meta text-subtle">{row.server}</span>
            ) : null}
          </span>
          <Pill size="xs">{row.access}</Pill>
          <Pill size="xs" tone={row.allowed ? "success" : "destructive"}>
            {row.decision}
          </Pill>
          {row.effect ? (
            <Pill size="xs" tone="warning">
              {row.effect}
            </Pill>
          ) : null}
          {row.duration ? (
            <span className="text-meta text-subtle tabular-nums">{row.duration}</span>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

/** The count line as a disclosure; nothing at all when the check-in used no tools. */
export function CheckInToolCallsDisclosure({
  summary,
  defaultOpen = false,
  className,
  children,
}: {
  summary: CheckInToolSummary | null | undefined;
  defaultOpen?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const line = toolCallLine(summary);
  if (!line) return null;
  return (
    <Disclosure className={className} defaultOpen={defaultOpen} label={line}>
      <div className="pt-1">{children}</div>
    </Disclosure>
  );
}

function OccurrenceRows({
  instance,
  occurrence,
}: {
  instance: AgentInstanceRef;
  occurrence: string;
}) {
  const { data, isError, isPending } = useQuery(
    agentInstanceOccurrenceCheckInOptions(instance, occurrence)
  );
  if (isPending) return <p className="text-small-body text-subtle">Loading tool calls…</p>;
  if (isError || !data || data.tool_calls.length === 0) {
    return <p className="text-small-body text-subtle">These tool calls are no longer kept.</p>;
  }
  return <CheckInToolCallRows calls={data.tool_calls} />;
}

/**
 * A report's tool-call line. Rows load from the check-in when the list opens; a report whose
 * check-in is unknown shows the counts only.
 */
export function ReportToolCalls({
  instance,
  occurrence,
  summary,
  defaultOpen,
}: {
  instance: AgentInstanceRef | undefined;
  occurrence: string | undefined;
  summary: CheckInToolSummary | null | undefined;
  defaultOpen?: boolean;
}) {
  const line = toolCallLine(summary);
  if (!line) return null;
  if (!instance || !occurrence) return <p className="text-meta text-muted-foreground">{line}</p>;
  return (
    <CheckInToolCallsDisclosure defaultOpen={defaultOpen} summary={summary}>
      <OccurrenceRows instance={instance} occurrence={occurrence} />
    </CheckInToolCallsDisclosure>
  );
}

/** A check-in history row's calls, already read with the row. */
export function CheckInHistoryToolCalls({ calls }: { calls: readonly CheckInToolCall[] }) {
  return (
    <CheckInToolCallsDisclosure className="w-full" summary={summarizeToolCalls(calls)}>
      <CheckInToolCallRows calls={calls} />
    </CheckInToolCallsDisclosure>
  );
}
