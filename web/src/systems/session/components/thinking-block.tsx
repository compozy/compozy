import { useState } from "react";
import { Brain, ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";
import { MessageMarkdown } from "@/systems/session/components/message-markdown";

export interface ThinkingBlockProps {
  thinking: string;
  thinkingComplete?: boolean;
}

/** Collapsed-row preview: the first non-empty reasoning line, trimmed. */
function firstThinkingLine(thinking: string): string {
  for (const line of thinking.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return "";
}

/**
 * Agent reasoning in the calm-transcript grammar. Live, it is a shimmering
 * "Thinking…" text label with the streaming body auto-opened beneath it;
 * settled, it collapses to a tool-row line — "Thought" verb + first-line
 * preview + chevron — expanding to the indented muted body. A user toggle pins
 * the state either way (auto-open while live, auto-collapse on settle).
 */
export function ThinkingBlock({ thinking, thinkingComplete }: ThinkingBlockProps) {
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const live = !thinkingComplete;
  const open = userOpen ?? live;
  const preview = firstThinkingLine(thinking);

  return (
    <div
      data-testid="thinking-block"
      data-live={live || undefined}
      className="flex min-w-0 flex-col"
    >
      <button
        type="button"
        data-testid="thinking-trigger"
        aria-expanded={open}
        onClick={() => setUserOpen(!open)}
        className={cn(
          live
            ? "w-fit rounded-sm px-1 py-0.5 text-left"
            : [
                "group/thinking flex min-h-transcript-line w-full min-w-0 cursor-pointer items-center gap-transcript-inline-gap",
                "rounded-sm px-1 text-left transition-colors duration-base ease-out hover:bg-hover",
              ],
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        {live ? (
          <span className="session-shimmer text-small-body font-medium">Thinking…</span>
        ) : (
          <>
            <span className="flex size-transcript-icon-well shrink-0 items-center justify-center">
              <Brain aria-hidden="true" className="size-3 shrink-0 text-subtle" strokeWidth={1.8} />
            </span>
            <span className="flex min-w-0 flex-1 items-baseline gap-transcript-inline-gap text-small-body">
              <span className="shrink-0 font-medium text-muted transition-colors group-hover/thinking:text-fg">
                Thought
              </span>
              {preview ? (
                <span className="min-w-0 flex-1 truncate text-subtle">{preview}</span>
              ) : null}
            </span>
            <ChevronDown
              aria-hidden="true"
              className={cn(
                "size-3 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
                open ? "rotate-180" : null
              )}
              strokeWidth={1.75}
            />
          </>
        )}
      </button>
      {open ? (
        <div
          aria-label="Reasoning"
          data-slot="thinking-detail"
          data-testid="thinking-content"
          role="region"
          tabIndex={0}
          className={cn(
            "mt-0.5 mb-1 ml-transcript-detail-indent max-h-60 overflow-y-auto border-l border-line pl-transcript-detail-gutter",
            "text-transcript-body leading-relaxed text-muted select-text",
            "focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
        >
          <MessageMarkdown content={thinking} streaming={live} />
        </div>
      ) : null}
    </div>
  );
}
