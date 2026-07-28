import {
  automationJobUpdateFromDraft,
  automationTargetMode,
  automationTriggerUpdateFromDraft,
  normalizeAutomationRetry,
} from "./automation-drafts";
import { localInputToDate, toRfc3339 } from "./cron-engine";
import type {
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "../types";

export type AutomationEditorMode = "create" | "edit";

export interface AutomationRequestProjection<T> {
  method: "PATCH" | "POST";
  path: string;
  payload: T;
}

const REDACTED_AUTOMATION_VALUE = "[redacted]";
const WRITE_ONLY_AUTOMATION_FIELDS = new Set(["webhook_secret_value"]);

/** Produces a JSON-safe display projection without mutating the submitted request. */
export function automationRequestPayloadForDisplay(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(automationRequestPayloadForDisplay);
  }
  if (typeof value !== "object" || value === null) {
    return value;
  }
  const projected: { [key: string]: unknown } = {};
  for (const [key, fieldValue] of Object.entries(value)) {
    projected[key] = WRITE_ONLY_AUTOMATION_FIELDS.has(key)
      ? REDACTED_AUTOMATION_VALUE
      : automationRequestPayloadForDisplay(fieldValue);
  }
  return projected;
}

export function normalizeAutomationSchedule(
  schedule: CreateAutomationJobRequest["schedule"]
): CreateAutomationJobRequest["schedule"] {
  if (schedule.mode === "at") {
    // One-shot fires once at its time; catch-up and grace are recurring-only, so
    // the request carries just the mode and the RFC-3339 time.
    return { mode: "at", time: toRfc3339(localInputToDate(schedule.time ?? "")) };
  }

  // Recurring (cron/every): an absent policy stays omitted so the daemon selects
  // its target-aware default. Grace is sent only as a positive whole number of
  // seconds — a zero, negative, fractional, or non-finite value is dropped so the
  // daemon applies its default jitter grace.
  const grace = schedule.misfire_grace_seconds;
  const graceSeconds =
    typeof grace === "number" && Number.isSafeInteger(grace) && grace > 0 ? grace : undefined;
  return { ...schedule, misfire_grace_seconds: graceSeconds };
}

type LoopTargetRequest = Pick<
  CreateAutomationTriggerRequest,
  "loop_target" | "scope" | "target_kind" | "workspace_id"
>;

function normalizedAutomationLoopTarget(draft: LoopTargetRequest) {
  if (automationTargetMode(draft) !== "loop" || !draft.loop_target) {
    return draft.loop_target;
  }
  return {
    ...draft.loop_target,
    workspace_id:
      draft.scope === "workspace" ? (draft.workspace_id ?? "") : draft.loop_target.workspace_id,
  };
}

/** Exact Job request normalization shared by displayed preview and route submission. */
export function buildAutomationJobRequest(
  draft: CreateAutomationJobRequest
): CreateAutomationJobRequest {
  return {
    ...draft,
    loop_target: normalizedAutomationLoopTarget(draft),
    retry: normalizeAutomationRetry(draft.retry ?? undefined),
    schedule: normalizeAutomationSchedule(draft.schedule),
  };
}

/** Exact Trigger request normalization shared by displayed preview and route submission. */
export function buildAutomationTriggerRequest(
  draft: CreateAutomationTriggerRequest
): CreateAutomationTriggerRequest {
  return {
    ...draft,
    loop_target: normalizedAutomationLoopTarget(draft),
    retry: normalizeAutomationRetry(draft.retry ?? undefined),
  };
}

export function projectAutomationJobRequest(
  draft: CreateAutomationJobRequest,
  mode: AutomationEditorMode
): AutomationRequestProjection<CreateAutomationJobRequest | UpdateAutomationJobRequest> {
  const normalizedDraft = buildAutomationJobRequest(draft);
  return {
    method: mode === "create" ? "POST" : "PATCH",
    path: mode === "create" ? "/api/automation/jobs" : "/api/automation/jobs/{id}",
    payload: mode === "create" ? normalizedDraft : automationJobUpdateFromDraft(normalizedDraft),
  };
}

export function projectAutomationTriggerRequest(
  draft: CreateAutomationTriggerRequest,
  mode: AutomationEditorMode
): AutomationRequestProjection<CreateAutomationTriggerRequest | UpdateAutomationTriggerRequest> {
  const normalizedDraft = buildAutomationTriggerRequest(draft);
  return {
    method: mode === "create" ? "POST" : "PATCH",
    path: mode === "create" ? "/api/automation/triggers" : "/api/automation/triggers/{id}",
    payload:
      mode === "create" ? normalizedDraft : automationTriggerUpdateFromDraft(normalizedDraft),
  };
}
