import type { operations } from "@/generated/compozy-openapi";
import { z } from "zod";

type TerminalListResponse =
  operations["listTerminals"]["responses"][200]["content"]["application/json"];
type TerminalResponse = operations["getTerminal"]["responses"][200]["content"]["application/json"];
type TerminalExitResponse =
  operations["deleteTerminal"]["responses"][200]["content"]["application/json"];
type TerminalAttachTicketResponse =
  operations["mintTerminalAttachTicket"]["responses"][201]["content"]["application/json"];
type TerminalReadResponse =
  operations["readTerminal"]["responses"][200]["content"]["application/json"];
type TerminalSignalResponse =
  operations["signalTerminal"]["responses"][200]["content"]["application/json"];
type TerminalInputRequestsResponse =
  operations["listTerminalInputRequests"]["responses"][200]["content"]["application/json"];
type TerminalInputAnswerResponse =
  operations["answerTerminalInputRequest"]["responses"][200]["content"]["application/json"];
type TerminalInputRejectResponse =
  operations["rejectTerminalInputRequest"]["responses"][200]["content"]["application/json"];
type TerminalJournalResponse =
  operations["queryTerminalJournal"]["responses"][200]["content"]["application/json"];
type TerminalRecordingResponse =
  operations["controlTerminalRecording"]["responses"][200]["content"]["application/json"];
type TerminalErrorEnvelope =
  operations["listTerminals"]["responses"][422]["content"]["application/json"];

export type TerminalInfo = TerminalListResponse["terminals"][number];
export type TerminalErrorCode = TerminalErrorEnvelope["error"]["code"];

function closedGeneratedEnum<Generated extends string>() {
  return <const Values extends readonly [Generated, ...Generated[]]>(
    values: Values & (Exclude<Generated, Values[number]> extends never ? unknown : never)
  ) => z.enum(values);
}

export const terminalActorKindSchema = closedGeneratedEnum<
  NonNullable<TerminalInfo["controller"]>["kind"]
>()(["human", "agent", "system"]);
export const terminalLeaseStateSchema = closedGeneratedEnum<TerminalInfo["lease"]>()([
  "human_owned",
  "agent_owned",
  "available",
]);
export const terminalModeSchema = closedGeneratedEnum<TerminalInfo["mode"]>()(["pty", "pipe"]);
export const terminalStateSchema = closedGeneratedEnum<TerminalInfo["state"]>()([
  "running",
  "exited",
]);
export const terminalExitCauseSchema = closedGeneratedEnum<
  NonNullable<TerminalInfo["exit"]>["cause"]
>()(["exited", "signaled", "unknown"]);
export const terminalSignalSchema = closedGeneratedEnum<
  NonNullable<NonNullable<TerminalInfo["exit"]>["signal"]>
>()(["INT", "TERM", "KILL", "HUP"]);

const terminalDetectedBySchema = closedGeneratedEnum<
  TerminalJournalResponse["entries"][number]["detected_by"]
>()(["exact", "marker", "idle"]);
const terminalApprovalSchema = closedGeneratedEnum<
  TerminalJournalResponse["entries"][number]["approval"]
>()(["approved_once", "approved_always", "allowlisted", "human", "none"]);
const terminalRecordingStateSchema = closedGeneratedEnum<
  TerminalRecordingResponse["recording"]["state"]
>()(["recording", "saved"]);

export const terminalErrorCodeSchema = closedGeneratedEnum<TerminalErrorCode>()([
  "terminal_not_found",
  "profile_selection_conflict",
  "profile_session_conflict",
  "terminal_requires_workspace",
  "profile_archived",
  "profile_unavailable",
  "terminal_limit_reached",
  "subscriber_limit_reached",
  "terminal_exited",
  "terminal_expired",
  "terminal_interactive_unavailable",
  "terminal_not_interactive",
  "invalid_cwd",
  "timeout_out_of_range",
  "write_owner_held",
  "lease_revoked",
  "generation_fenced",
  "typing_grant_rejected",
  "approval_rejected",
  "ticket_invalid",
  "ticket_expired",
  "input_request_not_found",
  "input_request_already_answered",
  "input_request_superseded",
  "input_request_limit_reached",
  "input_answer_requires_write",
  "recording_already_started",
  "recording_not_active",
  "recording_unavailable",
  "slow_consumer",
  "journal_unavailable",
]);

const dateTimeSchema = z.iso.datetime({ offset: true });
const terminalActorSchema = z.strictObject({
  id: z.string(),
  kind: terminalActorKindSchema,
});
export const terminalExitSchema = z.strictObject({
  at: dateTimeSchema,
  cause: terminalExitCauseSchema,
  code: z.number().int().nullable().optional(),
  signal: terminalSignalSchema.nullable().optional(),
});

export const terminalInfoSchema: z.ZodType<TerminalInfo> = z.strictObject({
  id: z.string(),
  workspace_id: z.string(),
  profile_id: z.string(),
  profile_name: z.string(),
  title: z.string(),
  shell: z.string(),
  cwd: z.string(),
  mode: terminalModeSchema,
  state: terminalStateSchema,
  controller: terminalActorSchema.nullable(),
  lease: terminalLeaseStateSchema,
  viewers: z.number().int().nonnegative(),
  bound_run: z
    .strictObject({
      session_id: z.string(),
      run_id: z.string(),
      generation: z.number().int().nonnegative(),
    })
    .nullable(),
  capabilities: z.strictObject({ interactive: z.boolean() }),
  created_at: dateTimeSchema,
  exit: terminalExitSchema.nullable().optional(),
});

export const terminalListResponseSchema: z.ZodType<TerminalListResponse> = z.strictObject({
  terminals: z.array(terminalInfoSchema),
});
export const terminalResponseSchema: z.ZodType<TerminalResponse> = z.strictObject({
  terminal: terminalInfoSchema,
});
export const terminalExitResponseSchema: z.ZodType<TerminalExitResponse> = z.strictObject({
  exit: terminalExitSchema.nullable(),
});
export const terminalAttachTicketResponseSchema: z.ZodType<TerminalAttachTicketResponse> =
  z.strictObject({
    ticket: z.string(),
    expires_at: dateTimeSchema,
  });
export const terminalReadResponseSchema: z.ZodType<TerminalReadResponse> = z.strictObject({
  busy: z.boolean(),
  content: z.string(),
  seq: z.number().int().nonnegative(),
  spill: z
    .strictObject({
      artifact_id: z.string(),
      bytes: z.number().int().nonnegative(),
    })
    .nullable()
    .optional(),
  truncated: z.boolean(),
  untrusted: z.boolean(),
});
export const terminalSignalResponseSchema: z.ZodType<TerminalSignalResponse> = z.strictObject({
  delivered: z.boolean(),
});
export const terminalInputRequestsResponseSchema: z.ZodType<TerminalInputRequestsResponse> =
  z.strictObject({
    requests: z.array(
      z.strictObject({
        id: z.string(),
        profile_id: z.string(),
        profile_name: z.string(),
        prompt_excerpt: z.string(),
        reason: z.string(),
        redacted: z.boolean(),
        requested_at: dateTimeSchema,
        terminal_id: z.string(),
        workspace_id: z.string().optional(),
      })
    ),
  });
export const terminalInputAnswerResponseSchema: z.ZodType<TerminalInputAnswerResponse> =
  z.strictObject({
    delivered_bytes: z.number().int().nonnegative(),
    redacted: z.boolean(),
  });
export const terminalInputRejectResponseSchema: z.ZodType<TerminalInputRejectResponse> =
  z.strictObject({
    outcome: z.literal("rejected"),
  });
export const terminalJournalResponseSchema: z.ZodType<TerminalJournalResponse> = z.strictObject({
  entries: z.array(
    z.strictObject({
      actor: terminalActorSchema,
      approval: terminalApprovalSchema,
      argv_digest: z.string().nullable().optional(),
      command: z.string(),
      command_id: z.string(),
      cwd: z.string(),
      detected_by: terminalDetectedBySchema,
      duration_ms: z.number().int().nonnegative().nullable(),
      exit_cause: terminalExitCauseSchema,
      exit_code: z.number().int().nullable(),
      output_bytes: z.number().int().nonnegative(),
      profile_id: z.string(),
      profile_name: z.string(),
      recording: z.string().nullable().optional(),
      signal: terminalSignalSchema.nullable(),
      started_at: dateTimeSchema,
      terminal_id: z.string().nullable(),
      truncated: z.boolean(),
    })
  ),
  next: z.string().nullable(),
});
export const terminalRecordingResponseSchema: z.ZodType<TerminalRecordingResponse> = z.strictObject(
  {
    recording: z.strictObject({
      bytes: z.number().int().nonnegative(),
      digest: z.string(),
      expires_at: dateTimeSchema,
      id: z.string(),
      profile_id: z.string(),
      started_at: dateTimeSchema,
      state: terminalRecordingStateSchema,
      stopped_at: dateTimeSchema.nullable().optional(),
      terminal_id: z.string(),
    }),
  }
);
export const terminalErrorEnvelopeSchema: z.ZodType<TerminalErrorEnvelope> = z.strictObject({
  error: z.strictObject({
    code: terminalErrorCodeSchema,
    message: z.string().regex(/\S/),
    details: z.record(z.string(), z.string()).optional(),
  }),
});
