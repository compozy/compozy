import {
  type DataMessagePartProps,
  MessagePrimitive,
  type TextMessagePartProps,
  useAuiState,
} from "@assistant-ui/react";
import type { ReactNode } from "react";

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

import { useSteerProvenance } from "@/systems/session/hooks/use-steer-provenance";

import { sessionSkillInvocationDirectives } from "./session-directive-registry";
import { SessionUserSteerMeta } from "./session-user-steer-meta";
import { SessionDirectiveText } from "./session-directive-text";
import { MessageActions } from "./message-actions";
import { MessageClampToggle } from "./message-clamp-toggle";
import { MESSAGE_CLAMPED_CLASS, useMessageClamp } from "./use-message-clamp";
import { SessionDataEventMarker } from "./session-message-parts";

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
export function UserMessageBubble({
  children,
  subdued = false,
}: {
  children: ReactNode;
  /** Guidance superseded by a later steer: one ink step quieter, never removed (US-001.EC-3). */
  subdued?: boolean;
}) {
  const { contentRef, clampable, clamped, expanded, toggle } = useMessageClamp();

  return (
    <>
      <div
        ref={contentRef}
        data-testid="user-message-bubble"
        data-clamped={clamped || undefined}
        data-subdued={subdued || undefined}
        className={cn(
          "w-fit max-w-full min-w-0 rounded-lg bg-chat-fill-user px-3 py-transcript-message-y",
          "text-transcript-message leading-relaxed [overflow-wrap:anywhere]",
          subdued ? "text-subtle" : "text-fg",
          clamped ? MESSAGE_CLAMPED_CLASS : null
        )}
      >
        {children}
      </div>
      <MessageClampToggle
        clampable={clampable}
        expanded={expanded}
        onToggle={toggle}
        testId="user-message-clamp-toggle"
      />
    </>
  );
}

/**
 * The authored body of a user turn: text with its skill chips and data
 * markers. Attachment refs render in the gallery above, not here. Shared by the
 * operator bubble and the session message card (S1).
 */
export function UserMessageParts() {
  const metadata = useAuiState(state => state.message.metadata);
  const directives = sessionSkillInvocationDirectives(metadata);
  return (
    <MessagePrimitive.Parts>
      {({ part }) => {
        if (part.type === "text") {
          return <SessionTextPart {...part} directives={directives} />;
        }
        if (part.type === "data") {
          if (part.name === SESSION_ATTACHMENT_PART_NAME && isSessionAttachmentRef(part.data)) {
            return null;
          }
          return part.dataRendererUI ?? <SessionDataPart {...part} />;
        }
        return null;
      }}
    </MessagePrimitive.Parts>
  );
}

export function UserMessage() {
  const message = useAuiState(state => state.message);
  const context = useSessionRuntimeRenderContext();
  const attachments = userMessageAttachmentItems(
    message.content,
    message.metadata,
    context?.workspaceId ?? "",
    context?.sessionId ?? ""
  );
  const hasText = userMessageHasText(message.content);
  // How this message reached the turn, bound by the daemon's explicit
  // message_id on its steer markers (VC-07) through this message's authored
  // identity — never by position, text, or the rendered (possibly uniquified) id.
  const steer = useSteerProvenance().forMessage(message);
  const superseded = steer?.kind === "superseded";
  return (
    <MessagePrimitive.Root
      className="group/message flex w-full min-w-0 justify-end pt-1 pb-transcript-turn-gap"
      data-steer={steer?.kind}
    >
      <div className="flex max-w-[80%] min-w-0 flex-col items-end gap-transcript-meta-gap">
        {attachments.length > 0 ? <SessionAttachmentGallery items={attachments} /> : null}
        {hasText ? (
          <UserMessageBubble subdued={superseded}>
            <UserMessageParts />
          </UserMessageBubble>
        ) : null}
        {steer ? (
          <SessionUserSteerMeta
            kind={steer.kind}
            meta={steer.queuePosition === null ? null : `was #${steer.queuePosition}`}
          />
        ) : null}
        <MessageActions align="end" copyLabel="Copy message" testId="user-message-actions" />
      </div>
    </MessagePrimitive.Root>
  );
}
