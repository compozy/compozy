import type { TaskTimelineItem } from "../types";

/** Activity feed filter buckets — filtering replaces the old grouping modes. */
export const TASK_ACTIVITY_FILTERS = ["all", "runs", "reviews", "changes"] as const;
export type TaskActivityFilter = (typeof TASK_ACTIVITY_FILTERS)[number];
export type TaskActivityCategory = Exclude<TaskActivityFilter, "all">;

export interface TaskActivityView {
  /** Plain-language event title (never the raw event type). */
  title: string;
  /** Optional secondary line (error snippet, reason, payload message). */
  detail?: string;
  category: TaskActivityCategory;
}

function payloadString(item: TaskTimelineItem, key: string): string | undefined {
  const payload = item.payload;
  if (!payload || typeof payload !== "object") return undefined;
  const value = (payload as Record<string, unknown>)[key];
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function payloadDetail(item: TaskTimelineItem): string | undefined {
  return (
    payloadString(item, "message") ??
    payloadString(item, "diagnostic") ??
    payloadString(item, "reason")
  );
}

function actorRef(item: TaskTimelineItem): string | undefined {
  const ref = item.actor?.ref;
  return typeof ref === "string" && ref.trim().length > 0 ? ref : undefined;
}

function attemptLabel(item: TaskTimelineItem): string {
  const attempt = item.run?.attempt;
  return typeof attempt === "number" && attempt > 0 ? `Attempt ${attempt}` : "Run";
}

function byActor(base: string, item: TaskTimelineItem): string {
  const actor = actorRef(item);
  return actor ? `${base} by ${actor}` : base;
}

/**
 * Humanizes a task timeline event into plain product language. Raw
 * `event_type` + `sequence` stay available on
 * the item for the operator microtext; this map owns only the words.
 * Unmapped types fall back to a generic "System event" title so new runtime
 * events degrade gracefully instead of leaking scheduler vocabulary.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §7.2
 */
export function humanizeTaskEvent(item: TaskTimelineItem): TaskActivityView {
  const detail = payloadDetail(item);
  const runError = item.run?.error ?? undefined;

  switch (item.event_type) {
    // ---- run lifecycle -------------------------------------------------
    case "task.run_enqueued":
      return { title: "Run queued", detail, category: "runs" };
    case "task.run_claimed": {
      const actor = actorRef(item);
      return {
        title: actor ? `${actor} picked this up` : "Picked up by a worker",
        category: "runs",
      };
    }
    case "task.run_starting":
      return { title: `${attemptLabel(item)} starting`, detail, category: "runs" };
    case "task.run_started":
      return { title: `${attemptLabel(item)} started`, detail, category: "runs" };
    case "task.run_session_bound":
      return { title: "Session attached", detail, category: "runs" };
    case "task.run.completed":
    case "task.run_completed":
      return { title: `${attemptLabel(item)} completed`, detail, category: "runs" };
    case "task.run.failed":
    case "task.run_failed":
      return {
        title: `${attemptLabel(item)} failed`,
        detail: runError ?? detail,
        category: "runs",
      };
    case "task.run_canceled":
      return { title: `${attemptLabel(item)} canceled`, detail, category: "runs" };
    case "task.run_needs_attention":
      return {
        title: `${attemptLabel(item)} got stuck`,
        detail: runError ?? detail,
        category: "runs",
      };
    case "task.run_recovered":
    case "task.run_recovered_from_attention":
      return { title: "Recovered and requeued", detail, category: "runs" };
    case "task.run_released":
      return { title: byActor("Run released", item), detail, category: "runs" };
    case "task.run_starved":
      return { title: "Waiting for a worker to pick this up", detail, category: "runs" };
    case "task.run_operator_retry":
      return { title: byActor("Retry queued", item), detail, category: "runs" };
    case "task.run_operator_forced_fail":
      return { title: byActor("Marked failed", item), detail, category: "runs" };
    case "task.run_force_stopped":
      return { title: byActor("Run force stopped", item), detail, category: "runs" };
    case "task.run_lease_expired":
      return { title: "Worker stopped responding", detail, category: "runs" };
    case "task.run_lease_extended":
      return { title: "Worker checked in", category: "runs" };
    case "task.run_rejected":
      return { title: "Run rejected", detail, category: "runs" };

    // ---- reviews -------------------------------------------------------
    case "task.run_review_requested":
      return { title: "Review requested", detail, category: "reviews" };
    case "task.run_review_bound":
      return { title: "Reviewer assigned", detail, category: "reviews" };
    case "task.run_review_recorded":
      return { title: "Review recorded", detail, category: "reviews" };
    case "task.run_review_approved":
      return { title: "Review: approved", detail, category: "reviews" };
    case "task.run_review_rejected":
      return { title: "Review: changes requested", detail, category: "reviews" };
    case "task.run_review_blocked":
      return { title: "Review blocked", detail, category: "reviews" };
    case "task.run_review_error":
      return { title: "Review hit an error", detail, category: "reviews" };
    case "task.run_review_invalid_output":
      return { title: "Review output could not be read", detail, category: "reviews" };
    case "task.run_review_timeout":
      return { title: "Review timed out", detail, category: "reviews" };
    case "task.run_review_retry_enqueued":
      return { title: "Review retry queued", detail, category: "reviews" };

    // ---- task record changes ------------------------------------------
    case "task.created":
      return { title: byActor("Task created", item), category: "changes" };
    case "task.updated":
      return { title: byActor("Details updated", item), detail, category: "changes" };
    case "task.published":
      return { title: byActor("Published", item), category: "changes" };
    case "task.paused":
      return { title: byActor("Paused", item), detail, category: "changes" };
    case "task.resumed":
      return { title: byActor("Resumed", item), category: "changes" };
    case "task.approved":
      return { title: byActor("Approved", item), detail, category: "changes" };
    case "task.rejected":
      return { title: byActor("Rejected", item), detail, category: "changes" };
    case "task.completed":
      return { title: "Task completed", detail, category: "changes" };
    case "task.failed":
      return { title: "Task failed", detail: runError ?? detail, category: "changes" };
    case "task.canceled":
      return { title: byActor("Task canceled", item), detail, category: "changes" };
    case "task.recovered":
      return { title: byActor("Recovered", item), detail, category: "changes" };
    case "task.needs_attention":
      return { title: "Needs attention", detail: runError ?? detail, category: "changes" };
    case "task.blocked":
      return { title: "Blocked", detail, category: "changes" };
    case "task.unblocked":
    case "task.block.cleared":
      return { title: "Block cleared", detail, category: "changes" };
    case "task.block.created":
      return { title: "Blocked", detail, category: "changes" };
    case "task.block.expired":
      return { title: "Block expired", detail, category: "changes" };
    case "task.child_created":
      return { title: "Subtask created", detail, category: "changes" };
    case "task.dependency_added":
      return { title: "Now waits on another task", detail, category: "changes" };
    case "task.dependency_removed":
      return { title: "Dependency removed", detail, category: "changes" };
    case "task.execution_profile_updated":
      return { title: byActor("Setup updated", item), category: "changes" };
    case "task.execution_profile_deleted":
      return { title: byActor("Setup cleared", item), category: "changes" };
    case "task.auto_enqueue.triggered":
      return { title: "Started automatically (dependencies cleared)", category: "changes" };
    case "task.status_changed":
      return { title: "Status changed", detail, category: "changes" };
    case "task.completion.hallucination_suspected":
      return { title: "Completion claim looks unverified", detail, category: "changes" };
    case "task.completion.hallucination_blocked":
      return { title: "Completion claim blocked", detail, category: "changes" };
    case "task.wake.delivered":
      return { title: "Wake delivered", detail, category: "changes" };
    case "task.wake.suppressed":
      return { title: "Wake suppressed", detail, category: "changes" };
    case "task.notification_delivered":
      return { title: "Notification delivered", detail, category: "changes" };

    default:
      return { title: "System event", detail, category: "changes" };
  }
}

export function matchesActivityFilter(view: TaskActivityView, filter: TaskActivityFilter): boolean {
  return filter === "all" || view.category === filter;
}
