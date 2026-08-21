import { createElement } from "react";
import { ArrowDown, Search } from "lucide-react";

import { Button, cn, Pill, Time } from "@compozy/ui";

import type { LoopRunOutcomeModel } from "../../lib/loop-run-artifacts";
import type { LoopBriefingView } from "../../lib/loop-run-briefing-view";
import { LoopRunArtifactList } from "./loop-run-artifact-list";

/**
 * The anchor the briefing's quiet action points at.
 *
 * A shared id rather than a callback: the strip has to move a keyboard user's
 * focus to the decision, and focus lives on a DOM node, not in a handler. The
 * region carries `tabIndex={-1}` so it can receive focus programmatically
 * without joining the tab order.
 */
export const LOOP_NEEDS_YOU_ANCHOR_ID = "loop-run-needs-you";

interface LoopRunBriefingProps {
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

function focusNeedsYou(): void {
  const region = document.getElementById(LOOP_NEEDS_YOU_ANCHOR_ID);
  if (!region) return;
  // Focus, not just scroll: pointing a sighted reader at the card while leaving
  // a keyboard user's caret in the strip is only half an action.
  region.scrollIntoView({ behavior: "smooth", block: "center" });
  region.focus({ preventScroll: true });
}

export function LoopRunBriefing({ briefing, outcome, onOpenInspect }: LoopRunBriefingProps) {
  const action = briefing.action;
  const handleAction = action?.target === "needs-you" ? focusNeedsYou : onOpenInspect;
  return (
    <section
      className={cn(
        "flex items-start gap-3 rounded-lg border px-4.5 py-4",
        WEIGHT_CLASS[briefing.weight]
      )}
      data-testid="loop-run-briefing"
      data-tone={briefing.tone}
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
              tone={briefing.weight === "danger" ? "danger" : "success"}
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
