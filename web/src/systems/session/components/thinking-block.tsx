import { memo, useState } from "react";
import { Brain, ChevronRight } from "lucide-react";

import { cn } from "@/lib/utils";
import { MessageMarkdown } from "@/systems/session/components/message-markdown";
import { Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow, TypingDots } from "@agh/ui";

export interface ThinkingBlockProps {
  thinking: string;
  thinkingComplete?: boolean;
  /** How many reasoning updates are grouped into this block; shows "N updates" when > 1. */
  updateCount?: number;
}

/**
 * `ThinkingBlock` renders one (possibly grouped) span of agent reasoning as a
 * muted collapsible row in the same visual family as `ToolCallRow`: a `size-5`
 * icon well + `size-3.5` Brain glyph, a quiet label, an optional "N updates"
 * count, and a `size-3` chevron, expanding to an indented left-rule region
 * (`ml-7 border-l border-line pl-3`) that renders the reasoning as markdown.
 *
 * It auto-opens while the turn streams and auto-collapses once settled, unless
 * the operator has toggled it (a user toggle pins the state either way).
 */
export const ThinkingBlock = memo(
  function ThinkingBlock({ thinking, thinkingComplete, updateCount = 1 }: ThinkingBlockProps) {
    const [userOpen, setUserOpen] = useState<boolean | null>(null);
    const open = userOpen ?? !thinkingComplete;
    const label = thinkingComplete ? "Thought process" : "Thinking";

    return (
      <Collapsible open={open} onOpenChange={setUserOpen}>
        <CollapsibleTrigger
          className={cn(
            "group/reasoning flex min-h-6 w-full min-w-0 cursor-pointer items-center gap-1.5",
            "rounded-sm px-1 text-left",
            "transition-colors duration-base ease-out hover:bg-hover",
            "focus-visible:outline-none focus-visible:shadow-focus-ring"
          )}
          data-testid="thinking-trigger"
        >
          <span className="flex size-5 shrink-0 items-center justify-center">
            <Brain
              className="size-3.5 shrink-0 text-subtle transition-colors group-hover/reasoning:text-muted"
              strokeWidth={1.75}
              aria-hidden="true"
            />
          </span>
          <span className="flex min-w-0 flex-1 items-center gap-1.5">
            <span className="truncate text-form-label font-medium text-muted transition-colors group-hover/reasoning:text-fg">
              {label}
            </span>
            {thinkingComplete ? null : <TypingDots className="shrink-0" />}
          </span>
          <span className="flex shrink-0 items-center gap-1.5 text-subtle">
            {updateCount > 1 ? (
              <Eyebrow className="text-muted">{updateCount} updates</Eyebrow>
            ) : null}
            <ChevronRight
              className={cn(
                "size-3 shrink-0 text-subtle transition-transform duration-base ease-out",
                "motion-reduce:transition-none",
                open ? "rotate-90 text-muted" : null
              )}
              strokeWidth={1.75}
              aria-hidden="true"
            />
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div
            className={cn(
              "mt-1 ml-7 max-h-60 overflow-y-auto border-l border-line pl-3",
              "select-text text-form-input text-muted"
            )}
            data-testid="thinking-content"
          >
            <MessageMarkdown content={thinking} streaming={!thinkingComplete} />
          </div>
        </CollapsibleContent>
      </Collapsible>
    );
  },
  (prev, next) =>
    prev.thinking === next.thinking &&
    prev.thinkingComplete === next.thinkingComplete &&
    prev.updateCount === next.updateCount
);
