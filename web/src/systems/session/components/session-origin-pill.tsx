import { ArrowRightLeft, GitFork } from "lucide-react";

import { cn, Pill } from "@compozy/ui";

import type { SessionOriginView } from "../lib/session-origin";

export interface SessionOriginPillProps {
  origin: SessionOriginView;
  /** Opens the source session; absent (or an unreadable source) renders plain text. */
  onOpenSource?: (sessionId: string) => void;
}

/**
 * `Continued from {agent}` / `Forked from {title}` after the provider in the
 * status line. Information, not state: a neutral pill that keeps its verb when
 * the head narrows, truncating only the subject.
 */
export function SessionOriginPill({ origin, onOpenSource }: SessionOriginPillProps) {
  const Glyph = origin.kind === "fork" ? GitFork : ArrowRightLeft;
  const linked = origin.linkable && onOpenSource !== undefined;
  const content = (
    <>
      <Glyph aria-hidden="true" className="text-subtle" />
      <span className="shrink-0">{origin.verb}</span>{" "}
      <span className="min-w-0 max-w-[22ch] truncate text-fg">{origin.pillSubject}</span>
    </>
  );
  const shared = {
    "data-kind": origin.kind,
    "data-link": linked ? "true" : "false",
    "data-testid": "session-origin-pill",
    size: "xs" as const,
    className: cn("min-w-0 shrink", linked && "cursor-pointer hover:text-fg"),
  };

  if (!linked) {
    return <Pill {...shared}>{content}</Pill>;
  }
  return (
    <Pill
      {...shared}
      aria-label={`${origin.verb} ${origin.pillSubject}. Open the source session`}
      render={<button type="button" onClick={() => onOpenSource(origin.parentSessionId)} />}
    >
      {content}
    </Pill>
  );
}
