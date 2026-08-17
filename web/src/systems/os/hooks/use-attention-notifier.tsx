import { useEffect, useRef, useState } from "react";

import { toast } from "@compozy/ui";

import { useDocumentActive } from "@/hooks/use-document-active";
import { notifyUser } from "@/lib/user-feedback";
import { isNeedsYouBadge } from "@/systems/session";

import { AttentionToast, AttentionToastOverflowLedge } from "../components/attention-toast";
import {
  createAttentionDeliveryEngine,
  type AttentionDelivery,
  type AttentionDeliveryContext,
  type AttentionDeliveryEngine,
  type AttentionDeliveryTarget,
} from "../lib/attention-delivery";
import { subscribeAttentionEdges, subscribeOperatorNotifications } from "../lib/attention-edge-bus";
import { systemNotificationContent } from "../lib/attention-notification-content";
import { createAttentionSoundPlayer } from "../lib/attention-sound";
import {
  showSystemNotification,
  systemNotificationsArmed,
} from "../lib/system-notification-channel";
import type { OsAttentionSections } from "../lib/attention-model";
import {
  useAttentionJump,
  type AttentionJump,
  type AttentionJumpTarget,
} from "./use-attention-jump";
import { useAttentionPolicy } from "./use-attention-policy";

const TOAST_DURATION_MS = 8_000;
export const MAX_VISIBLE_NEEDS_YOU_TOASTS = 4;

export interface AttentionToastPlan {
  deliveries: AttentionDelivery[];
  overflowNeedsYou: number;
}

/**
 * Keeps the four newest needs-you actions visible. Older actions remain in the
 * authoritative bell and collapse into one count ledge instead of filling the
 * screen with glass.
 */
export function planAttentionToasts(batch: AttentionDelivery[]): AttentionToastPlan {
  const needsYou = batch.filter(delivery => delivery.kind === "needs-you");
  const overflowNeedsYou = Math.max(0, needsYou.length - MAX_VISIBLE_NEEDS_YOU_TOASTS);
  if (overflowNeedsYou === 0) return { deliveries: batch, overflowNeedsYou };

  const visibleNeedsYou = new Set(needsYou.slice(-MAX_VISIBLE_NEEDS_YOU_TOASTS));
  return {
    deliveries: batch.filter(
      delivery => delivery.kind !== "needs-you" || visibleNeedsYou.has(delivery)
    ),
    overflowNeedsYou,
  };
}

function targetsFromSections(sections: OsAttentionSections): Map<string, AttentionDeliveryTarget> {
  const targets = new Map<string, AttentionDeliveryTarget>();
  for (const row of [...sections.needsYou, ...sections.finished]) {
    if (row.kind !== "session") continue;
    targets.set(row.id, row);
  }
  return targets;
}

export function attentionJumpTargetFor(delivery: AttentionDelivery): AttentionJumpTarget | null {
  if (delivery.kind === "agent") {
    return {
      sessionId: delivery.notification.session_id,
      workspaceId: delivery.notification.workspace_id,
      ...(delivery.target ? { agentName: delivery.target.agentName } : {}),
    };
  }
  const target =
    delivery.kind === "needs-you"
      ? delivery.target
      : delivery.targets.length === 1
        ? (delivery.targets[0] ?? null)
        : null;
  return target
    ? {
        sessionId: target.id,
        agentName: target.agentName,
        workspaceId: target.workspaceId,
      }
    : null;
}

function activate(
  delivery: AttentionDelivery,
  jump: AttentionJump,
  openBell: () => void,
  resolve: AttentionDeliveryContext["resolve"]
): void {
  const target = attentionJumpTargetFor(delivery);
  // A grouped completion lands on the list, not on a session — there is no
  // single session it could honestly claim to be about.
  if (target === null) {
    openBell();
    return;
  }
  if (delivery.kind === "needs-you" && !isNeedsYouBadge(resolve(delivery.target.id)?.badge)) {
    notifyUser({ message: "This session no longer needs attention.", tone: "info" });
  }
  jump(target);
}

export interface UseAttentionNotifierOptions {
  sections: OsAttentionSections;
  /** No notification ever fires from a stale stream. */
  streamLive: boolean;
  focusedSessionId: string | null;
  onOpenBell: () => void;
}

/**
 * Turns live attention edges into notifications.
 *
 * Edges are ephemeral: they say *what changed*, never *what is true*. Content
 * is read from the refetched read model, so a toast can only ever describe a
 * session the client actually knows about.
 *
 * A stale stream fires nothing at all. That is what stops a reconnect — or an
 * unmute — from replaying every transition the operator missed as a burst of
 * toasts for work that has already resolved.
 */
export function useAttentionNotifier({
  sections,
  streamLive,
  focusedSessionId,
  onOpenBell,
}: UseAttentionNotifierOptions): void {
  const policy = useAttentionPolicy();
  const appActive = useDocumentActive();
  const jump = useAttentionJump();
  // Stable per-mount instances: the engine owns timers and pending state, so a
  // fresh one would silently drop in-flight deliveries. A lazy `useState`
  // initializer gives one instance for the life of the mount without writing a
  // ref during render.
  const [sound] = useState(createAttentionSoundPlayer);
  // Latest-value refs, committed in effects rather than during render: a render
  // that React discards must not leave the engine reading state that never
  // reached the screen.
  const contextRef = useRef<AttentionDeliveryContext>({
    mutedWorkspaceIds: new Set<string>(),
    focusedSessionId: null,
    appActive: true,
    resolve: () => null,
  });
  const renderRef = useRef<(batch: AttentionDelivery[]) => void>(() => undefined);

  const targets = targetsFromSections(sections);
  const context: AttentionDeliveryContext = {
    mutedWorkspaceIds: policy.mutedWorkspaceIds,
    focusedSessionId,
    appActive,
    resolve: sessionId => targets.get(sessionId) ?? null,
  };

  const render = (batch: AttentionDelivery[]) => {
    if (batch.length === 0) return;
    if (policy.toasts) {
      const plan = planAttentionToasts(batch);
      if (plan.overflowNeedsYou > 0) {
        toast.custom(
          id => (
            <AttentionToastOverflowLedge
              count={plan.overflowNeedsYou}
              onActivate={() => {
                toast.dismiss(id);
                onOpenBell();
              }}
            />
          ),
          {
            id: `attention-overflow:${batch.map(delivery => delivery.key).join(",")}`,
            duration: TOAST_DURATION_MS,
            closeButton: false,
            dismissible: false,
          }
        );
      }
      for (const delivery of plan.deliveries) {
        toast.custom(
          () => (
            <AttentionToast
              delivery={delivery}
              onActivate={() => activate(delivery, jump, onOpenBell, contextRef.current.resolve)}
            />
          ),
          { id: delivery.key, duration: TOAST_DURATION_MS }
        );
      }
    }
    // System alerts escalate only while the app cannot show a toast the
    // operator would already see.
    if (systemNotificationsArmed(policy.system) && !appActive) {
      for (const delivery of batch) {
        showSystemNotification({
          ...systemNotificationContent(delivery),
          onActivate: () => activate(delivery, jump, onOpenBell, contextRef.current.resolve),
        });
      }
    }
    // One chime for the whole batch, never one per toast.
    if (policy.sound) sound.play();
  };

  // The engine's lifetime is the live stream's lifetime. That is the rule, not
  // a convenience: when the stream goes stale its pending windows die with it,
  // and reconnecting starts clean — which is exactly why a reconnect cannot
  // replay a burst of toasts for work that already resolved.
  const engineRef = useRef<AttentionDeliveryEngine | null>(null);
  useEffect(() => {
    if (!streamLive) return undefined;
    const engine = createAttentionDeliveryEngine({
      now: () => Date.now(),
      getContext: () => contextRef.current,
      deliver: batch => renderRef.current(batch),
    });
    engineRef.current = engine;
    const unsubscribeEdges = subscribeAttentionEdges(edge => engine.ingestEdge(edge));
    const unsubscribeNotifications = subscribeOperatorNotifications(notification =>
      engine.ingestNotification(notification)
    );
    return () => {
      unsubscribeEdges();
      unsubscribeNotifications();
      engineRef.current = null;
      engine.dispose();
    };
  }, [streamLive]);

  useEffect(() => {
    contextRef.current = context;
    renderRef.current = render;
    // A refetch may have named an edge that arrived before its catalog wake.
    engineRef.current?.refresh();
  });
}
