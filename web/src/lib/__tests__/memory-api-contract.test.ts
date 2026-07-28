import { describe, expectTypeOf, it } from "vitest";

import type {
  OperationId,
  OperationPath,
  OperationQuery,
  OperationRequestBody,
  OperationResponse,
} from "../api-contract";

describe("memory API generated contract types", () => {
  it("exposes the generated Memory v2 operation ids through the API contract helper", () => {
    type MemoryOperations =
      | "listMemory"
      | "writeMemory"
      | "editMemory"
      | "searchMemory"
      | "listMemoryDecisions"
      | "getMemoryDecision"
      | "revertMemoryDecision"
      | "getMemoryRecallTrace"
      | "triggerMemoryDream"
      | "getMemorySessionLedger";

    expectTypeOf<Extract<OperationId, MemoryOperations>>().toEqualTypeOf<MemoryOperations>();
    expectTypeOf<Extract<OperationId, "consolidateMemory">>().toEqualTypeOf<never>();
  });

  it("keeps generated Memory v2 selector and mutation payloads aligned", () => {
    type ListQuery = NonNullable<OperationQuery<"listMemory">>;
    type WriteRequest = OperationRequestBody<"writeMemory">;
    type EditRequest = OperationRequestBody<"editMemory">;
    type RevertRequest = OperationRequestBody<"revertMemoryDecision">;
    type RecallTracePath = OperationPath<"getMemoryRecallTrace">;

    expectTypeOf<ListQuery>().toMatchTypeOf<{
      scope?: "global" | "workspace" | "agent";
      workspace_id?: string;
      agent_name?: string;
      agent_tier?: "workspace" | "global";
    }>();
    expectTypeOf<Extract<keyof ListQuery, "workspace">>().toEqualTypeOf<never>();

    expectTypeOf<WriteRequest>().toMatchTypeOf<{
      scope: "global" | "workspace" | "agent";
      type: "user" | "feedback" | "project" | "reference";
      name: string;
      content: string;
      workspace_id?: string;
      agent_name?: string;
      agent_tier?: "workspace" | "global";
    }>();
    expectTypeOf<Extract<keyof WriteRequest, "workspace">>().toEqualTypeOf<never>();

    expectTypeOf<EditRequest>().toMatchTypeOf<{ content: string; expected_hash?: string }>();
    expectTypeOf<RevertRequest>().toMatchTypeOf<{ dry_run?: boolean; reason?: string }>();
    expectTypeOf<RecallTracePath>().toEqualTypeOf<{
      session_id: string;
      turn_seq: number;
    }>();
  });

  it("keeps public decision and error payloads redaction-safe", () => {
    type Decision = OperationResponse<"getMemoryDecision", 200>["decision"];
    type LLMTrace = NonNullable<Decision["llm_trace"]>;
    type MemoryError = OperationResponse<"writeMemory", 400>;

    expectTypeOf<Decision>().toMatchTypeOf<{
      id: string;
      candidate_hash: string;
      op: "noop" | "add" | "update" | "delete" | "reject";
      post_content_hash?: string;
      workspace_id?: string;
    }>();
    expectTypeOf<
      Extract<keyof Decision, "post_content" | "prior_content" | "raw_response">
    >().toEqualTypeOf<never>();
    expectTypeOf<LLMTrace>().toMatchTypeOf<{
      model: string;
      prompt_version: string;
      latency_ms: number;
      error?: string;
    }>();
    expectTypeOf<
      Extract<keyof LLMTrace, "raw_response" | "prompt" | "completion">
    >().toEqualTypeOf<never>();

    expectTypeOf<MemoryError>().toMatchTypeOf<{
      code: string;
      message: string;
      details?: { [key: string]: unknown };
    }>();
    expectTypeOf<Extract<keyof MemoryError, "error">>().toEqualTypeOf<never>();
  });
});
