/**
 * Send a message to a call's child.
 *
 * Messaging a parked child is one of the two ways to revive it — the other is
 * calling it again — which is why no Revive control exists anywhere in this
 * feature. Using the child *is* the revival.
 *
 * Every failure this form can hit is a typed, deterministic condition rather
 * than a generic error, so each one gets a plain first line that names the
 * recovery and the daemon's own code in mono after it. The operator can search
 * for that code in the docs, the CLI, or a log and find the same thing.
 */
import { Send } from "lucide-react";

import { ActionResultBanner, Button, Eyebrow, Panel, Textarea } from "@compozy/ui";

import { AGENT_COMMS_ERROR_CODES, type AgentCommsErrorCode } from "../adapters/agent-comms-api";

/** Plain-language recovery per typed refusal. The code renders beside it. */
const SEND_FAILURE_COPY: Partial<Record<AgentCommsErrorCode, string>> = {
  message_target_blocked:
    "That helper is waiting on a decision from you. Answer it on its own screen first — a message cannot approve a pending permission.",
  message_rate_limited: "Too many messages in the last minute. Try again shortly.",
  message_duplicate: "That exact message just went out. The original is already on its way.",
  message_too_large: "That message is longer than the runtime accepts. Shorten it and send again.",
  message_pending_cap:
    "That helper already has as many undelivered messages as it can hold. Wait for it to work through them.",
  call_target_expired:
    "That helper sat idle past its limit and left. Ask the agent again to start a fresh one.",
  call_target_denied: "That helper is outside your lineage, so you cannot message it.",
  call_workspace_denied: "That helper belongs to another workspace.",
};

const FALLBACK_FAILURE_COPY = "The message did not go out.";

function failureCopy(code: string | null): string {
  if (code === null) return FALLBACK_FAILURE_COPY;
  const known = (AGENT_COMMS_ERROR_CODES as readonly string[]).includes(code)
    ? SEND_FAILURE_COPY[code as AgentCommsErrorCode]
    : undefined;
  return known ?? FALLBACK_FAILURE_COPY;
}

export interface AgentComposeMessageProps {
  /** Who the message goes to, for the heading. */
  targetLabel: string;
  value: string;
  onChange: (next: string) => void;
  onSend: () => void;
  pending?: boolean;
  /** The daemon's code from a rejected send, or null. */
  failureCode?: string | null;
  /** The receipt from an accepted send — shown until the operator writes again. */
  accepted?: { messageId: string; delivery: string } | null;
  "data-testid"?: string;
}

export function AgentComposeMessage({
  targetLabel,
  value,
  onChange,
  onSend,
  pending = false,
  failureCode = null,
  accepted = null,
  "data-testid": testId,
}: AgentComposeMessageProps) {
  const empty = value.trim().length === 0;
  return (
    <Panel
      data-testid={testId}
      title={<Eyebrow>Message {targetLabel}</Eyebrow>}
      foot={
        <span className="flex w-full items-center gap-2">
          <span className="text-form text-muted">
            Writing to a resting helper is what wakes it.
          </span>
          <span className="flex-1" />
          <Button
            size="sm"
            type="button"
            disabled={empty || pending}
            onClick={onSend}
            data-testid="agent-compose-message-send"
          >
            <Send aria-hidden="true" />
            Send
          </Button>
        </span>
      }
    >
      <Textarea
        aria-label={`Message to ${targetLabel}`}
        rows={2}
        value={value}
        disabled={pending}
        onChange={event => onChange(event.target.value)}
        data-testid="agent-compose-message-input"
      />
      {failureCode ? (
        <ActionResultBanner
          className="mt-2"
          tone="danger"
          data-testid="agent-compose-message-error"
          title={failureCopy(failureCode)}
          description={<span className="font-mono text-form">{failureCode}</span>}
        />
      ) : null}
      {accepted ? (
        <ActionResultBanner
          className="mt-2"
          tone="success"
          data-testid="agent-compose-message-accepted"
          title="Message accepted."
          description={
            <span className="font-mono text-form">
              {accepted.messageId} · {accepted.delivery}
            </span>
          }
        />
      ) : null}
    </Panel>
  );
}
