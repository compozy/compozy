import { Megaphone } from "lucide-react";

import { Button, cn, Icon } from "@compozy/ui";

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
      data-testid={testId}
      className="pointer-events-auto relative flex w-full items-start gap-2.5 rounded-lg bg-canvas p-3 text-left shadow-pop"
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
          <span className="truncate text-small-body font-semibold text-fg-strong">{title}</span>
        )}
        {body ? <span className="text-small-body text-fg">{body}</span> : null}
        {meta ? <span className="mt-0.5 font-mono text-micro text-faint">{meta}</span> : null}
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
      // Unresolved targets omit the missing half rather than print raw IDs.
      meta={[target?.workspaceLabel, target?.agentName].filter(Boolean).join(" · ")}
      onActivate={onActivate}
    />
  );
}
