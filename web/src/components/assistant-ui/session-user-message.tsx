import {
  type DataMessagePartProps,
  MessagePrimitive,
  type TextMessagePartProps,
} from "@assistant-ui/react";
import { type ReactNode, useLayoutEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";
import { MessageActions } from "./message-actions";
import { SessionDataEventCard, SessionMessageText } from "./session-message-parts";

// The bubble clamps with a bottom mask at 176px — text is never truncated, the
// mask lifts on "Show more". Slack beyond the cap avoids flapping on rounding.
const USER_CLAMP_MAX_PX = 176;
const USER_CLAMP_SLACK_PX = 8;

function SessionTextPart({ text, state }: { text: string; state?: { type: string } }) {
  return <SessionMessageText text={text} streaming={state?.type === "running"} />;
}

function SessionDataPart(part: DataMessagePartProps<unknown>) {
  return <SessionDataEventCard name={part.name} data={part.data} />;
}

/**
 * The one message surface in the transcript: a right-aligned, borderless block
 * on the 4.5% ink wash. No avatar, no role label, no shadow. Long messages
 * clamp behind a fade mask with a quiet "Show more" toggle.
 */
function UserMessageBubble({ children }: { children: ReactNode }) {
  const contentRef = useRef<HTMLDivElement | null>(null);
  const [clampable, setClampable] = useState(false);
  const [expanded, setExpanded] = useState(false);

  useLayoutEffect(() => {
    const node = contentRef.current;
    if (!node) return;
    const measure = () => {
      setClampable(node.scrollHeight > USER_CLAMP_MAX_PX + USER_CLAMP_SLACK_PX);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const clamped = clampable && !expanded;

  return (
    <>
      <div
        ref={contentRef}
        data-testid="user-message-bubble"
        data-clamped={clamped || undefined}
        className={cn(
          "w-fit max-w-full min-w-0 rounded-lg bg-chat-fill-user px-3 py-[7px]",
          "text-[13.5px] leading-relaxed text-fg [overflow-wrap:anywhere]",
          clamped
            ? "max-h-44 overflow-hidden [mask-image:linear-gradient(to_bottom,#000_calc(100%-28px),transparent)]"
            : null
        )}
      >
        {children}
      </div>
      {clampable ? (
        <button
          type="button"
          data-testid="user-message-clamp-toggle"
          aria-expanded={expanded}
          onClick={() => setExpanded(value => !value)}
          className="rounded-xs px-1 text-[11px] text-subtle transition-colors duration-base ease-out hover:text-fg"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      ) : null}
    </>
  );
}

export function UserMessage() {
  return (
    <MessagePrimitive.Root className="group/message flex w-full min-w-0 justify-end pt-1 pb-[18px]">
      <div className="flex max-w-[80%] min-w-0 flex-col items-end gap-[3px]">
        <UserMessageBubble>
          <MessagePrimitive.Parts
            components={{
              Text: ({ text, status }: TextMessagePartProps) => (
                <SessionTextPart text={text} state={status} />
              ),
              data: {
                Fallback: SessionDataPart,
              },
            }}
          />
        </UserMessageBubble>
        <MessageActions align="end" copyLabel="Copy message" testId="user-message-actions" />
      </div>
    </MessagePrimitive.Root>
  );
}
