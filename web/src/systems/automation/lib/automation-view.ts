/**
 * One view model for every automation. Projects a persisted job or trigger
 * into the shape rows, cards, the palette and the detail page render, so no
 * surface branches on the daemon entity beyond routes and API calls.
 */

import type { ProfileOwner } from "@/systems/profiles";

import {
  automationDoesOf,
  automationStartOf,
  describeAutomation,
  type AutomationDoes,
  type AutomationSentence,
  type AutomationStart,
  type SentenceContext,
} from "./automation-sentence";
import { automationScopeLabel, formatRelativeTime } from "./automation-formatters";
import type { AutomationJob, AutomationRunStatus, AutomationTrigger } from "../types";

export type { AutomationDoes, AutomationSentence, AutomationStart } from "./automation-sentence";

export type AutomationSkipReasonCode = "self_overlap" | "misfire_grace_exceeded";

export interface AutomationLastRun {
  id: string;
  status: AutomationRunStatus;
  startedAt?: string;
  endedAt?: string;
  skipReason?: AutomationSkipReasonCode;
}

export type AutomationEntityKind = "job" | "trigger";

export type AutomationDetailPath =
  | `/automations/jobs/${string}`
  | `/automations/triggers/${string}`;

export interface AutomationView {
  /** Daemon entity; routes and API calls branch on it. */
  kind: AutomationEntityKind;
  id: string;
  name: string;
  start: AutomationStart;
  does: AutomationDoes;
  sentence: AutomationSentence;
  enabled: boolean;
  source: "dynamic" | "config" | "package";
  scope: "workspace" | "global";
  workspaceId?: string;
  /** Project name for `workspaceId`, when the workspace list knows it. */
  workspaceName?: string;
  /** The owning profile; set only in aggregate (all-profiles) mode. */
  owner?: ProfileOwner;
  /** The profile name mutations route to. */
  profileName: string;
  /** Jobs: `scheduler.next_run_at ?? next_run`. */
  nextRunAt?: string;
  lastRun?: AutomationLastRun;
  /** Webhook public link path, when the trigger has a full identity. */
  webhookPath?: string;
  /** Webhooks only: the public link is published and reachable. */
  publicLinkLive?: boolean;
  canRunNow: boolean;
  canEdit: boolean;
  detailPath: AutomationDetailPath;
}

export interface AutomationViewContext extends SentenceContext {
  /** Resolves the owner tag; supplied only in aggregate mode. */
  ownerOf?: (entity: AutomationJob | AutomationTrigger) => ProfileOwner | undefined;
}

type AutomationEntity = AutomationJob | AutomationTrigger;

function isTrigger(entity: AutomationEntity): entity is AutomationTrigger {
  return "event" in entity;
}

function nonEmpty(value: string | null | undefined): string | undefined {
  return value ? value : undefined;
}

function toLastRun(entity: AutomationEntity): AutomationLastRun | undefined {
  const run = entity.last_run;
  if (!run) return undefined;
  const reason = run.skip_reason;
  const skipReason =
    reason === "self_overlap" || reason === "misfire_grace_exceeded" ? reason : undefined;
  return {
    id: run.id,
    status: run.status,
    ...(run.started_at ? { startedAt: run.started_at } : {}),
    ...(run.ended_at ? { endedAt: run.ended_at } : {}),
    ...(skipReason ? { skipReason } : {}),
  };
}

function detailPathOf(kind: AutomationEntityKind, id: string): AutomationDetailPath {
  const segment = encodeURIComponent(id);
  return kind === "job" ? `/automations/jobs/${segment}` : `/automations/triggers/${segment}`;
}

/** `/api/webhooks/{global|workspaces/{ws}}/{slug}--{id}`, or undefined when incomplete. */
function webhookPathOf(trigger: AutomationTrigger): string | undefined {
  if (trigger.event !== "webhook") return undefined;
  const slug = trigger.endpoint_slug?.trim();
  const webhookId = trigger.webhook_id?.trim();
  if (!slug || !webhookId) return undefined;
  const base =
    trigger.scope === "workspace" && trigger.workspace_id
      ? `workspaces/${trigger.workspace_id}`
      : "global";
  return `/api/webhooks/${base}/${slug}--${webhookId}`;
}

export function toAutomationView(
  entity: AutomationEntity,
  ctx: AutomationViewContext = {}
): AutomationView {
  const kind: AutomationEntityKind = isTrigger(entity) ? "trigger" : "job";
  const owner = ctx.ownerOf?.(entity);
  const workspaceName = entity.workspace_id
    ? (ctx.workspaceName?.(entity.workspace_id) ?? undefined)
    : undefined;
  const lastRun = toLastRun(entity);
  const nextRunAt = isTrigger(entity)
    ? undefined
    : nonEmpty(entity.scheduler?.next_run_at ?? entity.next_run);
  const webhookPath = isTrigger(entity) ? webhookPathOf(entity) : undefined;
  return {
    kind,
    id: entity.id,
    name: entity.name,
    start: automationStartOf(entity),
    does: automationDoesOf(entity),
    sentence: describeAutomation(entity, ctx),
    enabled: entity.enabled,
    source: entity.source,
    scope: entity.scope,
    ...(entity.workspace_id ? { workspaceId: entity.workspace_id } : {}),
    ...(workspaceName ? { workspaceName } : {}),
    ...(owner ? { owner } : {}),
    profileName: entity.profile_name,
    ...(nextRunAt ? { nextRunAt } : {}),
    ...(lastRun ? { lastRun } : {}),
    ...(webhookPath ? { webhookPath } : {}),
    ...(isTrigger(entity) && entity.event === "webhook"
      ? { publicLinkLive: entity.ingress?.reachability === "live" }
      : {}),
    canRunNow: kind === "job",
    canEdit: entity.source === "dynamic",
    detailPath: detailPathOf(kind, entity.id),
  };
}

/** Listing order: name (case-insensitive), schedules before events on ties. */
export function compareAutomationViews(left: AutomationView, right: AutomationView): number {
  const byName = left.name.localeCompare(right.name, "en", { sensitivity: "base" });
  if (byName !== 0) return byName;
  if (left.kind === right.kind) return 0;
  return left.kind === "job" ? -1 : 1;
}

/** `In 14h` / `next run` for schedules, `2h ago` / `last ran` for events; null value = faint `—`. */
export function automationTimeStat(view: AutomationView): { value: string | null; label: string } {
  if (view.kind === "job") {
    return {
      value: view.enabled && view.nextRunAt ? formatRelativeTime(view.nextRunAt) : null,
      label: "next run",
    };
  }
  const at = view.lastRun?.startedAt ?? view.lastRun?.endedAt;
  return { value: at ? formatRelativeTime(at) : null, label: "last ran" };
}

/** Location label: `Project checkout-api` or `Global`. */
export function automationLocationLabel(
  view: Pick<AutomationView, "scope" | "workspaceId" | "workspaceName">
): string {
  if (view.scope !== "workspace") return automationScopeLabel(view.scope);
  const name = view.workspaceName ?? view.workspaceId;
  return name ? `Project ${name}` : automationScopeLabel(view.scope);
}
