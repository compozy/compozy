import { AlertCircle, Ban, CircleDashed, FoldVertical, LoaderCircle } from "lucide-react";
import type { ReactNode } from "react";

import { Disclosure, Marker, type MarkerTone } from "@compozy/ui";

import type { SessionCompactionItemData } from "../types";
import { MessageMarkdown } from "./message-markdown";

export interface SessionCompactionItemProps {
  item: SessionCompactionItemData;
}

interface CompactionView {
  tone: MarkerTone;
  icon: ReactNode;
  label: string;
  /** The subject reads at the marker's `<b>` ink; muted states stay one step quieter. */
  emphasized: boolean;
}

// The agent's own status, never CompozyOS's: only `in_progress` is live and only
// the three terminal words have a sentence of their own. Anything else is a
// vendor status (for example `…_paused`) passed through verbatim — not finished,
// so it never borrows a terminal sentence or tone.
function compactionView(status: string): CompactionView {
  switch (status) {
    case "in_progress":
      return {
        tone: "neutral",
        icon: (
          <LoaderCircle
            data-testid="session-compaction-spinner"
            className="animate-spin text-subtle motion-reduce:animate-none"
          />
        ),
        label: "Compacting context…",
        emphasized: true,
      };
    case "completed":
      return {
        tone: "neutral",
        icon: <FoldVertical />,
        label: "Context compacted",
        emphasized: true,
      };
    case "failed":
      return {
        tone: "danger",
        icon: <AlertCircle />,
        label: "Context compaction failed",
        emphasized: true,
      };
    case "cancelled":
      return {
        tone: "neutral",
        icon: <Ban />,
        label: "Context compaction cancelled",
        emphasized: false,
      };
    default:
      return { tone: "neutral", icon: <CircleDashed />, label: status, emphasized: false };
  }
}

/**
 * One observed agent compaction as a calm timeline row: a tone glyph plus one
 * sentence (`Marker`), with the agent's summary — when it sent one — behind a
 * closed "Summary" disclosure. The same compaction id updates in place, so the
 * row follows the item from "Compacting context…" to its terminal state.
 */
export function SessionCompactionItem({ item }: SessionCompactionItemProps) {
  const view = compactionView(item.status);
  const failed = item.status === "failed";
  const summary = item.summary?.trim() ? item.summary : null;
  const error = failed && item.error?.trim() ? item.error : null;
  const label = view.emphasized ? <b>{view.label}</b> : view.label;

  return (
    <div
      data-testid="session-compaction-item"
      data-status={item.status}
      className="flex min-w-0 flex-col"
    >
      <Marker role={failed ? "alert" : "status"} tone={view.tone} icon={view.icon}>
        {label}
        {error ? (
          <>
            {" — "}
            <span data-testid="session-compaction-error" className="wrap-anywhere">
              {error}
            </span>
          </>
        ) : null}
      </Marker>
      {summary ? (
        <Disclosure
          label="Summary"
          size="md"
          className="ml-1"
          contentProps={{ className: "pt-0.5" }}
          data-testid="session-compaction-summary"
        >
          <div
            aria-label="Compaction summary"
            data-testid="session-compaction-summary-content"
            role="region"
            tabIndex={0}
            className="mb-1 ml-transcript-detail-indent max-h-60 overflow-y-auto border-l border-line pl-transcript-detail-gutter text-transcript-body leading-relaxed text-muted select-text focus-visible:shadow-focus-ring focus-visible:outline-none"
          >
            <MessageMarkdown content={summary} compact="relaxed" />
          </div>
        </Disclosure>
      ) : null}
    </div>
  );
}
