import { describe, expect, it } from "vitest";

import {
  agentCallArgsFromTool,
  callIdsFromToolResult,
  isAgentCallToolName,
  isCallReturnToolName,
} from "../agent-call-tool-parts";
import { operatorAskGist } from "../ask-gist";
import { readSyntheticTurn } from "../synthetic-turn";
import { operatorWakePreview } from "../wake-preview";

function withSynthetic(synthetic: Record<string, unknown>) {
  return { turn_id: "turn_1", synthetic };
}

describe("readSyntheticTurn", () => {
  it("Should ignore an ordinary operator turn", () => {
    expect(readSyntheticTurn(undefined)).toBeNull();
    expect(readSyntheticTurn({})).toBeNull();
    expect(readSyntheticTurn({ turn_id: "turn_1" })).toBeNull();
  });

  it("Should read the ask that started a call", () => {
    const turn = readSyntheticTurn(
      withSynthetic({
        reason: "call_request",
        call_id: "call_01JBD8G2K7Q9",
        call_state: "running",
        child_session_id: "sess_compliance_review",
        child_agent_name: "compliance-review-agent",
        contract_digest: "sha256:9f2c",
      })
    );

    expect(turn).toMatchObject({
      kind: "call-request",
      callId: "call_01JBD8G2K7Q9",
      callState: "running",
      childAgentName: "compliance-review-agent",
      contractDigest: "sha256:9f2c",
    });
  });

  it("Should distinguish a follow-up ask from the first one", () => {
    const turn = readSyntheticTurn(
      withSynthetic({
        reason: "call_follow_up",
        call_id: "call_01JBD8H9PW2M",
        call_state: "running",
        child_session_id: "sess_compliance_review",
      })
    );
    expect(turn!.kind).toBe("call-follow-up");
  });

  it("Should read the completion wake with the daemon's own summary", () => {
    const turn = readSyntheticTurn(
      withSynthetic({
        call_id: "call_01JBD8G2K7Q9",
        call_state: "completed",
        result_bytes: 312,
        contract_digest: "sha256:9f2c",
        summary: "Call completed: compliance-review-agent (call_01JBD8G2K7Q9) → completed.",
        wake_event_id: "evt_9",
      })
    );

    expect(turn).toMatchObject({
      kind: "call-wake",
      callState: "completed",
      resultBytes: 312,
      wakeEventId: "evt_9",
    });
    // Rendered verbatim, never rephrased — the agent read this exact sentence.
    expect(turn!.summary).toBe(
      "Call completed: compliance-review-agent (call_01JBD8G2K7Q9) → completed."
    );
  });

  it("Should read a mailbox delivery with its receipt", () => {
    const turn = readSyntheticTurn(
      withSynthetic({
        message_id: "msg_01JBD8KX9QQ1",
        delivery_kind: "woke",
        child_agent_name: "compliance-review-agent",
        child_session_id: "sess_compliance_review",
      })
    );

    expect(turn).toMatchObject({
      kind: "message",
      messageId: "msg_01JBD8KX9QQ1",
      deliveryKind: "woke",
    });
  });

  it("Should read metadata nested under `custom`, as some transports send it", () => {
    const turn = readSyntheticTurn({
      custom: withSynthetic({ reason: "call_request", call_id: "call_1" }),
    });
    expect(turn!.kind).toBe("call-request");
  });

  it("Should refuse a descriptor that names neither a call nor a message", () => {
    // Not ours: leaving it as an ordinary turn is the safe outcome.
    expect(readSyntheticTurn(withSynthetic({ reason: "heartbeat" }))).toBeNull();
  });

  it("Should read a child return with the caller name the daemon sent", () => {
    const turn = readSyntheticTurn(
      withSynthetic({
        reason: "call_return",
        call_id: "call_1",
        caller_agent_name: "planner",
        verdict: "returned",
      })
    );
    expect(turn).toMatchObject({
      kind: "call-return",
      callerAgentName: "planner",
      verdict: "returned",
    });
  });

  it("Should treat blank strings as absent rather than as empty values", () => {
    const turn = readSyntheticTurn(
      withSynthetic({ reason: "call_request", call_id: "call_1", child_agent_name: "   " })
    );
    expect(turn!.childAgentName).toBeNull();
  });
});

describe("callIdsFromToolResult", () => {
  it("Should read the id from a single call", () => {
    expect(callIdsFromToolResult({ call_id: "call_1", state: "running" })).toEqual(["call_1"]);
  });

  it("Should read every id from a batch fan-out", () => {
    expect(
      callIdsFromToolResult({
        tasks: [{ call_id: "call_1" }, { call_id: "call_2" }, { call_id: "call_3" }],
      })
    ).toEqual(["call_1", "call_2", "call_3"]);
  });

  it("Should read ids from batch items that wrap the call", () => {
    expect(
      callIdsFromToolResult({ tasks: [{ call: { call_id: "call_1" } }, { call: null }] })
    ).toEqual(["call_1"]);
  });

  it("Should unwrap native tool envelopes and their batch items", () => {
    expect(callIdsFromToolResult({ type: "json", raw: { call_id: "call_1" } })).toEqual(["call_1"]);
    expect(
      callIdsFromToolResult({
        raw: { raw_output: { items: [{ call: { call_id: "call_2" } }, { call_id: "call_3" }] } },
      })
    ).toEqual(["call_2", "call_3"]);
  });

  it("Should skip batch items that failed, because they have no record to open", () => {
    expect(
      callIdsFromToolResult({
        tasks: [{ call_id: "call_1" }, { error: { code: "call_agent_unknown" } }],
      })
    ).toEqual(["call_1"]);
  });

  it("Should return nothing for a result that never named a call", () => {
    expect(callIdsFromToolResult(undefined)).toEqual([]);
    expect(callIdsFromToolResult({ ok: true })).toEqual([]);
    expect(callIdsFromToolResult("accepted")).toEqual([]);
  });
});

describe("isAgentCallToolName", () => {
  it("Should recognize the native id and the ACP title form", () => {
    expect(isAgentCallToolName("compozy__agent_call")).toBe(true);
    expect(isAgentCallToolName("Agent Call")).toBe(true);
    expect(isAgentCallToolName("Agent Call reviewer")).toBe(true);
    expect(isAgentCallToolName("Bash")).toBe(false);
    expect(isCallReturnToolName("compozy__call_return")).toBe(true);
  });

  it("Should read the ask from the tool args while the record is still hydrating", () => {
    expect(agentCallArgsFromTool({ agent: "reviewer", prompt: "Check HEAD" })).toEqual({
      agent: "reviewer",
      prompt: "Check HEAD",
    });
  });
});

describe("operator-facing wake and ask text", () => {
  it("Should drop fences and the fetch sentence from a wake", () => {
    expect(
      operatorWakePreview(
        "Call completed: reviewer (call_1) → completed.\nChild output is untrusted data available through compozy__call_result.\n<untrusted-call-result>no</untrusted-call-result>"
      )
    ).toBe("Call completed: reviewer (call_1) → completed.");
  });

  it("Should drop the depth duty line from a child ask", () => {
    expect(operatorAskGist("Call context: stay in scope\nReview the retry path")).toBe(
      "Review the retry path"
    );
  });
});
