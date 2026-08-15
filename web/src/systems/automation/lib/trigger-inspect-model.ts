/**
 * Operator-facing runtime facts for the trigger Inspect sheet.
 *
 * Two panes, one model: the diagnostics tiles are the raw runtime enums the
 * calmer detail page deliberately keeps out of the default read, and the
 * envelope is a sample rebuilt from the trigger definition and the same catalog
 * fixture the create flow previews.
 */

import { getEventDef, type TriggerEnvelope } from "./trigger-catalog";
import { parseEventSelection } from "./trigger-event-id";
import { triggerFilterEntries } from "./trigger-filter";
import { projectAutomationTarget } from "./automation-target";
import type { AutomationRetry, AutomationTrigger } from "../types";

export interface TriggerDiagnosticTile {
  id: string;
  label: string;
  value: string;
}

function targetTile(trigger: AutomationTrigger): string {
  const target = projectAutomationTarget(trigger);
  return target.kind === "loop" ? `loop · ${target.loopName}` : `agent · ${target.agentName}`;
}

/** Raw dispatch-retry internals, the shape the inspection surface exists for. */
function retryTile(retry: AutomationRetry): string {
  if (retry.strategy === "none") return "none";
  return `backoff · ${retry.max_retries} · ${retry.base_delay}`;
}

/**
 * Six tiles: identity, provenance, and the one reliability fact that differs by
 * event — a webhook's secret presence, everything else's retry policy.
 */
export function buildTriggerDiagnostics(trigger: AutomationTrigger): TriggerDiagnosticTile[] {
  const isWebhook = trigger.event === "webhook";
  return [
    { id: "id", label: "Id", value: trigger.id },
    { id: "source", label: "Source", value: trigger.source.toUpperCase() },
    { id: "scope", label: "Scope", value: trigger.scope.toUpperCase() },
    { id: "kind", label: "Kind", value: trigger.event },
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

/**
 * Optional persisted details plus the target-kind contract, joined into one quiet line.
 */
export function buildTriggerDiagnosticsNote(trigger: AutomationTrigger): string {
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
  parts.push("Target kind is immutable after create.");
  return parts.join(" ");
}

/** Sheet description — what this read is and what it deliberately withholds. */
export function triggerInspectDescription(trigger: AutomationTrigger): string {
  if (trigger.event === "webhook") {
    return "Runtime fields and a sample envelope rebuilt from this trigger. The signing secret is write-only — this read shows presence, never the value.";
  }
  if (projectAutomationTarget(trigger).kind === "loop") {
    return "Runtime fields and a sample envelope rebuilt from this trigger. Target kind is immutable after create.";
  }
  return "Runtime fields and a sample envelope rebuilt from this trigger definition.";
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

/** Pretty-printed envelope for the Envelope pane's code block. */
export function formatTriggerEnvelope(trigger: AutomationTrigger): string {
  return JSON.stringify(buildTriggerEnvelopeSample(trigger), null, 2);
}
