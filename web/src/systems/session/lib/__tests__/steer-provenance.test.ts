import { describe, expect, it } from "vitest";

import { deriveSteerProvenance } from "../steer-provenance";

// Suite: steer provenance derivation (VC-07, BUG-20260906-injected-guidance-missing-history).
// Invariant: every steer marker binds to the message identity the daemon named
// (`evidence.message_id`), never to the preceding bubble or same-turn proximity;
// one identity yields one bubble — the loaded real message owns the meta line,
// guidance never dispatched renders a single receipt with the latest truth, and
// markers without identity attach to nothing. Owning layer: pure lib.
function marker(kind: string, evidence: Record<string, unknown>, summary = "Steer.") {
  return {
    type: "data-compozy-event",
    data: {
      type: "transcript_marker.created",
      marker: { kind, summary, occurred_at: "2026-07-07T12:00:00Z", evidence },
    },
  };
}

const STEERED = "transcript_marker.prompt_steered";
const SUPERSEDED = "transcript_marker.prompt_superseded";
const ACCEPTED = "transcript_marker.prompt_accepted";
const QUEUED = "transcript_marker.prompt_queued";

function user(id: string, text: string, authoredId?: string) {
  return {
    id,
    role: "user",
    content: [{ type: "text", text }],
    metadata: authoredId === undefined ? undefined : { custom: { message_id: authoredId } },
  };
}

function assistant(id: string, ...parts: unknown[]) {
  return { id, role: "assistant", content: parts };
}

describe("steer provenance", () => {
  it("Should bind several same-turn steers to their own identities, not the preceding bubble", () => {
    const first = marker(STEERED, {
      steer_delivery: "injected",
      message_id: "m1",
      authored_text: "one",
    });
    const second = marker(STEERED, {
      steer_delivery: "interrupt_fallback",
      message_id: "m2",
      authored_text: "two",
    });
    const index = deriveSteerProvenance([
      user("m1", "one"),
      user("m2", "two"),
      assistant("a1", second, first),
    ]);
    expect(index.byMessageId.get("m1")).toMatchObject({ kind: "injected", authoredText: "one" });
    expect(index.byMessageId.get("m2")).toMatchObject({ kind: "interrupt_fallback" });
    // Both messages are loaded: the rows render nothing, the bubbles carry the meta.
    expect(index.renderFor(first.data)).toEqual({ kind: "bound" });
    expect(index.renderFor(second.data)).toEqual({ kind: "bound" });
    expect(index.receiptsForCluster(second.data, 2)).toEqual([]);
  });

  it("Should collapse pending then injected into one bubble owned by the real message", () => {
    const pending = marker(STEERED, {
      steer_delivery: "pending_injection",
      message_id: "m1",
      authored_text: "guide",
    });
    const injected = marker(STEERED, {
      steer_delivery: "injected",
      message_id: "m1",
      authored_text: "guide",
    });
    // Before confirmation: no user message yet — the pending receipt renders once.
    const before = deriveSteerProvenance([assistant("a1", pending)]);
    expect(before.renderFor(pending.data)).toMatchObject({
      kind: "receipt",
      provenance: { messageId: "m1", kind: "pending_injection", authoredText: "guide" },
    });
    // After: the real message exists and a newer injected marker updates the identity.
    const after = deriveSteerProvenance([
      assistant("a1", pending),
      user("m1", "guide"),
      assistant("a2", injected),
    ]);
    expect(after.byMessageId.get("m1")).toMatchObject({ kind: "injected" });
    expect(after.renderFor(pending.data)).toEqual({ kind: "bound" });
    expect(after.renderFor(injected.data)).toEqual({ kind: "bound" });
    // A fallback replacement message with the same identity binds the same way.
    const fallback = marker(STEERED, {
      steer_delivery: "interrupt_fallback",
      message_id: "m1",
      authored_text: "guide",
    });
    const replaced = deriveSteerProvenance([
      assistant("a1", pending),
      assistant("a2", fallback),
      user("m1", "guide"),
    ]);
    expect(replaced.byMessageId.get("m1")).toMatchObject({ kind: "interrupt_fallback" });
    expect(replaced.renderFor(pending.data)).toEqual({ kind: "bound" });
  });

  it("Should keep superseded authored text as one subdued receipt with the latest truth", () => {
    const pending = marker(STEERED, {
      steer_delivery: "pending_injection",
      message_id: "m3",
      authored_text: "old plan",
    });
    const superseded = marker(SUPERSEDED, {
      message_id: "m3",
      authored_text: "old plan",
      replacement_entry_id: "inq_9",
    });
    const again = marker(SUPERSEDED, {
      message_id: "m3",
      authored_text: "old plan",
      replacement_entry_id: "inq_10",
    });
    const index = deriveSteerProvenance([
      assistant("a1", pending, superseded),
      assistant("a2", again),
    ]);
    expect(index.byMessageId.get("m3")).toMatchObject({
      kind: "superseded",
      authoredText: "old plan",
    });
    // Only the latest marker renders the receipt; the earlier ones render nothing.
    expect(index.renderFor(pending.data)).toEqual({ kind: "superseded-marker" });
    expect(index.renderFor(superseded.data)).toEqual({ kind: "superseded-marker" });
    expect(index.renderFor(again.data)).toMatchObject({
      kind: "receipt",
      provenance: { kind: "superseded" },
    });
    // A clustered row (consecutive same-kind markers) renders the receipts it stands for.
    expect(index.receiptsForCluster(pending.data, 2)).toEqual([]);
    expect(index.receiptsForCluster(again.data, 1).map(receipt => receipt.messageId)).toEqual([
      "m3",
    ]);
  });

  it("Should bind a queued prompt to its message with only a daemon-recorded position", () => {
    const queued = marker(QUEUED, {
      queue_entry_id: "inq_7",
      queue_position: 2,
    });
    const accepted = marker(ACCEPTED, {
      queue_entry_id: "inq_7",
      message_id: "m7",
      authored_text: "Ship it",
    });
    const index = deriveSteerProvenance([
      assistant("a0", queued),
      user("m7", "Ship it"),
      assistant("a1", accepted),
    ]);
    expect(index.byMessageId.get("m7")).toMatchObject({ kind: "queued", queuePosition: "2" });
    expect(index.renderFor(accepted.data)).toEqual({ kind: "bound" });
    // No recorded position anywhere: the meta stays "From the queue" without a number.
    const bare = marker(ACCEPTED, {
      queue_entry_id: "inq_8",
      message_id: "m8",
    });
    const withoutPosition = deriveSteerProvenance([user("m8", "Later"), assistant("a2", bare)]);
    expect(withoutPosition.byMessageId.get("m8")).toMatchObject({
      kind: "queued",
      queuePosition: null,
    });
    // A queued prompt whose message is not loaded stays a neutral row, never a receipt bubble.
    expect(deriveSteerProvenance([assistant("a2", bare)]).renderFor(bare.data)).toEqual({
      kind: "legacy",
    });
  });

  it("Should keep legacy markers as neutral rows and attach them to nothing", () => {
    const legacy = marker(STEERED, { steer_delivery: "injected", queue_entry_id: "inq_1" });
    const index = deriveSteerProvenance([user("m1", "unrelated"), assistant("a1", legacy)]);
    expect(index.byMessageId.size).toBe(0);
    expect(index.renderFor(legacy.data)).toEqual({ kind: "legacy" });
    expect(index.renderFor({ type: "runtime_progress" })).toBeNull();
    // Delivered guidance whose message is not loaded stays the neutral line, never a bubble.
    const unloaded = marker(STEERED, {
      steer_delivery: "injected",
      message_id: "m9",
      authored_text: "gone",
    });
    expect(deriveSteerProvenance([assistant("a1", unloaded)]).renderFor(unloaded.data)).toEqual({
      kind: "legacy",
    });
    // A pending marker without authored text has nothing to show as a bubble.
    const textless = marker(STEERED, { steer_delivery: "pending_injection", message_id: "m8" });
    expect(deriveSteerProvenance([assistant("a1", textless)]).renderFor(textless.data)).toEqual({
      kind: "legacy",
    });
  });

  it("Should bind a uniquified rendered id through its authored identity, never by text", () => {
    // The daemon projects a duplicate MessageID with a suffix on the rendered id
    // while `metadata.message_id` keeps the authored one its markers name.
    const steered = marker(STEERED, {
      steer_delivery: "injected",
      message_id: "m1",
      authored_text: "again",
    });
    const accepted = marker(ACCEPTED, { message_id: "q1", queue_entry_id: "inq-1", mode: "queue" });
    const queued = marker(QUEUED, { queue_entry_id: "inq-1", queue_position: 3 });
    const twin = user("m1-2", "again", "m1");
    const lookalike = user("other", "again");
    const queuedTwin = user("q1-2", "index", "q1");
    const index = deriveSteerProvenance([
      user("m1", "first"),
      twin,
      lookalike,
      assistant("a1", steered),
      assistant("a2", queued),
      queuedTwin,
      assistant("a3", accepted),
    ]);
    expect(index.forMessage(twin)).toMatchObject({ kind: "injected", messageId: "m1" });
    expect(index.forMessage(lookalike)).toBeNull();
    expect(index.forMessage(queuedTwin)).toMatchObject({ kind: "queued", queuePosition: "3" });
    // Rows still resolve by the marker's own facts: the identity is loaded, so nothing renders there.
    expect(index.renderFor(steered.data)).toEqual({ kind: "bound" });
    expect(index.renderFor(accepted.data)).toEqual({ kind: "bound" });
    // Without a loaded message the receipt rule is unchanged for a pending steer.
    const pending = marker(STEERED, {
      steer_delivery: "pending_injection",
      message_id: "m9",
      authored_text: "later",
    });
    expect(deriveSteerProvenance([assistant("a4", pending)]).renderFor(pending.data)).toMatchObject(
      {
        kind: "receipt",
      }
    );
  });
});
