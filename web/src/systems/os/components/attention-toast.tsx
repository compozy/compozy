import { Megaphone } from "lucide-react";

import { Button, Icon } from "@compozy/ui";

import { SessionBadgeGlyph } from "@/systems/session";

import type { AttentionDelivery } from "../lib/attention-delivery";

/**
 * Attention toast content. One anatomy for all three kinds: mark, what
 * happened, and where it happened. The toast body itself is the jump, so
 * needs-you toasts carry no buttons — only the coalesced completion has an
 * action, because its landing is a list rather than a session.
 */

function ToastFrame({
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
      role={onActivate ? "button" : undefined}
      tabIndex={onActivate ? 0 : undefined}
      data-testid={testId}
      className="flex w-full items-start gap-2.5 text-left"
      onClick={onActivate}
      onKeyDown={event => {
        if (event.key !== "Enter" && event.key !== " ") return;
        event.preventDefault();
        onActivate?.();
      }}
    >
      {mark}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="truncate text-small-body font-semibold text-fg-strong">{title}</span>
        {body ? <span className="text-small-body text-fg">{body}</span> : null}
        <span className="mt-0.5 font-mono text-micro text-faint">{meta}</span>
        {action ? (
          <span className="mt-1.5 flex">
            <Button
              size="sm"
              variant="outline"
              onClick={event => {
                event.stopPropagation();
                action.onClick();
              }}
            >
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
      variant="ghost"
      size="sm"
      className="w-full justify-center border border-line-soft bg-canvas-soft font-mono text-micro text-muted hover:bg-canvas-tint hover:text-fg-strong"
      data-testid="os-attention-toast-overflow"
      onClick={onActivate}
    >
      +{count} more need you
    </Button>
  );
}

export function AttentionToast({ delivery, onActivate }: AttentionToastProps) {
  if (delivery.kind === "needs-you") {
    const { target } = delivery;
    return (
      <ToastFrame
        testId="os-attention-toast-needs-you"
        mark={<SessionBadgeGlyph badge={target.badge} />}
        title={target.title}
        body={target.reason}
        meta={`${target.workspaceLabel} · ${target.agentName}`}
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
        mark={<SessionBadgeGlyph badge="done" />}
        title={single ? single.title : `${targets.length} sessions finished`}
        {...(single ? { body: "Finished while you were away" } : {})}
        meta={single ? `${single.workspaceLabel} · ${single.agentName}` : workspaces}
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
        <span
          aria-hidden="true"
          className="grid size-4.5 shrink-0 place-items-center rounded-full bg-info-tint text-info"
        >
          <Icon as={Megaphone} size="xs" />
        </span>
      }
      title={notification.title}
      {...(notification.body ? { body: notification.body } : {})}
      meta={`${target?.workspaceLabel ?? notification.workspace_id} · ${target?.agentName ?? notification.session_id}`}
      onActivate={onActivate}
    />
  );
}
