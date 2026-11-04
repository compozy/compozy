import { cn } from "@/lib/utils";

import { sessionBadgeSignal } from "../../lib/session-badge";
import { SESSION_TONE_WORD_CLASS } from "../../lib/session-badge-classes";
import type { RowMeta } from "../../lib/session-row-meta";
import type { SessionPayload } from "../../types";

/**
 * The visible who line's phrase: the model, or, while the session needs you, the
 * words that say why in its place. Notes follow and give way first when the line is
 * full. Each separator lives inside the neighbour that gives way first, so a
 * collapsed part never leaves a stray `·`. The words repeat the target's accessible
 * name, so the line itself stays out of the a11y tree.
 */
export function SessionListRowWho({
  model,
  nextModel = "",
  session,
  meta,
}: {
  model: string;
  /** The model the next message switches to; drawn after an arrow while a switch is pending. */
  nextModel?: string;
  session: Pick<SessionPayload, "badge">;
  meta: Pick<RowMeta, "shown" | "notes">;
}) {
  const { tone } = sessionBadgeSignal(session.badge);
  const parts: { key: string; kind: "model" | "next" | "attention" | "note"; text: string }[] = [];
  if (meta.shown.length > 0) {
    meta.shown.forEach((text, index) =>
      parts.push({ key: `attention-${index}`, kind: "attention", text })
    );
  } else if (model) {
    parts.push({ key: "model", kind: "model", text: model });
    if (nextModel) parts.push({ key: "next", kind: "next", text: nextModel });
  }
  meta.notes.forEach((text, index) => parts.push({ key: `note-${index}`, kind: "note", text }));
  return (
    <>
      {parts.map((part, index) => {
        const next = parts[index + 1];
        const leading = index > 0 && part.kind === "note";
        const trailing = next !== undefined && next.kind !== "note" && next.kind !== "next";
        return (
          <span
            key={part.key}
            aria-hidden="true"
            className={cn(
              "min-w-0 truncate",
              part.kind === "model" && "shrink-[100] text-fg-2",
              part.kind === "next" && "shrink-[100] text-muted-foreground",
              part.kind === "attention" && "shrink text-fg-3",
              part.kind === "attention" &&
                index === 0 &&
                tone !== "neutral" &&
                SESSION_TONE_WORD_CLASS[tone],
              part.kind === "note" && "shrink-[10000]"
            )}
          >
            {leading ? <span className="text-faint">· </span> : null}
            {part.kind === "next" ? <span className="text-faint">→ </span> : null}
            {part.text}
            {trailing ? <span className="text-faint"> ·</span> : null}
          </span>
        );
      })}
    </>
  );
}
