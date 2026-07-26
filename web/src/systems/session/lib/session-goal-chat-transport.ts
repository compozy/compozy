import type { SessionGoalCommandResult } from "../types";

interface GoalAwareFetchOptions {
  fetch?: typeof globalThis.fetch;
  onRequest?: () => void;
  onResult: (result: SessionGoalCommandResult, requestText: string | null) => void;
}

type GoalCommandFailureReason = Exclude<SessionGoalCommandResult["reason_code"], null>;

const GOAL_COMMAND_FAILURE_GUIDANCE = {
  goal_not_active: "Start a Goal before using this control, then try again.",
  goal_replace_required:
    "A Goal is already active. Replace it explicitly or keep the current Goal.",
  goal_replace_stale: "The active Goal changed before replacement. Refresh its status, then retry.",
  goal_judge_unavailable:
    "Goal judge is not configured. Set loops.defaults.delivery.model_defaults.judge or use a session created with an explicit model, then retry.",
  goal_objective_required: "Add an objective after /goal, then try again.",
  goal_objective_too_large: "Shorten the Goal objective, then try again.",
  goal_contract_clause_empty: "Complete every Goal contract clause or remove the empty clause.",
  goal_command_invalid: "Check the Goal command syntax, then try again.",
  goal_draft_requires_idle: "Wait for the active Goal work to become idle before saving a draft.",
  goal_evidence_too_large: "Reduce the Goal evidence payload, then try again.",
  goal_report_conflict: "The Goal report changed concurrently. Refresh the Goal and submit again.",
  goal_session_busy_queue_full:
    "The Goal session queue is full. Wait for current work to settle, then retry.",
  goal_turn_filter_invalid: "Adjust the Goal turn filters, then try again.",
  goal_judge_broken:
    "The Goal judge failed repeatedly. Inspect the judge model and rubric before retrying.",
  goal_judge_outcome_invalid:
    "The Goal judge returned an invalid verdict. Inspect its output and rubric.",
  goal_action_control_invalid:
    "This Goal control is not valid in the current state. Refresh status first.",
  goal_stop_reason_invalid: "Choose a supported Goal stop reason, then try again.",
  goal_prompt_request_failed:
    "The Goal prompt could not be submitted. Check the session and retry.",
  goal_agent_refused:
    "The agent refused the Goal request. Review the objective and session policy.",
  goal_compaction_cancelled:
    "Goal compaction was cancelled. Retry after the active operation settles.",
  goal_recovery_ambiguous:
    "Goal recovery found conflicting state. Inspect Goal status before continuing.",
  goal_control_revoked_in_flight:
    "This Goal control was revoked while running. Refresh status before retrying.",
  goal_reseed_confirmation_required:
    "Confirm the Goal reseed before continuing with a fresh session binding.",
  goal_presubmit_retries_exhausted:
    "Goal submission retries were exhausted. Inspect session health, then retry.",
  goal_budget_fenced: "The Goal reached its budget fence. Approve more budget or stop the Goal.",
  goal_origin_invalid: "The Goal origin is invalid. Start it again from the intended session.",
  goal_origin_workspace_mismatch:
    "The Goal origin belongs to another workspace. Return to its workspace first.",
  goal_origin_profile_unavailable:
    "The Goal creation profile is unavailable. Restore it or start a new Goal.",
  goal_control_stale: "The Goal changed before this control applied. Refresh status, then retry.",
  goal_prompt_fenced:
    "The Goal prompt was fenced by newer control state. Refresh status before continuing.",
  session_creation_identity_mismatch:
    "The Goal session identity changed during creation. Retry from the active session.",
  continuous_binding_mismatch:
    "The Goal session binding no longer matches. Refresh status and rebind before retrying.",
} satisfies Record<GoalCommandFailureReason, string>;

function isGoalCommandFailureReason(value: unknown): value is GoalCommandFailureReason {
  return typeof value === "string" && Object.hasOwn(GOAL_COMMAND_FAILURE_GUIDANCE, value);
}

function requestText(init?: RequestInit): string | null {
  if (typeof init?.body !== "string") return null;
  try {
    const body: unknown = JSON.parse(init.body);
    if (typeof body !== "object" || body === null) return null;
    if ("message" in body && typeof body.message === "string") return body.message;
    if (!("messages" in body) || !Array.isArray(body.messages)) return null;
    const last = body.messages.at(-1);
    if (
      typeof last !== "object" ||
      last === null ||
      !("parts" in last) ||
      !Array.isArray(last.parts)
    ) {
      return null;
    }
    return (
      (last.parts as unknown[])
        .filter(
          (part): part is Record<string, unknown> =>
            typeof part === "object" && part !== null && "type" in part && part.type === "text"
        )
        .map(part => (typeof part.text === "string" ? part.text : ""))
        .join("\n")
        .trim() || null
    );
  } catch {
    return null;
  }
}

function isJsonContentType(value: string | null): boolean {
  return value?.toLowerCase().includes("json") ?? false;
}

function isGoalCommandResult(value: unknown): value is SessionGoalCommandResult {
  return (
    typeof value === "object" &&
    value !== null &&
    "outcome" in value &&
    typeof value.outcome === "string"
  );
}

function completionBody(messageId: string): string {
  return [
    `data: ${JSON.stringify({ type: "start", messageId })}\n\n`,
    `data: ${JSON.stringify({ type: "finish", finishReason: "stop" })}\n\n`,
  ].join("");
}

export function describeGoalCommandFailure(reasonCode: unknown): string | null {
  if (isGoalCommandFailureReason(reasonCode)) {
    return GOAL_COMMAND_FAILURE_GUIDANCE[reasonCode];
  }
  return null;
}

export function isGoalCommandFailureGuidance(message: string): boolean {
  return Object.values(GOAL_COMMAND_FAILURE_GUIDANCE).some(guidance => guidance === message);
}

function goalCommandFailureMessage(reasonCode: unknown): string {
  return (
    describeGoalCommandFailure(reasonCode) ??
    "Goal command failed. Refresh Goal status, then retry."
  );
}

export function createGoalAwareFetch({
  fetch = globalThis.fetch.bind(globalThis),
  onRequest,
  onResult,
}: GoalAwareFetchOptions): typeof globalThis.fetch {
  let sequence = 0;
  return async (input, init) => {
    onRequest?.();
    const submittedText = requestText(init);
    const response = await fetch(input, init);
    if (!isJsonContentType(response.headers.get("content-type"))) {
      return response;
    }

    let payload: unknown;
    try {
      payload = await response.clone().json();
    } catch {
      return response;
    }
    if (!isGoalCommandResult(payload)) {
      return response;
    }

    onResult(payload, submittedText);
    if (!response.ok) {
      return new Response(goalCommandFailureMessage(payload.reason_code), {
        status: response.status,
        statusText: response.statusText,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    }

    sequence += 1;
    return new Response(completionBody(`goal-command-${sequence}`), {
      status: response.status,
      statusText: response.statusText,
      headers: { "content-type": "text/event-stream" },
    });
  };
}
