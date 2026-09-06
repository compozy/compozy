import { ArrowUp, CornerDownRight, ListPlus, Scissors, TriangleAlert, WifiOff } from "lucide-react";
import type { ComponentType } from "react";

import { cn } from "@/lib/utils";
import {
  describeSessionBusyInputRefusal,
  describeSessionBusyInputUnconfirmed,
  type SessionSendOutcome,
} from "@/systems/session";

import type { SessionComposerFeedback } from "./hooks/session-busy-input-store";

interface FeedbackNoteView {
  Glyph: ComponentType<{ className?: string }>;
  lead: string;
  rest: string;
  suffix: string | null;
  tone: "neutral" | "warning" | "info";
}

function dispositionView(outcome: SessionSendOutcome): FeedbackNoteView {
  if (outcome.replayed) {
    return replayedView(outcome);
  }
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

/**
 * A replay of a retained identity: the daemon already had it, so the original
 * outcome comes back and nothing was sent twice (US-007.AC-2).
 */
function replayedView(outcome: SessionSendOutcome): FeedbackNoteView {
  const lead =
    outcome.disposition === "queued"
      ? outcome.queuePosition
        ? `Queued #${outcome.queuePosition}`
        : "Queued"
      : outcome.disposition === "steering"
        ? "Steering"
        : outcome.disposition === "interrupting"
          ? "Interrupting"
          : "Sent";
  return {
    Glyph: outcome.disposition === "queued" ? ListPlus : CornerDownRight,
    lead,
    rest: " — it had arrived; nothing was sent twice",
    suffix: "replayed",
    tone: "neutral",
  };
}

function splitSentence(sentence: string): Pick<FeedbackNoteView, "lead" | "rest"> {
  const separator = sentence.indexOf(" — ");
  return {
    lead: separator > 0 ? sentence.slice(0, separator) : sentence,
    rest: separator > 0 ? sentence.slice(separator) : "",
  };
}

function feedbackView(feedback: SessionComposerFeedback): FeedbackNoteView {
  if (feedback.kind === "disposition") {
    return dispositionView(feedback.outcome);
  }
  if (feedback.kind === "unconfirmed") {
    // No proof either way: the daemon may have accepted the send before the
    // answer was lost. The strip row keeps the identity and Retry; this line
    // only says so — never "Not sent".
    return {
      Glyph: TriangleAlert,
      ...splitSentence(describeSessionBusyInputUnconfirmed(feedback.message)),
      suffix: null,
      tone: "warning",
    };
  }
  const sentence = describeSessionBusyInputRefusal(feedback.refusal);
  const separator = sentence.indexOf(" — ");
  // A disconnect is the client refusing before any request left: info, no code suffix.
  const disconnected = feedback.refusal.code === "disconnected";
  return {
    Glyph: disconnected ? WifiOff : TriangleAlert,
    lead: separator > 0 ? sentence.slice(0, separator) : sentence,
    rest: separator > 0 ? sentence.slice(separator) : "",
    suffix:
      feedback.refusal.code === "not_delivered" || disconnected ? null : feedback.refusal.code,
    tone: disconnected ? "info" : "warning",
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
      data-code={
        feedback.kind === "refusal"
          ? feedback.refusal.code
          : feedback.kind === "unconfirmed"
            ? "unconfirmed"
            : feedback.outcome.disposition
      }
      data-kind={feedback.kind}
      data-testid="composer-feedback-note"
      role="status"
    >
      <view.Glyph
        aria-hidden="true"
        className={cn(
          "size-3 shrink-0",
          view.tone === "warning"
            ? "text-warning"
            : view.tone === "info"
              ? "text-info"
              : "text-subtle"
        )}
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
