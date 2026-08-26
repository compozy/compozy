import {
  type DataMessagePartProps,
  MessagePrimitive,
  type TextMessagePartProps,
  useAuiState,
} from "@assistant-ui/react";
import { type ReactNode, useLayoutEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";
import { SessionAttachmentGallery } from "@/systems/session/components/session-attachment-gallery";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import {
  userMessageAttachmentItems,
  userMessageHasText,
} from "@/systems/session/lib/session-attachment-items";
import {
  isSessionAttachmentRef,
  SESSION_ATTACHMENT_PART_NAME,
} from "@/systems/session/lib/attachment-kinds";

import { readSyntheticTurn } from "@/systems/agent-comms";

import { sessionSkillInvocationDirectives } from "./session-directive-registry";
import { SessionSyntheticTurn } from "./session-synthetic-turn";
import { SessionDirectiveText } from "./session-directive-text";
import { MessageActions } from "./message-actions";
import { SessionDataEventMarker } from "./session-message-parts";

// The bubble clamps with a bottom mask at 176px — text is never truncated, the
// mask lifts on "Show more". Slack beyond the cap avoids flapping on rounding.
const USER_CLAMP_MAX_PX = 44 * 4;
const USER_CLAMP_SLACK_PX = 8;

function SessionTextPart({
  text,
  status,
  directives,
}: TextMessagePartProps & { directives: ReturnType<typeof sessionSkillInvocationDirectives> }) {
  return (
    <SessionDirectiveText
      text={text}
      directives={directives}
      streaming={status?.type === "running"}
    />
  );
}

function SessionDataPart(part: DataMessagePartProps<unknown>) {
  return <SessionDataEventMarker name={part.name} />;
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
          "w-fit max-w-full min-w-0 rounded-lg bg-chat-fill-user px-3 py-transcript-message-y",
          "text-transcript-message leading-relaxed text-fg [overflow-wrap:anywhere]",
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
          className="rounded-xs px-1 text-transcript-caption text-subtle transition-colors duration-base ease-out hover:text-fg"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      ) : null}
    </>
  );
}

export function UserMessage() {
  const message = useAuiState(state => state.message);
  const context = useSessionRuntimeRenderContext();
  const synthetic = readSyntheticTurn(message.metadata);
  // A daemon-authored turn is not the operator speaking, so it must not wear the
  // operator's bubble. It renders in its own grammar, in the same durable place.
  if (synthetic !== null) {
    return (
      <MessagePrimitive.Root className="flex w-full min-w-0 flex-col pt-1 pb-transcript-turn-gap">
        <SessionSyntheticTurn
          synthetic={synthetic}
          content={message.content}
          timestamp={message.createdAt.toISOString()}
        />
      </MessagePrimitive.Root>
    );
  }
  const attachments = userMessageAttachmentItems(
    message.content,
    message.metadata,
    context?.workspaceId ?? "",
    context?.sessionId ?? ""
  );
  const hasText = userMessageHasText(message.content);
  const directives = sessionSkillInvocationDirectives(message.metadata);
  return (
    <MessagePrimitive.Root className="group/message flex w-full min-w-0 justify-end pt-1 pb-transcript-turn-gap">
      <div className="flex max-w-[80%] min-w-0 flex-col items-end gap-transcript-meta-gap">
        {attachments.length > 0 ? <SessionAttachmentGallery items={attachments} /> : null}
        {hasText ? (
          <UserMessageBubble>
            <MessagePrimitive.Parts>
              {({ part }) => {
                if (part.type === "text") {
                  return <SessionTextPart {...part} directives={directives} />;
                }
                if (part.type === "data") {
                  if (
                    part.name === SESSION_ATTACHMENT_PART_NAME &&
                    isSessionAttachmentRef(part.data)
                  ) {
                    return null;
                  }
                  return part.dataRendererUI ?? <SessionDataPart {...part} />;
                }
                return null;
              }}
            </MessagePrimitive.Parts>
          </UserMessageBubble>
        ) : null}
        <MessageActions align="end" copyLabel="Copy message" testId="user-message-actions" />
      </div>
    </MessagePrimitive.Root>
  );
}
