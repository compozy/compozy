import { Empty, Eyebrow } from "@agh/ui";
import { Activity } from "lucide-react";

import { cn } from "@/lib/utils";

import type { OpenWorkEntry } from "../../hooks/use-work";
import { WorkInspectorRow } from "./work-inspector-row";

export interface WorkInspectorProps {
  entries: ReadonlyArray<OpenWorkEntry>;
  isLoading?: boolean;
  onJump?: (entry: OpenWorkEntry) => void;
  className?: string;
  /**
   * When true, render only the list/empty state without the inner header
   * (count badge moves to the parent tab). Phase E uses this from
   * `<NetworkInspector>` to host the Work tab body.
   */
  chromeless?: boolean;
  totalCount?: number;
}

export function WorkInspector({
  entries,
  isLoading = false,
  onJump,
  className,
  chromeless = false,
  totalCount,
}: WorkInspectorProps) {
  const resolvedTotalCount = totalCount === undefined ? entries.length : totalCount;
  const body =
    isLoading && entries.length === 0 ? (
      <p className="px-4 py-6 text-small-body text-subtle">Loading…</p>
    ) : entries.length === 0 && resolvedTotalCount > 0 ? (
      <div className="flex justify-center px-4 py-6">
        <Empty
          className="max-w-sm"
          description={`${resolvedTotalCount} open work ${resolvedTotalCount === 1 ? "item is" : "items are"} known, but ${resolvedTotalCount === 1 ? "its" : "their"} lifecycle messages are outside the loaded timeline.`}
          fill={false}
          icon={Activity}
          title="Open work details not loaded."
        />
      </div>
    ) : entries.length === 0 ? (
      <div className="flex justify-center px-4 py-6">
        <Empty
          className="max-w-sm"
          description="The active container has no open work right now."
          fill={false}
          icon={Activity}
          title="No work in flight."
        />
      </div>
    ) : (
      <ul
        aria-label="Open work entries"
        className="flex flex-1 flex-col overflow-y-auto"
        data-testid="network-work-inspector-list"
      >
        {entries.map(entry => (
          <li className="border-b border-line last:border-b-0" key={entry.workId}>
            <WorkInspectorRow entry={entry} onJump={onJump} />
          </li>
        ))}
      </ul>
    );

  if (chromeless) {
    return (
      <section
        aria-label="Open network work"
        className={cn("flex min-h-0 flex-1 flex-col", className)}
        data-testid="network-work-inspector"
      >
        {body}
      </section>
    );
  }

  return (
    <section
      aria-label="Open network work"
      className={cn("flex min-h-0 flex-1 flex-col", className)}
      data-testid="network-work-inspector"
    >
      <header className="flex items-baseline justify-between border-b border-line px-4 py-3">
        <h2 className="text-sm font-medium text-fg">Work</h2>
        <Eyebrow data-testid="network-work-inspector-count">
          {entries.length} loaded · {resolvedTotalCount} open
        </Eyebrow>
      </header>
      {body}
    </section>
  );
}
