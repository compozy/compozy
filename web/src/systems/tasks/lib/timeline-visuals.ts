import {
  AlertTriangle,
  CheckCircle2,
  CircleDot,
  GitBranch,
  Hourglass,
  Inbox as InboxIcon,
  Pause,
  PlayCircle,
  Plus,
  RotateCw,
  Sparkles,
  XCircle,
  type LucideIcon,
} from "lucide-react";

import type { PillTone } from "@agh/ui";

export interface EventVisualMeta {
  tone: PillTone;
  icon: LucideIcon;
}

export const FAILURE_EVENT_TYPES: ReadonlySet<string> = new Set([
  "task.run_failed",
  "task.failed",
  "task.run_canceled",
  "task.canceled",
]);

export const LIVE_EVENT_TYPES: ReadonlySet<string> = new Set([
  "task.run_progress",
  "task.run_started",
  "task.run_claimed",
]);

export const SUCCESS_EVENT_TYPES: ReadonlySet<string> = new Set([
  "task.run_completed",
  "task.completed",
]);

const EVENT_VISUALS: Record<string, EventVisualMeta> = {
  "task.created": { tone: "neutral", icon: Plus },
  "task.run_enqueued": { tone: "neutral", icon: InboxIcon },
  "task.run_claimed": { tone: "info", icon: CircleDot },
  "task.run_started": { tone: "info", icon: PlayCircle },
  "task.run_progress": { tone: "info", icon: Hourglass },
  "task.run_completed": { tone: "success", icon: CheckCircle2 },
  "task.completed": { tone: "success", icon: CheckCircle2 },
  "task.run_failed": { tone: "danger", icon: XCircle },
  "task.failed": { tone: "danger", icon: XCircle },
  "task.run_canceled": { tone: "warning", icon: Pause },
  "task.canceled": { tone: "warning", icon: Pause },
  "task.run_blocked": { tone: "warning", icon: AlertTriangle },
  "task.run_operator_forced_fail": { tone: "danger", icon: XCircle },
  "task.run_operator_retry": { tone: "info", icon: RotateCw },
  "task.run_recovered_from_attention": { tone: "info", icon: RotateCw },
  "task.run_starved": { tone: "warning", icon: Hourglass },
  "task.run_needs_attention": { tone: "warning", icon: AlertTriangle },
  "task.dependency_added": { tone: "neutral", icon: GitBranch },
  "task.dependency_resolved": { tone: "success", icon: GitBranch },
};

export function visualFor(eventType: string): EventVisualMeta {
  return EVENT_VISUALS[eventType] ?? { tone: "neutral", icon: Sparkles };
}

export function isFailureEvent(eventType: string): boolean {
  return FAILURE_EVENT_TYPES.has(eventType);
}

export function isLiveEvent(eventType: string): boolean {
  return LIVE_EVENT_TYPES.has(eventType);
}

export function isSuccessEvent(eventType: string): boolean {
  return SUCCESS_EVENT_TYPES.has(eventType);
}

/**
 * Resolves the `<TimelineEvent>` tone for an event, factoring in the
 * top-level kind (failure / success) and the live state of the activity row.
 */
export function resolveEventTone(eventType: string, isLive: boolean): PillTone {
  if (isFailureEvent(eventType)) return "danger";
  if (isSuccessEvent(eventType)) return "success";
  if (isLive && isLiveEvent(eventType)) return "info";
  return visualFor(eventType).tone;
}
