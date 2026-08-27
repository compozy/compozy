/**
 * Who was asked, by whom, and how it ended — plus the controls that exist.
 *
 * The control row is the truthful-UI rule made concrete: each button is rendered
 * only when its operation is available right now. Cancel is present while the
 * call is in flight and gone the moment it settles; call-again and message
 * appear only afterwards. The one disabled state is a cancel already in flight,
 * which prevents a duplicate mutation while preserving the operation in place.
 *
 * Timer chrome follows the same rule. Most calls carry no deadline at all, so
 * most calls show none. The idle TTL states its own physics instead of a bare
 * countdown, because "suspended while running" is the fact that stops an
 * operator worrying about a clock that is not running.
 */
import { Ban, CornerUpRight, Mail } from "lucide-react";

import { Button, MetadataList, MonoId, OwnerAvatar, Pill, cn } from "@compozy/ui";

import { AgentCallLiveness, AgentCallStatePill } from "./agent-call-state-pill";
import { formatIdleTtlCopy, formatSettledDuration } from "../lib/call-clock-format";
import type { CallDetailView } from "../lib/call-detail-view-model";
import { CALL_VERDICT_SIGNAL } from "../lib/call-state";

export interface AgentCallDetailHeaderProps extends React.ComponentProps<"header"> {
  view: CallDetailView;
  onCancel?: () => void;
  onCallAgain?: () => void;
  onMessageChild?: () => void;
  onOpenCaller?: () => void;
  onOpenChildSession?: () => void;
  cancelPending?: boolean;
}

export function AgentCallDetailHeader({
  view,
  onCancel,
  onCallAgain,
  onMessageChild,
  onOpenCaller,
  onOpenChildSession,
  cancelPending = false,
  className,
  ...props
}: AgentCallDetailHeaderProps) {
  const title = view.agentName ?? view.callId;
  const ownerId = view.agentName ?? view.callId;
  const idleCopy = formatIdleTtlCopy(view.idleTtl);
  const settledDuration = view.settledAt
    ? formatSettledDuration(view.createdAt, view.settledAt)
    : null;
  return (
    <header {...props} className={cn("border-b border-line-soft pb-3", className)}>
      <div className="flex flex-wrap items-center gap-2">
        <OwnerAvatar ownerKind="agent" ownerId={ownerId} size="default" />
        <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">{title}</h1>
        <AgentCallStatePill state={view.state} fallbackLabel={view.stateLabel} />
        {view.verdict ? (
          <Pill
            size="xs"
            tone="neutral"
            mono
            data-testid="agent-call-verdict"
            data-verdict={view.verdict}
          >
            {CALL_VERDICT_SIGNAL[view.verdict].label}
          </Pill>
        ) : null}
        <AgentCallLiveness state={view.state} />
        <span className="flex-1" />
        <div className="flex items-center gap-1.5">
          {view.controls.cancel && onCancel ? (
            <Button
              size="sm"
              variant="outline"
              type="button"
              disabled={cancelPending}
              onClick={onCancel}
              data-testid="agent-call-cancel"
            >
              <Ban aria-hidden="true" />
              Cancel
            </Button>
          ) : null}
          {view.controls.callAgain && onCallAgain ? (
            <Button
              size="sm"
              variant="outline"
              type="button"
              onClick={onCallAgain}
              data-testid="agent-call-again"
            >
              <CornerUpRight aria-hidden="true" />
              Call again
            </Button>
          ) : null}
          {view.controls.messageChild && onMessageChild ? (
            <Button
              size="sm"
              variant="outline"
              type="button"
              onClick={onMessageChild}
              data-testid="agent-call-message-child"
            >
              <Mail aria-hidden="true" />
              Message child
            </Button>
          ) : null}
        </div>
      </div>

      <MetadataList className="mt-2">
        <MetadataList.Row label="caller">
          {onOpenCaller ? (
            <Button variant="link" size="xs" type="button" onClick={onOpenCaller}>
              <MonoId value={view.callerId} />
            </Button>
          ) : (
            <MonoId value={view.callerId} />
          )}
          {view.callerKind === "session" ? <span className="text-muted">(session)</span> : null}
        </MetadataList.Row>
        {view.childSessionId ? (
          <MetadataList.Row label="child">
            {view.controls.openChildSession && onOpenChildSession ? (
              <Button variant="link" size="xs" type="button" onClick={onOpenChildSession}>
                <MonoId value={view.childSessionId} />
              </Button>
            ) : (
              <MonoId value={view.childSessionId} />
            )}
          </MetadataList.Row>
        ) : null}
        <MetadataList.Row label="depth">{view.depth}</MetadataList.Row>
        {idleCopy ? (
          <MetadataList.Row label="idle TTL">
            <span className="text-small-body text-fg">{idleCopy}</span>
          </MetadataList.Row>
        ) : null}
        {view.deadlineAt ? (
          <MetadataList.Row label="deadline">
            <span className="font-mono text-form">
              {view.deadlineAt} <span className="font-sans text-muted">(opt-in)</span>
            </span>
          </MetadataList.Row>
        ) : null}
        {view.settledAt ? (
          <MetadataList.Row label="settled">
            <span className="font-mono text-form">
              {view.settledAt}
              {settledDuration ? <span className="text-muted"> ({settledDuration})</span> : null}
            </span>
          </MetadataList.Row>
        ) : null}
      </MetadataList>
    </header>
  );
}
