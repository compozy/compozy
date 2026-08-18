import { cn, formatRelativeTime, type PillTone } from "@compozy/ui";

import type { LoopRequestView } from "../../../lib/loop-request-model";

const TONE_TEXT: Record<PillTone, string> = {
  neutral: "text-muted",
  accent: "text-accent",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  info: "text-info",
};

export interface LoopRequestSettledRowProps {
  view: LoopRequestView;
  withDivider?: boolean;
}

/** A request that can no longer be answered: the recorded outcome, never a form. */
export function LoopRequestSettledRow({ view, withDivider }: LoopRequestSettledRowProps) {
  const Glyph = view.signal.icon;
  return (
    <div
      className={cn("flex items-start gap-3 px-4 py-3", withDivider && "border-t border-line-soft")}
      data-testid="loop-request-resolution"
    >
      <Glyph
        aria-hidden="true"
        className={cn("mt-0.5 size-3.5 shrink-0", TONE_TEXT[view.signal.tone])}
      />
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
