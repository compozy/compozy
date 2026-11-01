import { cn } from "@/lib/utils";

import { sessionBadgeSignal } from "../../lib/session-badge";
import { SESSION_TONE_WORD_CLASS } from "../../lib/session-badge-classes";
import type { RowMeta } from "../../lib/session-row-meta";
import type { SessionPayload } from "../../types";

/**
 * The visible who line. Truncation order when the line is full: notes give way
 * first, then the agent name, and attention words only as a last resort. Each
 * separator lives inside the neighbour that gives way first, so a collapsed part
 * never leaves a stray `·`. The words repeat the target's accessible name, so the
 * line itself stays out of the a11y tree.
 *
 * Tone runs title (the row's `fg`) > attention word > agent > notes and the
 * row's counts, chevron and time. `muted`, `subtle` and `faint` are one value in
 * the light theme, so the agent and a neutral word take the steps above them.
 */
export function SessionListRowWho({
  session,
  meta,
}: {
  session: Pick<SessionPayload, "agent_name" | "badge">;
  meta: Pick<RowMeta, "attention" | "notes">;
}) {
  const { tone } = sessionBadgeSignal(session.badge);
  const parts: { key: string; kind: "agent" | "attention" | "note"; text: string }[] = [];
  if (session.agent_name) parts.push({ key: "agent", kind: "agent", text: session.agent_name });
  meta.attention.forEach((text, index) =>
    parts.push({ key: `attention-${index}`, kind: "attention", text })
  );
  meta.notes.forEach((text, index) => parts.push({ key: `note-${index}`, kind: "note", text }));
  return (
    <>
      {parts.map((part, index) => {
        const next = parts[index + 1];
        const leading = index > 0 && part.kind === "note";
        const trailing = next !== undefined && next.kind !== "note";
        return (
          <span
            key={part.key}
            aria-hidden="true"
            className={cn(
              "min-w-0 truncate",
              part.kind === "agent" && "shrink-[100] text-fg-2",
              part.kind === "attention" && "shrink text-fg-3",
              part.kind === "attention" &&
                index === firstAttention(parts) &&
                tone !== "neutral" &&
                SESSION_TONE_WORD_CLASS[tone],
              part.kind === "note" && "shrink-[10000]"
            )}
          >
            {leading ? <span className="text-faint">· </span> : null}
            {part.text}
            {trailing ? <span className="text-faint"> ·</span> : null}
          </span>
        );
      })}
    </>
  );
}

function firstAttention(parts: { kind: string }[]): number {
  return parts.findIndex(part => part.kind === "attention");
}
