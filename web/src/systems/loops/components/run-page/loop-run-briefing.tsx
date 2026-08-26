import type { ComponentProps } from "react";
import { createElement } from "react";
import { useReducedMotionConfig } from "motion/react";
import { ArrowDown, Search } from "lucide-react";

import { Button, cn, Pill, Time } from "@compozy/ui";
import type { PillTone } from "@compozy/ui";

import type { LoopRunOutcomeModel } from "../../lib/loop-run-artifacts";
import type { LoopBriefingView } from "../../lib/loop-run-briefing-view";
import { LoopRunArtifactList } from "./loop-run-artifact-list";
import { LOOP_NEEDS_YOU_ANCHOR_ID } from "./loop-run-briefing-constants";

/**
 * The anchor the briefing's quiet action points at.
 *
 * A shared id rather than a callback: the strip has to move a keyboard user's
 * focus to the decision, and focus lives on a DOM node, not in a handler. The
 * region carries `tabIndex={-1}` so it can receive focus programmatically
 * without joining the tab order.
 */
interface LoopRunBriefingProps extends Omit<ComponentProps<"section">, "children"> {
  briefing: LoopBriefingView;
  outcome: LoopRunOutcomeModel | null;
  onOpenInspect?: () => void;
}

/**
 * The first thing anyone reads.
 *
 * It answers "what is going on" in one sentence the daemon wrote, and it is the
 * only element on the page that is allowed to lead. Failure and needs-you states
 * render here regardless of what is collapsed below, because a signal you have
 * to expand to see is a signal you will miss.
 *
 * It never carries the decision. When something needs answering, the strip says
 * so and points down to the card that owns Approve and Reject — one primary per
 * decision, in one viewport.
 */
const WEIGHT_CLASS = {
  calm: "border-line bg-canvas-soft",
  lead: "border-warning/40 bg-warning-tint",
  danger: "border-danger/40 bg-danger-tint",
} as const;

const WEIGHT_ICON_CLASS = {
  calm: "text-muted",
  lead: "text-warning",
  danger: "text-danger",
} as const;

/**
 * How a terminal run's outcome is toned.
 *
 * The briefing's weight describes urgency (`calm`, `lead`, `danger`) and is the
 * wrong axis for a finished run: a cancelled run is calm and is not a success.
 * The disposition the daemon settled on is the only thing that can say which of
 * these words the pill deserves.
 */
const OUTCOME_TONE: Record<string, PillTone> = {
  done: "success",
  canceled: "neutral",
  "no-op": "neutral",
  blocked: "warning",
  stalled: "warning",
  exhausted: "danger",
  failed: "danger",
};

function outcomeTone(status: string, weight: LoopBriefingView["weight"]): PillTone {
  return OUTCOME_TONE[status] ?? (weight === "danger" ? "danger" : "neutral");
}

function focusNeedsYou(reduced: boolean): void {
  const region = document.getElementById(LOOP_NEEDS_YOU_ANCHOR_ID);
  if (!region) return;
  // Focus, not just scroll: pointing a sighted reader at the card while leaving
  // a keyboard user's caret in the strip is only half an action. The travel is
  // the decoration, not the destination, so it is dropped under reduced motion
  // rather than the jump being cancelled with it.
  region.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "center" });
  region.focus({ preventScroll: true });
}

export function LoopRunBriefing({
  briefing,
  outcome,
  onOpenInspect,
  className,
  ...props
}: LoopRunBriefingProps) {
  const reduced = useReducedMotionConfig() === true;
  const action = briefing.action;
  const handleAction =
    action?.target === "needs-you" ? () => focusNeedsYou(reduced) : onOpenInspect;
  return (
    <section
      className={cn(
        "flex items-start gap-3 rounded-lg border px-4.5 py-4",
        WEIGHT_CLASS[briefing.weight],
        className
      )}
      data-testid="loop-run-briefing"
      data-tone={briefing.tone}
      {...props}
    >
      {createElement(briefing.icon, {
        "aria-hidden": true,
        className: cn("mt-0.5 size-3.5 shrink-0", WEIGHT_ICON_CLASS[briefing.weight]),
      })}
      <div className="min-w-0 flex-1">
        {outcome?.outcome ? (
          <div className="mb-1.5 flex flex-wrap items-center gap-2">
            <Pill
              data-testid="loop-run-briefing-outcome"
              data-outcome={outcome.outcome.status}
              tone={outcomeTone(outcome.outcome.status, briefing.weight)}
            >
              {outcome.outcome.label}
            </Pill>
            {outcome.outcome.actorLabel ? (
              <span className="text-form-hint text-subtle">by {outcome.outcome.actorLabel}</span>
            ) : null}
            {/* Who stopped it is half the answer; when they stopped it is the
                other half, and a cancellation without a time is a fact nobody
                can place against the rest of the story (US-008.EC-2). */}
            {outcome.outcome.at ? (
              <span
                className="text-form-hint text-subtle"
                data-testid="loop-run-briefing-outcome-at"
              >
                <Time iso={outcome.outcome.at} />
              </span>
            ) : null}
          </div>
        ) : null}
        <h2
          className="text-item-title font-medium tracking-tight text-pretty text-fg-strong"
          data-testid="loop-run-briefing-headline"
        >
          {briefing.headline}
        </h2>
        {briefing.detail ? (
          <p
            className="mt-1 max-w-[68ch] text-small-body leading-relaxed text-muted"
            data-testid="loop-run-briefing-detail"
          >
            {briefing.detail}
          </p>
        ) : null}
        {outcome ? <LoopRunArtifactList outcome={outcome} /> : null}
        {action && handleAction ? (
          <div className="mt-3">
            <Button
              data-testid="loop-run-briefing-action"
              onClick={handleAction}
              size="sm"
              type="button"
              variant="outline"
            >
              {action.target === "needs-you" ? (
                <ArrowDown aria-hidden="true" />
              ) : (
                <Search aria-hidden="true" />
              )}
              {action.label}
            </Button>
          </div>
        ) : null}
      </div>
    </section>
  );
}
