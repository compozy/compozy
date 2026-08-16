import type {
  OperatorNotificationEventPayload,
  SessionAttentionEventPayload,
} from "@/systems/session";

/**
 * Carries live attention edges from the catalog stream to the notifier.
 *
 * The stream and the notifier have different lifetimes: the stream is a
 * generation-fenced single-owner connection, while the notifier re-renders with
 * the shell. A module-level bus keeps them decoupled, so a notifier re-render
 * can never tear down and reopen the stream.
 *
 * These events are *ephemeral delivery signals only*. State is never patched
 * from them — every surface refetches on the normal catalog wake, which the
 * daemon publishes for seen-clears too. An edge that arrives without its wake
 * is a missed toast, never a wrong badge.
 */

export type AttentionEdgeListener = (edge: SessionAttentionEventPayload) => void;
export type OperatorNotificationListener = (notification: OperatorNotificationEventPayload) => void;

const edgeListeners = new Set<AttentionEdgeListener>();
const notificationListeners = new Set<OperatorNotificationListener>();

export function subscribeAttentionEdges(listener: AttentionEdgeListener): () => void {
  edgeListeners.add(listener);
  return () => {
    edgeListeners.delete(listener);
  };
}

export function subscribeOperatorNotifications(listener: OperatorNotificationListener): () => void {
  notificationListeners.add(listener);
  return () => {
    notificationListeners.delete(listener);
  };
}

export function publishAttentionEdge(edge: SessionAttentionEventPayload): void {
  for (const listener of edgeListeners) listener(edge);
}

export function publishOperatorNotification(notification: OperatorNotificationEventPayload): void {
  for (const listener of notificationListeners) listener(notification);
}
