import { cn, formatRelativeTime, StateGlyph } from "@compozy/ui";

import type { LoopRequestView } from "../../../lib/loop-request-model";

export interface LoopRequestSettledRowProps {
  view: LoopRequestView;
  withDivider?: boolean;
}

/** A request that can no longer be answered: the recorded outcome, never a form. */
export function LoopRequestSettledRow({ view, withDivider }: LoopRequestSettledRowProps) {
  return (
    <div
      className={cn("flex items-start gap-3 px-4 py-3", withDivider && "border-t border-line-soft")}
      data-testid="loop-request-resolution"
    >
      <StateGlyph className="mt-1" state={view.signal.glyph} />
      <div className="min-w-0 flex-1">
        <div className="text-ws-name font-medium text-fg">
          {view.request.prompt === "" ? view.title : view.request.prompt}
        </div>
        <p className="mt-0.5 max-w-[62ch] text-form-hint leading-relaxed text-subtle">
          {outcomeSentence(view)}
        </p>
      </div>
      {view.resolution?.at ? (
        <span className="shrink-0 pt-0.5 font-mono text-mono-id whitespace-nowrap text-subtle">
          {formatRelativeTime(view.resolution.at)}
        </span>
      ) : null}
    </div>
  );
}

function outcomeSentence(view: LoopRequestView): string {
  if (view.state === "pending") return "The run ended before this request was answered.";
  if (view.state === "expired") return "The deadline passed before this request was answered.";
  const actor = [view.resolution?.actorKind, view.resolution?.actorId].filter(Boolean).join(" ");
  const answered = view.resolution?.decision ?? "";
  if (view.state === "canceled") {
    return actor === "" ? "This request was canceled." : `${actor} canceled this request.`;
  }
  if (actor !== "" && answered !== "") return `${actor} answered with ${answered}.`;
  if (actor !== "") return `${actor} answered this request.`;
  if (answered !== "") return `Answered with ${answered}.`;
  return "Someone else answered this request.";
}
