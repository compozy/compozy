import { useEffect, useEffectEvent, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { notifyUser } from "@/lib/user-feedback";
import { subagentRevealStore } from "@/systems/session/lib/subagent-navigation";
import type { SessionSequenceIndex } from "@/systems/session/lib/session-navigation-transcript";
import type { SessionNavigationReveal } from "./session-navigation-target-context";
import type { SessionNavigationHostOptions } from "./use-session-navigation-host";

interface SubagentRevealOptions {
  workspaceId: string;
  sessionId: string;
  ready: boolean;
  readIndex: () => SessionSequenceIndex;
  loadOlder: () => Promise<boolean>;
  scrollToMessage: SessionNavigationHostOptions["scrollToMessage"];
}

function findSubagent(index: SessionSequenceIndex, subagentId: string) {
  for (const entry of index.entries) {
    const partIndex = entry.parts.findIndex(part => {
      if (typeof part !== "object" || part === null) return false;
      const candidate = part as { type?: string; data?: { subagent_id?: string } };
      return (
        candidate.type === "data-compozy-subagent" && candidate.data?.subagent_id === subagentId
      );
    });
    if (partIndex !== -1) return { entry, partIndex };
  }
  return null;
}

/** Reveal through the transcript's paging and scroll owner, including virtualized history. */
export function useSubagentNavigationReveal(options: SubagentRevealOptions) {
  const request = useSelector(subagentRevealStore, snapshot => {
    const pending = snapshot.context.request;
    return pending?.sessionId === options.sessionId && pending.workspaceId === options.workspaceId
      ? pending
      : null;
  });
  const [reveal, setReveal] = useState<SessionNavigationReveal | null>(null);
  const [released, setReleased] = useState<ReadonlySet<string>>(new Set());
  const perform = useEffectEvent(
    async (target: NonNullable<typeof request>, canceled: () => boolean) => {
      try {
        let match = findSubagent(options.readIndex(), target.subagentId);
        for (let page = 0; !match && page < 500 && !canceled(); page += 1) {
          if (!(await options.loadOlder())) break;
          match = findSubagent(options.readIndex(), target.subagentId);
        }
        if (canceled()) return;
        if (!match) throw new Error("Subagent activity is not available in this transcript.");
        setReleased(new Set());
        setReveal({
          key: target.key,
          messageId: match.entry.messageId,
          turnId: match.entry.turnId,
          partIndex: match.partIndex,
          subagentId: target.subagentId,
          field: null,
          toolCallId: null,
          opensBody: true,
        });
        await new Promise<void>(resolve =>
          requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
        );
        if (canceled()) return;
        let card: HTMLElement | null = null;
        const landed = await options.scrollToMessage(match.entry.messageId, 48, row => {
          card = row.querySelector<HTMLElement>(
            `[data-subagent-id="${CSS.escape(target.subagentId)}"]`
          );
          if (!card) return null;
          const range = document.createRange();
          range.selectNodeContents(card);
          return range;
        });
        if (canceled()) return;
        if (!landed || !card) throw new Error("Subagent activity did not come into view.");
        // A short token-colored focus tint; no new motion or visual vocabulary.
        (card as HTMLElement).animate?.(
          [
            { backgroundColor: "var(--color-selected)" },
            { backgroundColor: "var(--color-selected)" },
          ],
          { duration: 1400 }
        );
      } catch (error) {
        if (!canceled())
          notifyUser({
            message: error instanceof Error ? error.message : "Couldn't show subagent activity.",
            tone: "error",
          });
      } finally {
        if (!canceled()) {
          subagentRevealStore.trigger.settled({ key: target.key });
        }
      }
    }
  );
  useEffect(() => {
    if (!request || !options.ready) return;
    let disposed = false;
    // Wait for the destination viewport to mount before opening and measuring its rows.
    const frame = requestAnimationFrame(() => {
      void perform(
        request,
        () => disposed || subagentRevealStore.getSnapshot().context.request?.key !== request.key
      );
    });
    return () => {
      disposed = true;
      cancelAnimationFrame(frame);
    };
  }, [request, options.ready]);
  return {
    reveal,
    released,
    jumpActive: request !== null,
    clear: () => {
      if (request) subagentRevealStore.trigger.settled({ key: request.key });
      setReveal(null);
    },
    releaseDisclosure: (id: string) => setReleased(previous => new Set([...previous, id])),
  };
}
