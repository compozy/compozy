import { FileCode, Gauge } from "lucide-react";

import { Empty, Eyebrow, Metric, ScrollArea } from "@compozy/ui";
import { describeCost } from "@/lib/cost-provenance";

import { hasReportableUsage } from "./session-inspector.logic";
import type { InspectorFileEntry } from "./session-inspector.logic";
import type { InspectorUsage } from "./session-inspector-types";

function formatNumber(value?: number): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "—";
  return value.toLocaleString();
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
