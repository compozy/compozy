import type { AttentionDelivery } from "./attention-delivery";

export interface SystemNotificationContent {
  title: string;
  body?: string;
  tag: string;
}

/**
 * Content for the system (OS) notification. The notification chrome belongs to
 * the platform; only these words are ours, and they say the same thing the
 * toast says so the two channels never disagree.
 */
export function systemNotificationContent(delivery: AttentionDelivery): SystemNotificationContent {
  if (delivery.kind === "needs-you") {
    return {
      title: delivery.target.title,
      body: `${delivery.target.agentName}: ${delivery.target.reason}`,
      tag: delivery.key,
    };
  }
  if (delivery.kind === "finished") {
    const single = delivery.targets.length === 1 ? delivery.targets[0] : undefined;
    if (single) {
      return {
        title: single.title,
        body: `${single.agentName} finished while you were away`,
        tag: delivery.key,
      };
    }
    return {
      title: `${delivery.targets.length} sessions finished`,
      body: [...new Set(delivery.targets.map(target => target.workspaceLabel))].join(" · "),
      tag: delivery.key,
    };
  }
  return {
    title: delivery.notification.title,
    ...(delivery.notification.body ? { body: delivery.notification.body } : {}),
    tag: delivery.key,
  };
}
