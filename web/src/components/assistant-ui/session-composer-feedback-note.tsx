import { ArrowUp, CornerDownRight, ListPlus, Scissors, TriangleAlert } from "lucide-react";
import type { ComponentType } from "react";

import { cn } from "@/lib/utils";
import { describeSessionBusyInputRefusal, type SessionSendOutcome } from "@/systems/session";

import type { SessionComposerFeedback } from "./hooks/session-busy-input-store";

interface FeedbackNoteView {
  Glyph: ComponentType<{ className?: string }>;
  lead: string;
  rest: string;
  suffix: string | null;
  tone: "neutral" | "warning";
}

function dispositionView(outcome: SessionSendOutcome): FeedbackNoteView {
  switch (outcome.disposition) {
    case "steering":
      switch (outcome.steerDelivery) {
        case "pending_injection":
          return {
            Glyph: CornerDownRight,
            lead: "Steering",
            rest: " — the agent sees it when the current tool finishes",
            suffix: outcome.steerDelivery,
            tone: "neutral",
          };
        case "interrupt_fallback":
          return {
            Glyph: Scissors,
            lead: "Interrupted and replaced",
            rest: " — this agent can't take guidance mid-turn",
            suffix: outcome.steerDelivery,
            tone: "neutral",
          };
        default:
          return {
            Glyph: CornerDownRight,
            lead: "Steering",
            rest: " — delivered into the live turn",
            suffix: outcome.steerDelivery,
            tone: "neutral",
          };
      }
    case "queued":
      return {
        Glyph: ListPlus,
        lead: outcome.queuePosition ? `Queued #${outcome.queuePosition}` : "Queued",
        rest: " — runs after the current turn",
        suffix: outcome.entryId,
        tone: "neutral",
      };
    case "interrupting":
      return {
        Glyph: Scissors,
        lead: "Interrupting",
        rest: " — stopping the turn, then running your message",
        suffix: null,
        tone: "neutral",
      };
    case "direct":
      return {
        Glyph: ArrowUp,
        lead: "Turn ended",
        rest: " — sent as your next message instead",
        suffix: null,
        tone: "neutral",
      };
  }
}

function feedbackView(feedback: SessionComposerFeedback): FeedbackNoteView {
  if (feedback.kind === "disposition") {
    return dispositionView(feedback.outcome);
  }
  const sentence = describeSessionBusyInputRefusal(feedback.refusal);
  const separator = sentence.indexOf(" — ");
  return {
    Glyph: TriangleAlert,
    lead: separator > 0 ? sentence.slice(0, separator) : sentence,
    rest: separator > 0 ? sentence.slice(separator) : "",
    suffix: feedback.refusal.code === "not_delivered" ? null : feedback.refusal.code,
    tone: "warning",
  };
}

/**
 * One line inside the composer card that says what happened to the last busy
 * send: the glyph names the verb, the bold phrase names the outcome, the mono
 * suffix is the daemon's own delivery word or entry id. Refusals lead with
 * "Not sent" under a warning glyph — a gate is not a failure of the system.
 */
export function SessionComposerFeedbackNote({
  feedback,
  className,
}: {
  feedback: SessionComposerFeedback;
  className?: string;
}) {
  const view = feedbackView(feedback);
  return (
    <p
      className={cn("flex min-w-0 items-center gap-1.5 text-micro text-muted", className)}
      data-code={feedback.kind === "refusal" ? feedback.refusal.code : feedback.outcome.disposition}
      data-kind={feedback.kind}
      data-testid="composer-feedback-note"
      role="status"
    >
      <view.Glyph
        aria-hidden="true"
        className={cn("size-3 shrink-0", view.tone === "warning" ? "text-warning" : "text-subtle")}
      />
      <span className="min-w-0 truncate">
        <span className="font-medium text-fg">{view.lead}</span>
        {view.rest}
      </span>
      {view.suffix ? (
        <span className="shrink-0 font-mono text-faint" data-testid="composer-feedback-suffix">
          {view.suffix}
        </span>
      ) : null}
    </p>
  );
}
