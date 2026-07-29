import { Clock, MessageCircleQuestion, X } from "lucide-react";

import { Receipt } from "@compozy/ui";

import { clarifyAnswerLabel } from "../lib/clarify-event";
import type { ClarifyEventView } from "../types";

/**
 * One-line truthful receipt for a terminal clarification, from durable
 * historical evidence — never invents an answer for a timeout or cancel.
 * Static timeline entry, not a live region: bulk-loading a transcript full of
 * receipts must not fire a burst of announcements.
 */
export function ClarificationReceipt({ view }: { view: ClarifyEventView }) {
  if (view.status === "pending") {
    return null;
  }
  const question = view.request.question;

  if (view.status === "resolved") {
    const answer = clarifyAnswerLabel(view);
    return (
      <Receipt
        tone="neutral"
        icon={<MessageCircleQuestion strokeWidth={1.8} />}
        data-status={view.status}
        data-testid="clarification-receipt"
      >
        Answered{" "}
        {answer ? (
          <b data-testid="clarification-receipt-answer">&quot;{answer}&quot;</b>
        ) : (
          "the question"
        )}{" "}
        — {question}
      </Receipt>
    );
  }

  if (view.status === "timed_out") {
    return (
      <Receipt
        tone="neutral"
        icon={<Clock strokeWidth={1.8} />}
        data-status={view.status}
        data-testid="clarification-receipt"
      >
        <b>Question timed out</b> — no answer before the deadline · {question}
      </Receipt>
    );
  }

  return (
    <Receipt
      tone="neutral"
      icon={<X strokeWidth={1.8} />}
      data-status={view.status}
      data-testid="clarification-receipt"
    >
      <b>Question canceled</b> — {question}
    </Receipt>
  );
}
