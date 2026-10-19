import { createStoreLogic } from "@xstate/store";

import {
  readSessionMentionRecents,
  withSessionMentionRecent,
  writeSessionMentionRecents,
  type SessionMentionRecent,
} from "@/systems/session";

import type { SessionMentionTrigger } from "./session-mention-model";

interface SessionMentionTriggerState {
  workspaceId: string;
  /** The `@` occurrence before the caret, as the editor last reported it. */
  observed: SessionMentionTrigger | null;
  /** The composer text the dismissed offsets refer to. */
  text: string;
  /** Offsets of the `@` occurrences the operator closed with Esc. */
  dismissedOffsets: readonly number[];
  /** This workspace's recent file/folder mentions, newest first. */
  recents: readonly SessionMentionRecent[];
}

type SessionMentionTriggerEvents = {
  /** `text` is the full composer text; omitted when the editor could not read it. */
  observed: { trigger: SessionMentionTrigger | null; text?: string };
  dismissed: Record<never, never>;
  picked: { recent: SessionMentionRecent };
};

function sameTrigger(a: SessionMentionTrigger | null, b: SessionMentionTrigger | null): boolean {
  return a === b || (a !== null && b !== null && a.offset === b.offset && a.query === b.query);
}

/**
 * Carries dismissed `@` offsets across one text edit. The edit is the span
 * between the longest common prefix and suffix of the two texts: an `@`
 * before it keeps its offset, one after it shifts by the length change, and
 * one inside it was deleted or rewritten, so its dismissal ends.
 */
export function remapDismissedOffsets(
  offsets: readonly number[],
  previous: string,
  next: string
): readonly number[] {
  if (offsets.length === 0 || previous === next) return offsets;
  const limit = Math.min(previous.length, next.length);
  let prefix = 0;
  while (prefix < limit && previous[prefix] === next[prefix]) prefix++;
  let suffix = 0;
  while (
    suffix < limit - prefix &&
    previous[previous.length - 1 - suffix] === next[next.length - 1 - suffix]
  ) {
    suffix++;
  }
  const editEnd = previous.length - suffix;
  const delta = next.length - previous.length;
  const remapped: number[] = [];
  for (const offset of offsets) {
    if (offset < prefix) remapped.push(offset);
    else if (offset >= editEnd) remapped.push(offset + delta);
  }
  return remapped.length === offsets.length && remapped.every((v, i) => v === offsets[i])
    ? offsets
    : remapped;
}

/**
 * Picker interaction state for one composer. A dismissal belongs to one `@`
 * occurrence and lasts as long as that `@` does: typing after it, moving the
 * caret away and back, or editing elsewhere never reopens it; deleting or
 * retyping that `@` ends it, and a new `@` opens the picker again
 * (US-004.AC-1/2/4, VC-10). Picked files and folders become this workspace's
 * recents (browser-local, US-001.AC-3).
 */
export const sessionMentionTriggerLogic = createStoreLogic<
  SessionMentionTriggerState,
  SessionMentionTriggerEvents,
  never,
  { workspaceId: string }
>({
  context: ({ workspaceId }) => ({
    workspaceId,
    observed: null,
    text: "",
    dismissedOffsets: [],
    recents: readSessionMentionRecents(workspaceId),
  }),
  on: {
    observed: (context, { trigger, text }) => {
      const nextText = text ?? context.text;
      const dismissedOffsets = remapDismissedOffsets(
        context.dismissedOffsets,
        context.text,
        nextText
      );
      if (
        sameTrigger(context.observed, trigger) &&
        nextText === context.text &&
        dismissedOffsets === context.dismissedOffsets
      ) {
        return context;
      }
      return { ...context, observed: trigger, text: nextText, dismissedOffsets };
    },
    dismissed: context =>
      context.observed === null || context.dismissedOffsets.includes(context.observed.offset)
        ? context
        : {
            ...context,
            dismissedOffsets: [...context.dismissedOffsets, context.observed.offset],
          },
    picked: (context, { recent }, enqueue) => {
      const recents = withSessionMentionRecent(context.recents, recent);
      enqueue.effect(() => writeSessionMentionRecents(context.workspaceId, recents));
      return { ...context, recents };
    },
  },
});

/** The occurrence the picker should serve, or `null` when none or dismissed. */
export function activeMentionTrigger(
  state: Pick<SessionMentionTriggerState, "observed" | "dismissedOffsets">
): SessionMentionTrigger | null {
  const { observed, dismissedOffsets } = state;
  return observed !== null && !dismissedOffsets.includes(observed.offset) ? observed : null;
}
