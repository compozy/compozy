/**
 * S11: asks from sessions this agent started, on its main session. Answering here is the same as
 * answering there: decisions go to the original session and request.
 */
import { Check, ShieldQuestion, X } from "@compozy/ui/icons";
import { useState } from "react";

import { AgentAvatar, Marker, Receipt } from "@compozy/ui";

import type { AgentInstancePayload } from "@/systems/agent";
import { PROFILE_SPRITE_URL } from "@/systems/profiles";
import { terminalPermissionDetail } from "@/systems/terminal/parts";

import { usePermissionDock } from "../../../hooks/use-permission-dock";
import { useRelayedAsks } from "../../../hooks/use-relayed-asks";
import { useSession } from "../../../hooks/use-sessions";
import { countWord } from "../../../lib/session-away";
import type { PermissionRequest, SessionPendingInteraction } from "../../../types";
import { PermissionDockView } from "../../permission-dock-view";
import { reportAgent } from "../../../lib/session-report-identity";

function relayedRequest(ask: SessionPendingInteraction): PermissionRequest {
  return {
    requestId: ask.provider_request_id,
    ...(ask.tool_id ? { toolId: ask.tool_id } : {}),
    toolName: ask.tool_id || ask.title || "unknown",
    toolInput: {},
    action: "",
    resource: "",
    supportedDecisions: ask.decisions,
    turnId: ask.turn_id,
  };
}

function waitsMore(expiresAt: string | undefined, now: number) {
  const left = expiresAt ? Date.parse(expiresAt) - now : Number.NaN;
  if (!Number.isFinite(left) || left <= 0) return null;
  const minutes = Math.ceil(left / 60_000);
  const hours = Math.floor(minutes / 60);
  return hours > 0 ? `waits ${hours}h ${minutes % 60}m more` : `waits ${minutes}m more`;
}

function RelayedAsk({
  ask,
  countLabel,
  instances,
  onDecided,
  workspaceId,
}: {
  ask: SessionPendingInteraction;
  countLabel: string | null;
  instances: readonly AgentInstancePayload[];
  onDecided: (receipt: { allowed: boolean; subject: string; agent: string }) => void;
  workspaceId: string;
}) {
  const permission = relayedRequest(ask);
  const { data: session } = useSession(ask.session_id);
  const name = session?.agent_name ?? "";
  const agent = reportAgent(
    instances.find(candidate => candidate.agent_name === name),
    name || "An agent"
  );
  const [now] = useState(() => Date.now());
  const subject = ask.title ?? null;
  const { decide, decisionOptions, isResolved, isSubmitting } = usePermissionDock({
    permission,
    sessionId: ask.session_id,
    workspaceId,
  });
  if (isResolved) return null;
  const wait = waitsMore(ask.expires_at ?? undefined, now);
  const where = session?.name?.trim() || "another session";
  return (
    <PermissionDockView
      agentName={agent.label}
      asker={
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          provider={agent.provider}
          quietIdle
          size={32}
          spriteUrl={PROFILE_SPRITE_URL}
          surfaceClassName="ring-card"
        />
      }
      countLabel={countLabel}
      decide={decision => {
        decide(decision);
        onDecided({
          allowed: decision.startsWith("allow"),
          subject: subject ?? "this request",
          agent: agent.label,
        });
      }}
      decisionOptions={decisionOptions}
      isSubmitting={isSubmitting}
      origin={
        <span className="truncate text-small-body text-muted-foreground">
          In {where}
          {wait ? ` · ${wait}` : ""}
        </span>
      }
      permission={permission}
      subject={subject}
      terminalDetail={terminalPermissionDetail(permission.toolName, permission.toolInput)}
    />
  );
}

export function RelayedApprovalDock({
  sessionId,
  workspaceId,
}: {
  sessionId: string;
  workspaceId: string;
}) {
  const { asks, instances } = useRelayedAsks(sessionId, workspaceId);
  const [answered, setAnswered] = useState<
    readonly { id: string; allowed: boolean; subject: string; agent: string }[]
  >([]);
  const waiting = asks.filter(
    (ask: SessionPendingInteraction) => !answered.some(row => row.id === ask.interaction_id)
  );
  const ask = waiting[0];
  if (!ask && answered.length === 0) return null;
  const position = answered.length + 1;
  const total = answered.length + waiting.length;
  return (
    <div className="flex flex-col gap-2">
      {answered.map(row => (
        <Receipt
          icon={row.allowed ? <Check aria-hidden="true" /> : <X aria-hidden="true" />}
          key={row.id}
          tone={row.allowed ? "allowed" : "rejected"}
        >
          {row.allowed ? "You allowed" : "You declined"} <code>{row.subject}</code> for{" "}
          <b>{row.agent}</b>
        </Receipt>
      ))}
      {ask ? (
        <RelayedAsk
          ask={ask}
          countLabel={total > 1 ? `${position} of ${total}` : null}
          instances={instances}
          key={ask.interaction_id}
          onDecided={receipt =>
            setAnswered(rows => [...rows, { id: ask.interaction_id, ...receipt }])
          }
          workspaceId={workspaceId}
        />
      ) : null}
    </div>
  );
}

export function RelayedWaitingMarker({
  sessionId,
  workspaceId,
}: {
  sessionId: string;
  workspaceId: string;
}) {
  const { asks, instance } = useRelayedAsks(sessionId, workspaceId);
  if (!instance || asks.length === 0) return null;
  const many = asks.length > 1;
  return (
    <Marker icon={<ShieldQuestion aria-hidden="true" className="size-3" />}>
      <b>
        {countWord(asks.length)} {many ? "requests" : "request"}
      </b>{" "}
      {many ? "are" : "is"} waiting for you, from {many ? "sessions" : "a session"} {instance.label}{" "}
      started. Nothing moves until you answer.
    </Marker>
  );
}
