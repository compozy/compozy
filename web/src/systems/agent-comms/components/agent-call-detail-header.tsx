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
import {
  Ban,
  Calendar,
  CornerUpRight,
  GitBranch,
  Layers,
  Mail,
  Timer,
  User,
  type LucideIcon,
} from "lucide-react";

import { Button, MonoId, OwnerAvatar, Time, cn } from "@compozy/ui";

import {
  AgentCallLiveness,
  AgentCallStatePill,
  AgentCallVerdictChip,
} from "./agent-call-state-pill";
import type { CallDetailView } from "../lib/call-detail-view-model";

function Fact({ icon: Icon, children }: { icon: LucideIcon; children: React.ReactNode }) {
  return (
    <span className="flex items-center gap-1.5 text-form text-muted">
      <Icon className="size-3 shrink-0" aria-hidden="true" />
      {children}
    </span>
  );
}

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
  return (
    <header {...props} className={cn("border-b border-line-soft pb-3", className)}>
      <div className="flex flex-wrap items-center gap-2">
        <OwnerAvatar ownerKind="agent" ownerId={title} size="default" />
        <h2 className="text-item-title text-fg-strong">{title}</h2>
        <AgentCallStatePill state={view.state} fallbackLabel={view.stateLabel} />
        <AgentCallVerdictChip verdict={view.verdict} data-testid="agent-call-verdict" />
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

      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5">
        <Fact icon={User}>
          caller{" "}
          {onOpenCaller ? (
            <Button variant="link" size="xs" type="button" onClick={onOpenCaller}>
              <MonoId value={view.callerId} />
            </Button>
          ) : (
            <MonoId value={view.callerId} />
          )}
        </Fact>
        {view.childSessionId ? (
          <Fact icon={GitBranch}>
            child{" "}
            {view.controls.openChildSession && onOpenChildSession ? (
              <Button variant="link" size="xs" type="button" onClick={onOpenChildSession}>
                <MonoId value={view.childSessionId} />
              </Button>
            ) : (
              // Retention pruned the session: identity stays, while the unavailable jump is omitted.
              <MonoId value={view.childSessionId} />
            )}
          </Fact>
        ) : null}
        <Fact icon={Layers}>depth {view.depth}</Fact>
        {view.idleTtl.kind === "suspended" ? (
          <Fact icon={Timer}>
            idle TTL <span className="font-mono">suspended while running</span>
          </Fact>
        ) : null}
        {view.idleTtl.kind === "counting" ? (
          <Fact icon={Timer}>
            idle TTL <Time iso={view.idleTtl.expiresAt} />
          </Fact>
        ) : null}
        {view.deadlineAt ? (
          <Fact icon={Timer}>
            deadline <Time iso={view.deadlineAt} mode="absolute" /> (opt-in)
          </Fact>
        ) : null}
        {view.settledAt ? (
          <Fact icon={Calendar}>
            settled <Time iso={view.settledAt} mode="absolute" />
          </Fact>
        ) : null}
      </div>
    </header>
  );
}
