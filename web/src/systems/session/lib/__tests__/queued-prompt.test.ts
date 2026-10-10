import { describe, expect, it } from "vitest";

import {
  queueCapFromInputs,
  queuedPromptOwner,
  queuedPromptsFromInputs,
  withCachedInputs,
} from "../queued-prompt";
import { queuedPromptPreview } from "../queued-prompt-preview";
import type { SessionInputPayload } from "../../types";

// Suite: queue strip read model (UT-101 preview flattening, owner attribution, positions).
// Invariant: every row is the daemon's entry in the daemon's order; the preview is one
// safe line derived from the text; attribution derives from the daemon's owner pair.
// Boundary IN: `GET …/prompt/queue` payloads. Boundary OUT: SessionQueueStrip props.
describe("queued prompt read model", () => {
  it("Should flatten headings, lists, and quotes to one safe preview line (UT-101)", () => {
    expect(queuedPromptPreview("# Ship it\n\nwith tests")).toEqual({
      kind: "text",
      text: "Ship it",
    });
    expect(queuedPromptPreview("\n\n- [ ] Update the changelog\n- second")).toEqual({
      kind: "text",
      text: "Update the changelog",
    });
    expect(queuedPromptPreview("> quoted first line\r\nsecond")).toEqual({
      kind: "text",
      text: "quoted first line",
    });
    expect(queuedPromptPreview("2) numbered")).toEqual({ kind: "text", text: "numbered" });
    expect(queuedPromptPreview("   \n\t")).toEqual({ kind: "text", text: "Queued message" });
    expect(queuedPromptPreview("###")).toEqual({ kind: "text", text: "Queued message" });
  });

  it("Should read a leading fence as code with the first line inside it", () => {
    expect(queuedPromptPreview("```go\nfunc TestRetry(t *testing.T) {\n```")).toEqual({
      kind: "code",
      text: "func TestRetry(t *testing.T) {",
    });
    expect(queuedPromptPreview("~~~\n\n~~~")).toEqual({ kind: "code", text: "Code block" });
  });

  it("Should attribute only other actors and never the operator to themselves", () => {
    expect(queuedPromptOwner(undefined, undefined)).toBeNull();
    expect(queuedPromptOwner("", "agent_1")).toBeNull();
    expect(queuedPromptOwner("user", "u_1")).toBeNull();
    expect(queuedPromptOwner("agent", " agent_42 ")).toEqual({ id: "agent_42", kind: "agent" });
    expect(queuedPromptOwner("coordinator", "")).toEqual({ id: null, kind: "coordinator" });
  });

  it("Should number rows by the daemon's list order without counting on its own", () => {
    const input = (id: string, status: SessionInputPayload["status"]): SessionInputPayload => ({
      delivery: "after_turn",
      enqueued_at: "2026-09-06T10:00:00Z",
      id,
      mode: "queue",
      queue_generation: 1,
      session_id: "sess-1",
      status,
      text: `${id} text`,
    });
    const rows = queuedPromptsFromInputs(
      [
        input("inq-b", "dispatching"),
        { ...input("inq-a", "queued"), owner_id: "reviewer", owner_kind: "agent" },
      ],
      "ws",
      "sess-1"
    );
    expect(rows.map(row => [row.id, row.position, row.status, row.owner])).toEqual([
      ["inq-b", 1, "dispatching", null],
      ["inq-a", 2, "queued", { id: "reviewer", kind: "agent" }],
    ]);
    expect(queuedPromptsFromInputs(undefined, "ws", "sess-1")).toEqual([]);
  });

  it("Should map a session message's origin to its sender and leave other rows without one (UT-064)", () => {
    const base: SessionInputPayload = {
      delivery: "after_turn",
      enqueued_at: "2026-10-09T22:30:12Z",
      id: "inp-41aa",
      mode: "queue",
      queue_generation: 1,
      session_id: "sess-c03f",
      status: "queued",
      text: "Is the retry budget per request or per job?",
    };
    const origin = {
      kind: "session",
      session_id: "sess-7f3a",
      workspace_id: "ws",
      agent_name: "claude",
      title_at_send: "Refactor billing",
      hop: 1,
      notify_on_complete: true,
      reply_watch_id: "rw-6e2d81a0",
    };
    const [sent, own] = queuedPromptsFromInputs(
      [{ ...base, origin } as SessionInputPayload, { ...base, id: "inp-41ab" }],
      "ws",
      "sess-c03f"
    );
    expect(sent?.sender).toMatchObject({
      sessionId: "sess-7f3a",
      agentName: "claude",
      titleAtSend: "Refactor billing",
    });
    expect(sent?.owner).toBeNull();
    expect(own).not.toHaveProperty("sender");
  });

  it("Should read the cap only from the daemon's queue summary and never invent one", () => {
    expect(queueCapFromInputs(undefined)).toBeNull();
    expect(queueCapFromInputs({ inputs: [] })).toBeNull();
    expect(queueCapFromInputs({ inputs: [], queue: null })).toBeNull();
    expect(queueCapFromInputs({ inputs: [], queue: { cap: 0, entries: 0 } })).toBeNull();
    expect(queueCapFromInputs({ inputs: [], queue: { cap: 10, entries: 3 } })).toBe(10);
  });

  it("Should keep the daemon's queue summary across list-only cache writes", () => {
    const summary = { cap: 10, entries: 2 };
    expect(withCachedInputs({ inputs: [], queue: summary }, [])).toEqual({
      inputs: [],
      queue: summary,
    });
    // A response that predates the summary stays without one — nothing is fabricated.
    expect(withCachedInputs({ inputs: [] }, [])).toEqual({ inputs: [] });
    expect(withCachedInputs(undefined, [])).toEqual({ inputs: [] });
  });
});
