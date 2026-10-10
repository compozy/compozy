import { CornerDownRight, Reply, Scissors } from "lucide-react";
import type { ReactNode } from "react";

import { SubagentAvatar } from "../subagents/subagent-avatar";
import {
  SessionMessageChip,
  SessionMessageFrame,
  SessionMessagePartyLabel,
} from "./session-message-frame";
import {
  sessionPartyPhrase,
  type SessionMessageOpen,
  type SessionMessageParty,
} from "./session-message-party";

/** How the message reached the turn; queue and direct delivery show no chip (Gap 3). */
export type SessionMessageDelivery = "steered" | "interrupted" | "superseded";

const DELIVERY_CHIP: Record<SessionMessageDelivery, { label: string; icon: ReactNode }> = {
  steered: { label: "Steered", icon: <CornerDownRight aria-hidden="true" /> },
  interrupted: { label: "Interrupted", icon: <Scissors aria-hidden="true" /> },
  superseded: { label: "Superseded", icon: <CornerDownRight aria-hidden="true" /> },
};

export interface SessionMessageCardProps {
  sender: SessionMessageParty;
  delivery: SessionMessageDelivery | null;
  /** The sender asked for this turn's answer (`notify_on_complete`). */
  replyRequested: boolean;
  timestampMs: number | null;
  onOpenSender?: SessionMessageOpen;
  /** Attachments and skill chips, reused from the operator message. */
  attachments?: ReactNode;
  /** The message text. */
  children: ReactNode;
}

/**
 * A user turn another session sent (S1, VC-01): left-aligned and framed, with
 * the sender's provider mark, "From {title}", how it arrived, whether a reply
 * was asked for, and when. The operator's own prompts stay right-aligned bubbles.
 */
export function SessionMessageCard({
  sender,
  delivery,
  replyRequested,
  timestampMs,
  onOpenSender,
  attachments,
  children,
}: SessionMessageCardProps) {
  const chip = delivery ? DELIVERY_CHIP[delivery] : null;
  const superseded = delivery === "superseded";
  const name = sessionPartyPhrase("Message from", sender);
  return (
    <SessionMessageFrame
      aria-label={superseded ? `${name}, superseded` : name}
      aria-busy={sender.pending || undefined}
      data-testid="session-message-card"
      data-delivery={delivery ?? undefined}
      avatar={<SubagentAvatar provider={sender.agentName} surface="rail" />}
      header={
        <>
          <SessionMessagePartyLabel verb="From" party={sender} onOpen={onOpenSender} />
          {chip ? (
            <SessionMessageChip icon={chip.icon} subtle={superseded}>
              {chip.label}
            </SessionMessageChip>
          ) : null}
          {replyRequested ? (
            <SessionMessageChip icon={<Reply aria-hidden="true" />}>
              Reply requested
            </SessionMessageChip>
          ) : null}
        </>
      }
      timestampMs={timestampMs}
      subdued={superseded}
      footer={attachments}
    >
      {children}
    </SessionMessageFrame>
  );
}
