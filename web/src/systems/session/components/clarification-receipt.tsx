import { Clock, MessageCircleQuestion, RotateCcw, X } from "@compozy/ui/icons";

import { Receipt } from "@compozy/ui";

import { clarifyAnswerLabel } from "../lib/clarify-event";
import type { ClarifyEventView } from "../types";

export interface ClarificationReceiptProps {
  view: ClarifyEventView;
  /**
   * Why a canceled question ended without an answer when the durable interaction
   * row says so: `restart` = CompozyOS restarted before anyone answered.
   */
  cause?: "restart";
}

/**
 * Truthful receipt for a terminal clarification, from durable historical
 * evidence — never invents an answer for a timeout or cancel. It wraps and
 * keeps the question's line breaks so a multi-line ask stays reviewable.
 * Static timeline entry, not a live region: bulk-loading a transcript full of
 * receipts must not fire a burst of announcements.
 */
export function ClarificationReceipt({ view, cause }: ClarificationReceiptProps) {
  if (view.status === "pending") {
    return null;
  }
  const question = view.request.question;

  if (view.status === "canceled" && cause === "restart") {
    return (
      <Receipt
        tone="neutral"
        wrap
        icon={<RotateCcw />}
        data-status={view.status}
        data-cause={cause}
        data-testid="clarification-receipt"
      >
        <b>Question not answered</b> — CompozyOS restarted before you answered · {question}
      </Receipt>
    );
  }

  if (view.status === "resolved") {
    const answer = clarifyAnswerLabel(view);
    return (
      <Receipt
        tone="neutral"
        wrap
        icon={<MessageCircleQuestion />}
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
        wrap
        icon={<Clock />}
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
      wrap
      icon={<X />}
      data-status={view.status}
      data-testid="clarification-receipt"
    >
      <b>Question canceled</b> — {question}
    </Receipt>
  );
}
