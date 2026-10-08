/**
 * Operator-facing runtime facts for the automation Inspect sheet.
 *
 * The detail page speaks plain language; Inspect keeps the daemon's terms. A
 * job shows its kind, scheduler state and fire bookkeeping (the old job page's
 * advanced details); a trigger shows its dispatch facts and a sample event
 * rebuilt from the definition and the catalog fixture the create flow previews.
 */

import { getEventDef, type TriggerEnvelope } from "./trigger-catalog";
import { parseEventSelection } from "./trigger-event-id";
import { triggerFilterEntries } from "./trigger-filter";
import { projectAutomationTarget } from "./automation-target";
import { automationDoesOf } from "./automation-sentence";
import { isAutomationTrigger, type AutomationEntity } from "./automation-entity";
import type { AutomationJob, AutomationRetry, AutomationTrigger } from "../types";

export interface AutomationDiagnosticTile {
  id: string;
  label: string;
  value: string;
  /** Scheduler registration renders a status dot beside the word. */
  tone?: "success" | "faint";
}

function targetTile(entity: AutomationEntity): string {
  if (!isAutomationTrigger(entity) && automationDoesOf(entity) === "task") {
    return `task · ${entity.task?.title?.trim() || entity.name}`;
  }
  const target = projectAutomationTarget(entity);
  return target.kind === "loop" ? `loop · ${target.loopName}` : `agent · ${target.agentName}`;
}

/** Raw dispatch-retry internals, the shape the inspection surface exists for. */
function retryTile(retry: AutomationRetry): string {
  if (retry.strategy === "none") return "none";
  return `backoff · ${retry.max_retries} · ${retry.base_delay}`;
}

function jobDiagnostics(job: AutomationJob): AutomationDiagnosticTile[] {
  const scheduler = job.scheduler;
  return [
    { id: "kind", label: "Kind", value: `job · ${job.schedule?.mode ?? "manual"}` },
    { id: "source-scope", label: "Source · scope", value: `${job.source} · ${job.scope}` },
    {
      id: "scheduler",
      label: "Scheduler",
      value: scheduler?.registered ? "Registered" : "Not registered",
      tone: scheduler?.registered ? "success" : "faint",
    },
    { id: "missed", label: "Times missed", value: String(scheduler?.misfire_count ?? 0) },
    { id: "last-fire", label: "Last fire id", value: scheduler?.last_fire_id || "—" },
    { id: "target", label: "Target", value: targetTile(job) },
  ];
}

function triggerDiagnostics(trigger: AutomationTrigger): AutomationDiagnosticTile[] {
  const isWebhook = trigger.event === "webhook";
  return [
    { id: "kind", label: "Kind", value: `trigger · ${trigger.event}` },
    { id: "source-scope", label: "Source · scope", value: `${trigger.source} · ${trigger.scope}` },
    { id: "id", label: "Id", value: trigger.id },
    { id: "target", label: "Target", value: targetTile(trigger) },
    isWebhook
      ? {
          id: "secret",
          label: "Signing secret",
          value: trigger.webhook_secret_present ? "present" : "absent",
        }
      : { id: "retry", label: "Retry", value: retryTile(trigger.retry) },
  ];
}

/** Six-ish tiles: identity, provenance, and the runtime facts that differ by kind. */
export function buildAutomationDiagnostics(entity: AutomationEntity): AutomationDiagnosticTile[] {
  return isAutomationTrigger(entity) ? triggerDiagnostics(entity) : jobDiagnostics(entity);
}

function jobNote(job: AutomationJob): string[] {
  const schedule = job.schedule;
  const parts: string[] = [];
  if (schedule?.mode === "cron" && schedule.expr) parts.push(`Expression ${schedule.expr}`);
  if (schedule?.mode === "every" && schedule.interval) parts.push(`Every ${schedule.interval}`);
  if (schedule?.mode === "at" && schedule.time) parts.push(`At ${schedule.time}`);
  const catchUp = job.scheduler?.catch_up_policy ?? schedule?.catch_up_policy;
  if (catchUp) parts.push(`catch-up ${catchUp}`);
  const grace = job.scheduler?.misfire_grace_seconds ?? schedule?.misfire_grace_seconds;
  if (grace !== undefined) parts.push(`grace ${grace}s`);
  parts.push(`fire limit ${job.fire_limit.max} / ${job.fire_limit.window}`);
  return [`${parts.join(" · ")}.`];
}

function triggerNote(trigger: AutomationTrigger): string[] {
  const parts: string[] = [];
  if (trigger.event === "webhook" && trigger.endpoint_slug && trigger.webhook_id) {
    parts.push(`Slug ${trigger.endpoint_slug} · id ${trigger.webhook_id}.`);
  }
  const entries = triggerFilterEntries(trigger);
  if (entries.length > 0) {
    parts.push(`Filter ${entries.map(([key, value]) => `${key}=${value}`).join(" AND ")}.`);
  }
  const target = projectAutomationTarget(trigger);
  if (target.kind === "loop") {
    const mapping = Object.entries(target.inputMapping);
    if (mapping.length > 0) {
      parts.push(`Mapping ${mapping.map(([key, path]) => `${key} ← ${path}`).join(", ")}.`);
    }
  }
  parts.push(`Fire limit ${trigger.fire_limit.max} / ${trigger.fire_limit.window}.`);
  return parts;
}

/** Optional persisted details plus the target-kind contract, joined into one quiet line. */
export function buildAutomationDiagnosticsNote(entity: AutomationEntity): string {
  const parts = isAutomationTrigger(entity) ? triggerNote(entity) : jobNote(entity);
  return [...parts, "Target kind can't change after create."].join(" ");
}

/** Sheet description: whose runtime fields these are, in the daemon's noun. */
export function automationInspectDescription(entity: AutomationEntity): string {
  const noun = isAutomationTrigger(entity) ? "a trigger" : "a job";
  const secret =
    isAutomationTrigger(entity) && entity.event === "webhook"
      ? " The signing secret is write-only, so this shows presence, never the value."
      : "";
  return `Runtime fields for ${entity.name}. This is ${noun} in the daemon's terms.${secret}`;
}

/** The scheduler state JSON a job's second tab shows; `null` before the scheduler reports. */
export function formatSchedulerState(job: AutomationJob): string {
  return JSON.stringify(job.scheduler ?? null, null, 2);
}

export interface TriggerEnvelopeSample extends Omit<TriggerEnvelope, "data" | "scope" | "source"> {
  data: Record<string, unknown>;
  scope: string;
  source: string;
}

function setNestedValue(target: Record<string, unknown>, path: string[], value: string): void {
  let cursor = target;
  path.forEach((segment, index) => {
    if (index === path.length - 1) {
      cursor[segment] = value;
      return;
    }
    const existing = cursor[segment];
    const next =
      typeof existing === "object" && existing !== null && !Array.isArray(existing)
        ? { ...(existing as Record<string, unknown>) }
        : {};
    cursor[segment] = next;
    cursor = next;
  });
}

/**
 * A sample activation envelope for this trigger: the catalog payload overlaid
 * with every exact-match filter path and the current webhook identity.
 */
export function buildTriggerEnvelopeSample(trigger: AutomationTrigger): TriggerEnvelopeSample {
  const selection = parseEventSelection(trigger.event);
  const def = getEventDef(selection.catalogId);
  const envelope: TriggerEnvelopeSample = {
    kind: trigger.event,
    scope: trigger.scope,
    workspace_id: trigger.scope === "workspace" ? (trigger.workspace_id ?? "") : "",
    source: def?.sample.source ?? "observer",
    data: structuredClone(def?.sample.data ?? {}),
  };
  for (const [path, value] of triggerFilterEntries(trigger)) {
    if (path === "kind" || path === "scope" || path === "workspace_id" || path === "source") {
      envelope[path] = value;
      continue;
    }
    if (path.startsWith("data.")) setNestedValue(envelope.data, path.slice(5).split("."), value);
  }
  if (trigger.event === "webhook") {
    if (trigger.endpoint_slug) envelope.data.endpoint_slug = trigger.endpoint_slug;
    if (trigger.webhook_id) envelope.data.webhook_id = trigger.webhook_id;
    if (trigger.endpoint_slug && trigger.webhook_id) {
      envelope.data.endpoint = `${trigger.endpoint_slug}--${trigger.webhook_id}`;
    }
  }
  return envelope;
}

/** Pretty-printed sample for the Sample event pane's code block. */
export function formatTriggerEnvelope(trigger: AutomationTrigger): string {
  return JSON.stringify(buildTriggerEnvelopeSample(trigger), null, 2);
}
