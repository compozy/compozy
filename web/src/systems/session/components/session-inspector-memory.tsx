import { AlertCircle, Library } from "lucide-react";

import { Empty, Eyebrow, MetadataList, Pill, Skeleton, cn } from "@agh/ui";

import { SessionLedgerUnavailableError } from "../adapters/session-api";
import type { SessionLedgerEvent, SessionLedgerMeta } from "../types";
import type { InspectorMemoryState } from "./session-inspector-types";

const LEDGER_EVENT_LIMIT = 20;

export function SessionInspectorMemorySection({ memory }: { memory: InspectorMemoryState }) {
  if (memory.isLoading) {
    return (
      <div
        className="flex min-h-full flex-col gap-4"
        data-state="loading"
        data-testid="session-inspector-memory"
      >
        <div
          aria-label="Loading session ledger"
          className="flex flex-col gap-2"
          data-testid="session-inspector-memory-loading"
          role="status"
        >
          <Skeleton className="h-5 w-24 rounded-xs" />
          <div className="flex flex-col gap-2 rounded-md border border-line p-3">
            {Array.from({ length: 5 }, (_, index) => (
              <div className="flex items-center justify-between gap-3" key={index}>
                <Skeleton className="h-2.5 w-20 rounded-xs" />
                <Skeleton className="h-2.5 w-32 rounded-xs" />
              </div>
            ))}
          </div>
          <Skeleton className="mt-1 h-4 w-28 rounded-xs" />
          <Skeleton className="h-16 w-full rounded-md" />
        </div>
      </div>
    );
  }
  if (memory.error && !(memory.error instanceof SessionLedgerUnavailableError)) {
    return (
      <div
        className="flex min-h-full flex-col"
        data-state="error"
        data-testid="session-inspector-memory"
      >
        <Empty
          data-testid="session-inspector-memory-error"
          description={memory.error.message || "Failed to load forensic session ledger."}
          icon={AlertCircle}
          title="Unable to load session ledger"
        />
      </div>
    );
  }
  if (!memory.ledger) {
    return (
      <div
        className="flex min-h-full flex-col"
        data-state="unavailable"
        data-testid="session-inspector-memory"
      >
        <Empty
          data-testid="session-inspector-memory-empty"
          description="The forensic ledger materializes once the session stops. Lineage and ledger event metadata appear here after that."
          icon={Library}
          title="No session ledger yet"
        />
      </div>
    );
  }
  return (
    <div
      className="flex min-h-full flex-col gap-4"
      data-state="ready"
      data-testid="session-inspector-memory"
    >
      <SessionLedgerMetaPanel meta={memory.ledger.meta} />
      <SessionLedgerEventsPanel events={memory.ledger.events} />
    </div>
  );
}

function SessionLedgerMetaPanel({ meta }: { meta: SessionLedgerMeta }) {
  const items: Array<{ label: string; value: string; testId: string; mono?: boolean }> = [
    { label: "Workspace", value: meta.workspace_id ?? "—", testId: "workspace", mono: true },
    {
      label: "Root session",
      value: meta.root_session_id ?? meta.session_id,
      testId: "root-session",
      mono: true,
    },
    {
      label: "Parent session",
      value: meta.parent_session_id ?? "—",
      testId: "parent-session",
      mono: true,
    },
    { label: "Spawn depth", value: String(meta.spawn_depth), testId: "spawn-depth", mono: true },
    {
      label: "Created",
      value: formatLedgerTimestamp(meta.created_at),
      testId: "created-at",
      mono: true,
    },
    {
      label: "Stopped",
      value: meta.stopped_at ? formatLedgerTimestamp(meta.stopped_at) : "--",
      testId: "stopped-at",
      mono: true,
    },
    { label: "Path", value: meta.path, testId: "path", mono: true },
    { label: "Checksum", value: meta.checksum, testId: "checksum", mono: true },
    { label: "Version", value: `v${meta.version}`, testId: "version", mono: true },
  ];
  return (
    <section
      aria-label="Session ledger lineage"
      className="flex flex-col gap-2"
      data-testid="session-inspector-memory-meta"
    >
      <div className="flex items-center gap-2">
        <Pill data-testid="session-inspector-memory-meta-kind" mono tone="info">
          LEDGER
        </Pill>
        <Eyebrow className="text-muted">Forensic</Eyebrow>
      </div>
      <MetadataList>
        {items.map(item => (
          <MetadataList.Row
            className="items-baseline justify-between gap-2"
            data-testid={`session-inspector-memory-meta-${item.testId}`}
            key={item.testId}
          >
            <MetadataList.Term>{item.label}</MetadataList.Term>
            <MetadataList.Value
              className={cn(
                "min-w-0 flex-1 break-all text-right text-form-label text-fg",
                item.mono ? "font-mono text-eyebrow" : null
              )}
              data-testid={`session-inspector-memory-meta-${item.testId}-value`}
            >
              {item.value}
            </MetadataList.Value>
          </MetadataList.Row>
        ))}
      </MetadataList>
    </section>
  );
}

function SessionLedgerEventsPanel({ events }: { events: readonly SessionLedgerEvent[] }) {
  const visible = events.slice(-LEDGER_EVENT_LIMIT);
  return (
    <section
      aria-label="Session ledger events"
      className="flex flex-col gap-2"
      data-testid="session-inspector-memory-events"
    >
      <div className="flex items-center justify-between gap-2">
        <Eyebrow className="text-muted">Ledger events</Eyebrow>
        <span
          className="font-mono text-badge text-subtle"
          data-testid="session-inspector-memory-events-count"
        >
          {events.length}
        </span>
      </div>
      {visible.length === 0 ? (
        <Empty
          data-testid="session-inspector-memory-events-empty"
          description="The session ended without recorded events; nothing was journaled for this run."
          icon={Library}
          title="No ledger events"
        />
      ) : (
        <ul
          className="flex flex-col divide-y divide-line"
          data-testid="session-inspector-memory-events-list"
        >
          {visible.map(event => (
            <li
              className="flex items-center gap-2 py-2"
              data-testid="session-inspector-memory-event-row"
              key={`${event.sequence}-${event.event_type}`}
            >
              <Eyebrow
                className="text-subtle shrink-0"
                data-testid="session-inspector-memory-event-sequence"
              >
                #{event.sequence}
              </Eyebrow>
              <Pill data-testid="session-inspector-memory-event-type" mono tone="neutral">
                {event.event_type}
              </Pill>
              <span
                className="ml-auto shrink-0 font-mono text-badge text-subtle"
                data-testid="session-inspector-memory-event-timestamp"
              >
                {formatLedgerTimestamp(event.emitted_at)}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function formatLedgerTimestamp(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
