import { createElement } from "react";

import type { SteerMarkerKind } from "@/systems/session";
import { steerMetaGlyph, steerMetaText } from "@/systems/session/lib/steer-marker";

import { cn } from "@/lib/utils";

/**
 * The one line under an operator's bubble that says how the message reached
 * the turn (VC-07): the glyph is the verb used (corner-down-right for a steer,
 * scissors for interrupt-and-replace, list-plus for the queue), the words are
 * what happened. A superseded steer drops one ink step so nothing disappears
 * silently (US-001.EC-3).
 */
export function SessionUserSteerMeta({
  kind,
  meta,
}: {
  kind: SteerMarkerKind;
  meta?: string | null;
}) {
  const glyph = createElement(steerMetaGlyph(kind), {
    "aria-hidden": true,
    className: "size-3 shrink-0 text-faint",
    strokeWidth: 1.8,
  });
  return (
    <span
      className={cn(
        "flex items-center gap-1.5 px-1 text-transcript-meta",
        kind === "superseded" ? "text-faint" : "text-subtle"
      )}
      data-steer={kind}
      data-testid="user-message-steer-meta"
    >
      {glyph}
      <span>{steerMetaText(kind)}</span>
      {meta ? (
        <>
          <span aria-hidden="true" className="text-faint">
            ·
          </span>
          <span className="tabular-nums">{meta}</span>
        </>
      ) : null}
    </span>
  );
}
