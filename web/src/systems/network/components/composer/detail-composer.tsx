import {
  useSendNetworkMessage,
  type SendNetworkMessageInput,
} from "../../hooks/use-network-actions";
import { Composer } from "./composer";

export interface DetailComposerThreadProps {
  workspaceId?: string;
  surface: "thread";
  channel: string;
  threadId: string;
  sessionId: string;
  peerFrom: string;
  displayName?: string;
  disabledReason?: string;
}

export interface DetailComposerDirectProps {
  workspaceId?: string;
  surface: "direct";
  channel: string;
  directId: string;
  sessionId: string;
  peerFrom: string;
  peerTo?: string;
  /** Other party label for the placeholder + Send target hint. */
  peerLabel?: string;
  displayName?: string;
  disabledReason?: string;
}

export type DetailComposerProps = DetailComposerThreadProps | DetailComposerDirectProps;

function buildSendInput(
  props: DetailComposerProps,
  text: string,
  mentions: string[] = []
): SendNetworkMessageInput {
  if (props.surface === "thread") {
    return {
      surface: "thread",
      channel: props.channel,
      threadId: props.threadId,
      sessionId: props.sessionId,
      peerFrom: props.peerFrom,
      text,
      mentions,
      displayName: props.displayName,
    };
  }
  return {
    surface: "direct",
    channel: props.channel,
    directId: props.directId,
    sessionId: props.sessionId,
    peerFrom: props.peerFrom,
    peerTo: props.peerTo,
    text,
    mentions,
    displayName: props.displayName,
  };
}

export function DetailComposer(props: DetailComposerProps) {
  const { send, isSending } = useSendNetworkMessage({ workspaceId: props.workspaceId });
  const disabled = props.disabledReason != null;

  const placeholder =
    props.surface === "thread" ? "Reply..." : `Message ${props.peerLabel ?? "@peer"}...`;
  const sendLabel =
    props.surface === "thread"
      ? `Send to #${props.channel}`
      : `Send to ${props.peerLabel ?? "@peer"}`;
  const hint =
    props.surface === "thread"
      ? "Replies deliver to thread subscribers; muted sessions skip the wake."
      : "Direct rooms are restricted bilateral conversations.";

  const handleSubmit = async ({
    text,
    mentions,
    reset,
  }: {
    text: string;
    mentions: string[];
    reset: () => void;
  }) => {
    try {
      await send(buildSendInput(props, text, mentions));
      reset();
    } catch {
      // The optimistic message remains visible with retry/discard inline per
      // `_design.md` §7.3.
      reset();
    }
  };

  return (
    <Composer
      disabled={disabled}
      disabledReason={props.disabledReason}
      hint={hint}
      isSending={isSending}
      onSubmit={handleSubmit}
      placeholder={placeholder}
      sendLabel={sendLabel}
      testIdSuffix={props.surface === "thread" ? "thread" : "direct"}
    />
  );
}
