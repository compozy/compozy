import { Eyebrow } from "@agh/ui";

import { clarifyAnswerLabel } from "../lib/clarify-event";
import type { ClarifyEventView } from "../types";

interface TerminalReceiptMeta {
  label: string;
  toneClass: string;
  detail: string;
}

const RECEIPT_META: Record<"resolved" | "timed_out" | "canceled", TerminalReceiptMeta> = {
  resolved: { label: "Answered", toneClass: "text-success", detail: "" },
  timed_out: {
    label: "Timed out",
    toneClass: "text-warning",
    detail: "No answer was given before the deadline.",
  },
  canceled: { label: "Canceled", toneClass: "text-subtle", detail: "This question was canceled." },
};

/**
 * Non-interactive, truthful receipt for a terminal clarification. Rendered from durable historical
 * evidence — never invents an answer for a timeout and stays visually quiet and distinct from
 * permission receipts.
 */
export function ClarificationReceipt({ view }: { view: ClarifyEventView }) {
  if (view.status === "pending") {
    return null;
  }
  const meta = RECEIPT_META[view.status];
  const answer = view.status === "resolved" ? clarifyAnswerLabel(view) : null;
  // Durable historical evidence — a static timeline entry, not a live-region status. Bulk-loading a
  // transcript full of receipts must not fire a burst of announcements (matches `SessionToolCallRow`).
  return (
    <div
      className="my-1.5 flex w-full min-w-0 flex-col gap-0.5 rounded-md border border-line bg-canvas-soft px-3 py-2"
      data-status={view.status}
      data-testid="clarification-receipt"
    >
      <span className="flex min-w-0 items-center gap-1.5">
        <Eyebrow className={meta.toneClass}>{meta.label}</Eyebrow>
        <span className="min-w-0 truncate text-form-label text-muted">{view.request.question}</span>
      </span>
      {answer ? (
        <p className="text-small-body text-fg" data-testid="clarification-receipt-answer">
          {answer}
        </p>
      ) : meta.detail ? (
        <p className="text-form-hint text-muted">{meta.detail}</p>
      ) : null}
    </div>
  );
}
