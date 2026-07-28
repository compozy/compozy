import type { AutomationJob, AutomationTrigger } from "@/systems/automation";
import { describeSchedule, formatRelativeTime } from "@/systems/automation";
import type { LoopBindingKind, LoopBindingRow } from "@/systems/loops";

/** The active workspace's attached loop-target automations, keyed by loop name. */
export interface LoopBindingIndexEntry {
  kinds: LoopBindingKind[];
  rows: LoopBindingRow[];
}

function scheduleMeta(job: AutomationJob): string {
  const cadence = describeSchedule(job.schedule);
  const nextRun = job.next_run ?? job.scheduler?.next_run_at ?? null;
  return nextRun ? `${cadence} · next ${formatRelativeTime(nextRun)}` : cadence;
}

/**
 * Maps a scheduled job to a loop binding row when it targets a Loop (a job's
 * loop-target binding kind is always `schedule`, TechSpec §9.14). Returns null for
 * agent-target jobs or jobs whose `loop_target` names a different workspace.
 */
export function jobToBindingRow(job: AutomationJob, workspaceId: string): LoopBindingRow | null {
  if (!job.loop_target || job.loop_target.workspace_id !== workspaceId) return null;
  return {
    id: job.id,
    name: job.name,
    kind: "schedule",
    enabled: job.enabled,
    meta: scheduleMeta(job),
  };
}

/** A webhook trigger carries a webhook endpoint; every other event trigger is `trigger`. */
function triggerBindingKind(trigger: AutomationTrigger): LoopBindingKind {
  return trigger.webhook_id || trigger.endpoint_slug ? "webhook" : "trigger";
}

function triggerMeta(trigger: AutomationTrigger, kind: LoopBindingKind): string {
  if (kind === "webhook") {
    return trigger.endpoint_slug ? `endpoint ${trigger.endpoint_slug}` : "webhook";
  }
  return trigger.event || "event";
}

/**
 * Maps an event trigger to a loop binding row when it targets a Loop, deriving the
 * binding kind (`webhook` vs `trigger`) from the trigger itself. Returns null for
 * agent-target triggers or targets naming a different workspace.
 */
export function triggerToBindingRow(
  trigger: AutomationTrigger,
  workspaceId: string
): LoopBindingRow | null {
  if (!trigger.loop_target || trigger.loop_target.workspace_id !== workspaceId) return null;
  const kind = triggerBindingKind(trigger);
  return {
    id: trigger.id,
    name: trigger.name,
    kind,
    enabled: trigger.enabled,
    meta: triggerMeta(trigger, kind),
  };
}

/** Loop name a job/trigger targets in this workspace, or null. */
function targetLoopName(
  item: Pick<AutomationJob, "loop_target"> | Pick<AutomationTrigger, "loop_target">,
  workspaceId: string
): string | null {
  const target = item.loop_target;
  if (!target || target.workspace_id !== workspaceId) return null;
  return target.loop_name;
}

/**
 * Builds the workspace's loop-binding index from all triggers + jobs: a map from
 * loop name to its attached-automation kinds (catalog badge) and detail rows.
 */
export function buildLoopBindingIndex(
  triggers: readonly AutomationTrigger[],
  jobs: readonly AutomationJob[],
  workspaceId: string
): Map<string, LoopBindingIndexEntry> {
  const index = new Map<string, LoopBindingIndexEntry>();
  const add = (loopName: string, row: LoopBindingRow) => {
    const entry = index.get(loopName) ?? { kinds: [], rows: [] };
    entry.rows.push(row);
    if (!entry.kinds.includes(row.kind)) entry.kinds.push(row.kind);
    index.set(loopName, entry);
  };
  for (const trigger of triggers) {
    const loopName = targetLoopName(trigger, workspaceId);
    const row = loopName ? triggerToBindingRow(trigger, workspaceId) : null;
    if (loopName && row) add(loopName, row);
  }
  for (const job of jobs) {
    const loopName = targetLoopName(job, workspaceId);
    const row = loopName ? jobToBindingRow(job, workspaceId) : null;
    if (loopName && row) add(loopName, row);
  }
  for (const entry of index.values()) entry.kinds.sort((a, b) => a.localeCompare(b));
  return index;
}
