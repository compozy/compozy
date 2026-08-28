import type { operations } from "@/generated/compozy-openapi";
import { z } from "zod";

type TerminalListResponse =
  operations["listTerminals"]["responses"][200]["content"]["application/json"];
type TerminalResponse = operations["getTerminal"]["responses"][200]["content"]["application/json"];
type TerminalExitResponse =
  operations["deleteTerminal"]["responses"][200]["content"]["application/json"];
type TerminalAttachTicketResponse =
  operations["mintTerminalAttachTicket"]["responses"][201]["content"]["application/json"];
type TerminalSignalResponse =
  operations["signalTerminal"]["responses"][200]["content"]["application/json"];
type TerminalInputAnswerResponse =
  operations["answerTerminalInputRequest"]["responses"][200]["content"]["application/json"];
type TerminalInputRejectResponse =
  operations["rejectTerminalInputRequest"]["responses"][200]["content"]["application/json"];
type TerminalJournalResponse =
  operations["queryTerminalJournal"]["responses"][200]["content"]["application/json"];
type TerminalRecordingResponse =
  operations["controlTerminalRecording"]["responses"][200]["content"]["application/json"];
type TerminalWaitResponse =
  operations["waitTerminal"]["responses"][200]["content"]["application/json"];

export type TerminalInfo = TerminalListResponse["terminals"][number];

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

type GeneratedApiReasonCode = NonNullable<
  operations["listTools"]["responses"][200]["content"]["application/json"]["tools"][number]["availability"]["reason_codes"]
>[number];

/**
 * Domain refusals in the generated reason-code union.
 *
 * Terminal HTTP `error.code` is an open string in OpenAPI. The same frozen
 * vocabulary is emitted as string-literal members of this generated union, so
 * a new `terminal_*` / `profile_*` / input / recording / ticket code fails
 * `closedGeneratedEnum` until the runtime list is updated.
 */
export type GeneratedTerminalErrorCode = Extract<
  GeneratedApiReasonCode,
  | `terminal_${string}`
  | `profile_${string}`
  | `input_request_${string}`
  | `input_answer_${string}`
  | `recording_${string}`
  | `ticket_${string}`
  | "invalid_cwd"
  | "timeout_out_of_range"
  | "write_owner_held"
  | "lease_revoked"
  | "generation_fenced"
  | "typing_grant_rejected"
  | "approval_rejected"
  | "slow_consumer"
  | "journal_unavailable"
  | "subscriber_limit_reached"
>;

export const terminalErrorCodeSchema = closedGeneratedEnum<GeneratedTerminalErrorCode>()([
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
export type TerminalErrorCode = z.infer<typeof terminalErrorCodeSchema>;

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
const terminalSequenceSchema = z
  .string()
  .regex(/^(0|[1-9]\d*)$/)
  .transform(value => BigInt(value))
  .refine(value => value <= 0xffff_ffff_ffff_ffffn, "sequence exceeds u64");

export const terminalReadResponseSchema = z.strictObject({
  busy: z.boolean(),
  content: z.string(),
  seq: terminalSequenceSchema,
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
const terminalInputActorSchema = z.strictObject({
  kind: terminalActorKindSchema,
  id: z.string().min(1),
});
const terminalPendingInputRequestSchema = z.strictObject({
  id: z.string(),
  profile_id: z.string(),
  profile_name: z.string(),
  prompt_excerpt: z.string(),
  reason: z.string(),
  redacted: z.boolean(),
  requested_at: dateTimeSchema,
  requester: terminalInputActorSchema,
  terminal_id: z.string(),
  workspace_id: z.string().optional(),
});
const terminalResolvedInputRequestSchema = z.strictObject({
  id: z.string(),
  profile_id: z.string(),
  profile_name: z.string(),
  requester: terminalInputActorSchema,
  outcome: z.enum(["answered", "rejected", "superseded", "expired"]),
  resolved_by: terminalInputActorSchema,
  reason: z.string().optional(),
  redacted: z.boolean(),
  length: z.number().int().nonnegative(),
  requested_at: dateTimeSchema,
  resolved_at: dateTimeSchema,
  terminal_id: z.string(),
  workspace_id: z.string().optional(),
});
export const terminalInputRequestsResponseSchema = z.strictObject({
  pending: z.array(terminalPendingInputRequestSchema),
  resolved: z.array(terminalResolvedInputRequestSchema),
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
export const terminalWaitUntilSchema = z.enum(["exit", "idle", "match"]);
export const terminalWaitResponseSchema: z.ZodType<TerminalWaitResponse> = z.strictObject({
  exit_code: z.number().int().nullable().optional(),
  reason: z.string(),
  screen: z.string(),
  untrusted: z.boolean(),
});
export const terminalErrorDetailsSchema = z.strictObject({
  current: z.number().int().nonnegative().optional(),
  max: z.number().int().positive().optional(),
  controller: terminalInputActorSchema.optional(),
  path: z.string().optional(),
  mode: terminalModeSchema.optional(),
  platform: z.string().optional(),
  action: z.string().optional(),
});
export type TerminalErrorDetails = z.infer<typeof terminalErrorDetailsSchema>;

export const terminalErrorEnvelopeSchema = z.strictObject({
  error: z.strictObject({
    code: z.string().regex(/\S/),
    message: z.string().regex(/\S/),
    details: terminalErrorDetailsSchema.optional(),
  }),
});
