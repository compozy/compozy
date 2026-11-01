import { Megaphone } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, cn, Icon, type AgentAvatarState } from "@compozy/ui";

import { AgentIdentityMark } from "@/systems/agent/components/agent-identity-mark";
import { useAgentIdentityResolverView } from "@/systems/agent/hooks/use-agent-identity-view";
import { SessionBadgeGlyph } from "@/systems/session/components/session-badge-mark";

import { finishedDeliveryBody, type AttentionDelivery } from "../lib/attention-delivery";

/**
 * Attention toast content. One anatomy for all three kinds: mark, what
 * happened, and where it happened. The toast body itself is the jump, so
 * needs-you toasts carry no buttons — only the coalesced completion has an
 * action, because its landing is a list rather than a session.
 */

export function ToastFrame({
  mark,
  title,
  body,
  meta,
  action,
  onActivate,
  testId,
}: {
  mark: React.ReactNode;
  title: string;
  body?: string;
  meta: string;
  action?: { label: string; onClick: () => void };
  onActivate?: () => void;
  testId: string;
}) {
  return (
    <div
      data-testid={testId}
      className="pointer-events-auto relative flex w-full items-start gap-2.5 rounded-lg bg-popover p-3 text-left shadow-pop"
    >
      {mark}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        {onActivate ? (
          // The title's stretched overlay makes the whole card the jump without
          // nesting the action button inside another interactive element.
          <button
            type="button"
            onClick={onActivate}
            className={cn(
              "truncate text-left text-small-body font-semibold text-fg-strong focus-visible:outline-none",
              "after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring"
            )}
          >
            {title}
          </button>
        ) : (
          <span className="truncate text-small-body font-medium text-fg-strong">{title}</span>
        )}
        {body ? <span className="text-small-body text-fg">{body}</span> : null}
        {meta ? <span className="mt-0.5 font-mono text-eyebrow text-faint">{meta}</span> : null}
        {action ? (
          <span className="relative z-10 mt-1.5 flex">
            <Button size="sm" variant="secondary" onClick={action.onClick}>
              {action.label}
            </Button>
          </span>
        ) : null}
      </span>
    </div>
  );
}

export interface AttentionToastProps {
  delivery: AttentionDelivery;
  onActivate: () => void;
}

export interface AttentionToastOverflowLedgeProps {
  count: number;
  onActivate: () => void;
}

/** The quiet fifth row in a needs-you burst; older actions remain in the bell. */
export function AttentionToastOverflowLedge({
  count,
  onActivate,
}: AttentionToastOverflowLedgeProps) {
  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      className="pointer-events-auto w-full justify-center shadow-pop"
      data-testid="os-attention-toast-overflow"
      onClick={onActivate}
    >
      +{count} more need you
    </Button>
  );
}

function attentionDeliveryWorkspace(delivery: AttentionDelivery) {
  if (delivery.kind === "finished") return delivery.targets[0]?.workspaceId;
  return delivery.target?.workspaceId;
}

/** A payload with no agent keeps the session's badge rather than a face for nobody. */
function AttentionAgentMark({
  target,
  state,
  fallback,
}: {
  target: { agentName: string; workspaceId?: string };
  state?: AgentAvatarState;
  fallback: ReactNode;
}) {
  if (!target.agentName) return fallback;
  return (
    <AgentIdentityMark
      agentScope={{ workspace: target.workspaceId }}
      name={target.agentName}
      size={32}
      state={state}
      surfaceClassName="ring-popover"
    />
  );
}

export function AttentionToast({ delivery, onActivate }: AttentionToastProps) {
  const identityOf = useAgentIdentityResolverView({
    workspace: attentionDeliveryWorkspace(delivery),
  });
  if (delivery.kind === "needs-you") {
    const { target } = delivery;
    const label = identityOf(target.agentName).label;
    return (
      <ToastFrame
        testId="os-attention-toast-needs-you"
        mark={
          <AttentionAgentMark
            fallback={<SessionBadgeGlyph badge={target.badge} />}
            state="needs-you"
            target={target}
          />
        }
        title={label ? `${label} needs you` : target.title}
        body={label ? `${target.title}: ${target.reason}` : target.reason}
        meta={target.workspaceLabel}
        onActivate={onActivate}
      />
    );
  }
  if (delivery.kind === "finished") {
    const { targets } = delivery;
    const workspaces = [...new Set(targets.map(target => target.workspaceLabel))].join(" · ");
    const single = targets.length === 1 ? targets[0] : undefined;
    return (
      <ToastFrame
        testId="os-attention-toast-finished"
        mark={
          single ? (
            <AttentionAgentMark fallback={<SessionBadgeGlyph badge="done" />} target={single} />
          ) : (
            <SessionBadgeGlyph badge="done" />
          )
        }
        title={single ? single.title : `${targets.length} sessions finished`}
        {...(single ? { body: finishedDeliveryBody(single) } : {})}
        meta={
          single
            ? [single.workspaceLabel, identityOf(single.agentName).label]
                .filter(Boolean)
                .join(" · ")
            : workspaces
        }
        {...(single
          ? { onActivate }
          : { action: { label: "Review finished", onClick: onActivate } })}
      />
    );
  }
  const { notification, target } = delivery;
  return (
    <ToastFrame
      testId="os-attention-toast-agent"
      mark={
        <AttentionAgentMark
          fallback={
            <span
              aria-hidden="true"
              className="grid size-4.5 shrink-0 place-items-center rounded-full bg-info-tint text-info"
            >
              <Icon as={Megaphone} size="xs" />
            </span>
          }
          target={{ agentName: target?.agentName ?? "", workspaceId: target?.workspaceId }}
        />
      }
      title={notification.title}
      {...(notification.body ? { body: notification.body } : {})}
      // Unresolved targets omit the missing half rather than print raw IDs.
      meta={[target?.workspaceLabel, target?.agentName ? identityOf(target.agentName).label : null]
        .filter(Boolean)
        .join(" · ")}
      onActivate={onActivate}
    />
  );
}
