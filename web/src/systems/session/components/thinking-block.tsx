import { useState } from "react";
import { Brain } from "@compozy/ui/icons";

import { TranscriptDisclosure } from "@compozy/ui";

import { compactSessionSummary } from "../lib/session-summary";

import { cn } from "@/lib/utils";
import { MessageMarkdown } from "@/systems/session/components/message-markdown";

export interface ThinkingBlockProps {
  thinking: string;
  thinkingComplete?: boolean;
  /** An expanded work group already discloses its reasoning; retain the body on settle. */
  defaultOpen?: boolean;
  /** The projected part this row renders (`data-part-index`), so find can land on it. */
  partIndex?: number;
  /** A find jump needs the reasoning open; layered over the reader's own toggle. */
  revealOpen?: boolean;
  /** The reader closed a body a jump held open: the hold is theirs to drop. */
  onRevealRelease?: () => void;
}

/** A paragraph as one plain line: markdown marks stripped. */
function plainParagraph(paragraph: string): string {
  return paragraph
    .replace(/^\s{0,3}(#{1,6}|[-*+]|\d+\.)\s+/gmu, "")
    .replace(/[*_~`]/gu, "")
    .replace(/\[([^\]]*)\]\([^)]*\)/gu, "$1")
    .trim();
}

/**
 * Plain paragraphs of the reasoning. Code fences (even one still streaming
 * open) drop out of the whole text first, so a blank line inside a code block
 * never splits it into paragraphs that read as prose.
 */
function paragraphs(thinking: string): string[] {
  return thinking
    .replace(/```[\s\S]*?(?:```|$)/gu, "\n\n")
    .split(/\n\s*\n/u)
    .map(plainParagraph)
    .filter(paragraph => paragraph.length > 0);
}

/** Settled preview: the first paragraph. Live line: the latest one, what it is thinking now. */
function thinkingSummary(thinking: string, live: boolean): string {
  const all = paragraphs(thinking);
  return compactSessionSummary((live ? all.at(-1) : all[0]) ?? "");
}

/**
 * Agent reasoning in the calm-transcript grammar. Live, it is one line — the
 * latest paragraph it is thinking, breathing in the subtle ink (a shimmering
 * "Thinking…" before any text arrives); settled, it collapses to a tool-row
 * line — "Thought" + first-paragraph preview — expanding to the indented,
 * quieter-than-answer body. The body opens on its own only inside a disclosed
 * work group; a user toggle pins the state either way.
 */
export function ThinkingBlock({
  thinking,
  thinkingComplete,
  defaultOpen = false,
  partIndex,
  revealOpen = false,
  onRevealRelease,
}: ThinkingBlockProps) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const live = !thinkingComplete;
  // Live reasoning rests as one line of what it is thinking now; its body opens
  // on its own only inside a work group the reader already disclosed.
  const ownOpen = userOpen ?? defaultOpen;
  const open = ownOpen || revealOpen;
  const preview = thinkingSummary(thinking, live);
  const toggle = () => {
    if (open && revealOpen) onRevealRelease?.();
    setUserOpen(!open);
  };

  return (
    <div
      data-testid="thinking-block"
      data-live={live || undefined}
      data-part-index={partIndex}
      className="flex min-w-0 flex-col"
    >
      {live ? (
        <button
          type="button"
          data-testid="thinking-trigger"
          aria-expanded={open}
          onClick={toggle}
          className="flex w-full min-w-0 items-center rounded-sm px-1 py-0.5 text-left focus-visible:shadow-focus-ring focus-visible:outline-none"
        >
          {preview ? (
            <span className="session-thinking-pulse min-w-0 truncate text-small-body text-subtle">
              {preview}
            </span>
          ) : (
            <span className="session-shimmer text-small-body">Thinking…</span>
          )}
        </button>
      ) : (
        <TranscriptDisclosure
          className="w-full min-w-0"
          data-testid="thinking-trigger"
          expanded={open}
          icon={<Brain aria-hidden="true" className="size-3 shrink-0 text-subtle" />}
          label={
            <>
              Thought
              {preview ? (
                <span className="ml-transcript-inline-gap text-faint">{preview}</span>
              ) : null}
            </>
          }
          onToggle={toggle}
        />
      )}
      {open ? (
        <div
          aria-label="Reasoning"
          data-slot="thinking-detail"
          data-testid="thinking-content"
          role="region"
          tabIndex={0}
          className={cn(
            "mt-0.5 mb-1 ml-transcript-detail-indent max-h-60 overflow-y-auto border-l border-line pl-transcript-detail-gutter",
            "text-transcript-body leading-relaxed text-subtle select-text",
            "[&_p]:text-fg/50 [&_li]:text-fg/50 [&_strong]:text-fg/60",
            "focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
        >
          <MessageMarkdown content={thinking} streaming={live} compact="relaxed" />
        </div>
      ) : null}
    </div>
  );
}
