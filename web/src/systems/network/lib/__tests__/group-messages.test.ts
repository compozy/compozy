import { describe, expect, it } from "vitest";

import { buildTimelineEntries, isSystemKind, type TimelineEntry } from "../group-messages";
import type { NetworkConversationMessage } from "../../types";

function makeMessage(overrides: Partial<NetworkConversationMessage>): NetworkConversationMessage {
  return {
    body: { text: overrides.text ?? "Hello" },
    channel: "ops",
    direction: "sent",
    display_name: "Codex",
    kind: "say",
    local: true,
    message_id: overrides.message_id ?? "msg",
    peer_from: overrides.peer_from ?? "peer-codex",
    preview_text: overrides.text ?? "Hello",
    session_id: "sess-1",
    text: overrides.text ?? "Hello",
    timestamp: overrides.timestamp ?? "2026-04-17T14:32:00Z",
    ...overrides,
  } as NetworkConversationMessage;
}

function entryIds(entries: ReadonlyArray<TimelineEntry>): string[] {
  return entries.map(entry => entry.id);
}

describe("buildTimelineEntries", () => {
  it("Should mark the first message of a group as full row", () => {
    const result = buildTimelineEntries({
      messages: [makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" })],
    });
    expect(result).toHaveLength(2);
    expect(result[0]?.kind).toBe("date-pill");
    const message = result[1];
    expect(message?.kind).toBe("message");
    if (message?.kind === "message") {
      expect(message.variant).toBe("full");
      expect(message.startsGroup).toBe(true);
    }
  });

  it("Should collapse continuation messages within the 60s window", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({ message_id: "m2", timestamp: "2026-04-17T14:32:45Z" }),
      ],
    });

    const messages = result.filter(entry => entry.kind === "message");
    expect(messages).toHaveLength(2);
    expect(messages[0]?.kind === "message" && messages[0].variant).toBe("full");
    expect(messages[1]?.kind === "message" && messages[1].variant).toBe("collapsed");
  });

  it("Should break the group when the gap exceeds 60 seconds", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({ message_id: "m2", timestamp: "2026-04-17T14:33:30Z" }),
      ],
    });

    const messages = result.filter(entry => entry.kind === "message");
    expect(messages.every(entry => entry.kind === "message" && entry.variant === "full")).toBe(
      true
    );
  });

  it("Should break the group when the author changes", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({ message_id: "m1", peer_from: "peer-a", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({ message_id: "m2", peer_from: "peer-b", timestamp: "2026-04-17T14:32:30Z" }),
      ],
    });
    const messages = result.filter(entry => entry.kind === "message");
    expect(messages[1]?.kind === "message" && messages[1].variant).toBe("full");
  });

  it("Should break the group when the kind changes", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({
          kind: "receipt",
          message_id: "m2",
          timestamp: "2026-04-17T14:32:15Z",
        }),
      ],
    });
    const messages = result.filter(entry => entry.kind === "message");
    expect(messages[1]?.kind === "message" && messages[1].variant).toBe("system");
  });

  it("Should render system kinds as system rows even when same author", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({
          kind: "trace",
          message_id: "m1",
          timestamp: "2026-04-17T14:32:00Z",
        }),
      ],
    });
    const messages = result.filter(entry => entry.kind === "message");
    expect(messages[0]?.kind === "message" && messages[0].variant).toBe("system");
  });

  it("Should emit a date pill on day change", () => {
    const result = buildTimelineEntries({
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T23:50:00Z" }),
        makeMessage({ message_id: "m2", timestamp: "2026-04-18T00:30:00Z" }),
      ],
    });

    const dayPills = result.filter(entry => entry.kind === "date-pill");
    expect(dayPills).toHaveLength(2);
  });

  it("Should position the New divider at the first message after lastReadAt", () => {
    const result = buildTimelineEntries({
      lastReadAt: "2026-04-17T14:32:30Z",
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({ message_id: "m2", timestamp: "2026-04-17T14:33:00Z" }),
        makeMessage({ message_id: "m3", timestamp: "2026-04-17T14:34:00Z" }),
      ],
    });

    const ids = entryIds(result);
    const dividerIndex = ids.indexOf("new:m2");
    const messageIndex = ids.indexOf("msg:m2");
    expect(dividerIndex).toBeGreaterThan(-1);
    expect(messageIndex).toBeGreaterThan(dividerIndex);
  });

  it("Should not emit a divider when lastReadAt is at or after the last message", () => {
    const result = buildTimelineEntries({
      lastReadAt: "2026-04-17T14:34:00Z",
      messages: [
        makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
        makeMessage({ message_id: "m2", timestamp: "2026-04-17T14:33:00Z" }),
      ],
    });

    expect(result.some(entry => entry.kind === "new-divider")).toBe(false);
  });
});

describe("isSystemKind", () => {
  it("Should classify protocol system kinds", () => {
    expect(isSystemKind("greet")).toBe(true);
    expect(isSystemKind("trace")).toBe(true);
    expect(isSystemKind("receipt")).toBe(true);
    expect(isSystemKind("capability")).toBe(true);
    expect(isSystemKind("whois")).toBe(true);
  });

  it("Should not classify say as system", () => {
    expect(isSystemKind("say")).toBe(false);
  });
});
