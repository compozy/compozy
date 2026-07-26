// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MessageTimeline } from "../timeline";
import type { NetworkConversationMessage } from "../../../types";

function makeMessage(overrides: Partial<NetworkConversationMessage>): NetworkConversationMessage {
  return {
    body: { text: overrides.text ?? "Hello" },
    channel: "ops",
    direction: "sent",
    display_name: overrides.display_name ?? "Codex",
    kind: overrides.kind ?? "say",
    local: true,
    message_id: overrides.message_id ?? "msg-1",
    peer_from: overrides.peer_from ?? "peer-codex",
    preview_text: overrides.text ?? "Hello",
    session_id: overrides.session_id ?? "sess-1",
    text: overrides.text ?? "Hello",
    timestamp: overrides.timestamp ?? "2026-04-17T14:32:00Z",
    ...overrides,
  } as NetworkConversationMessage;
}

describe("MessageTimeline", () => {
  it("Should render a full message row with avatar (announcing role + name) and body", () => {
    render(
      <MessageTimeline
        messages={[
          makeMessage({
            message_id: "m1",
            text: "Sample body content",
          }),
        ]}
      />
    );

    expect(screen.getByTestId("network-message-row-full")).toBeInTheDocument();
    const avatar = screen.getByTestId("network-message-avatar");
    expect(avatar).toBeInTheDocument();
    expect(avatar).toHaveAttribute("role", "img");
    expect(avatar.getAttribute("aria-label")).toMatch(/^Agent /);
    expect(avatar.getAttribute("data-owner-role")).toBe("agent");
    expect(screen.getByText("Sample body content")).toBeInTheDocument();
  });

  it("Should NOT render the role pill on message rows", () => {
    render(<MessageTimeline messages={[makeMessage({ message_id: "m1", text: "Sample" })]} />);

    expect(screen.queryByTestId("network-message-role-chip")).toBeNull();
    expect(screen.queryByText("agent")).toBeNull();
  });

  it("Should render a collapsed continuation row", () => {
    render(
      <MessageTimeline
        messages={[
          makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
          makeMessage({
            message_id: "m2",
            text: "Continuation",
            timestamp: "2026-04-17T14:32:30Z",
          }),
        ]}
      />
    );

    const collapsed = screen.getByTestId("network-message-row-collapsed");
    expect(collapsed).toBeInTheDocument();
    // Role pill is never rendered on any row.
    expect(screen.queryByTestId("network-message-role-chip")).toBeNull();
  });

  it("Should never render a kind chip for kind say", () => {
    render(
      <MessageTimeline messages={[makeMessage({ message_id: "m1", text: "default kind" })]} />
    );
    expect(screen.queryByTestId("network-message-kind-chip")).toBeNull();
  });

  it("Should render system kinds as a single-line system row", () => {
    render(
      <MessageTimeline
        messages={[makeMessage({ kind: "trace", message_id: "m1", text: "tracing" })]}
      />
    );

    expect(screen.getByTestId("network-message-row-system")).toBeInTheDocument();
  });

  it("Should render a date pill across midnight boundaries", () => {
    render(
      <MessageTimeline
        messages={[
          makeMessage({ message_id: "m1", timestamp: "2026-04-17T23:50:00Z" }),
          makeMessage({ message_id: "m2", timestamp: "2026-04-18T00:10:00Z" }),
        ]}
        now={new Date("2026-04-18T00:30:00Z")}
      />
    );

    const pills = screen.getAllByTestId("network-timeline-date-pill");
    expect(pills.length).toBeGreaterThanOrEqual(2);
  });

  it("Should render the New divider at the boundary of the last-read timestamp", () => {
    render(
      <MessageTimeline
        lastReadAt="2026-04-17T14:32:30Z"
        messages={[
          makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
          makeMessage({ message_id: "m2", timestamp: "2026-04-17T14:33:00Z" }),
        ]}
      />
    );

    expect(screen.getByTestId("network-timeline-new-divider")).toBeInTheDocument();
  });

  it("Should reveal the timestamp on collapsed gutter hover", async () => {
    const user = userEvent.setup();
    render(
      <MessageTimeline
        messages={[
          makeMessage({ message_id: "m1", timestamp: "2026-04-17T14:32:00Z" }),
          makeMessage({
            message_id: "m2",
            text: "next",
            timestamp: "2026-04-17T14:32:30Z",
          }),
        ]}
      />
    );

    const collapsedTimestamp = screen.getByTestId("network-message-collapsed-timestamp");
    expect(collapsedTimestamp).toBeInTheDocument();
    await user.hover(screen.getByTestId("network-message-row-collapsed"));
    // jsdom does not apply group-hover variants, so we assert the title carries the ISO.
    expect(collapsedTimestamp.getAttribute("title")).toMatch(/\d{4}-\d{2}-\d{2}T/);
  });

  it("Should NOT render the hover-toolbar reactions button", () => {
    render(<MessageTimeline messages={[makeMessage({ message_id: "m1" })]} />);

    expect(screen.queryByRole("button", { name: /add reaction/i })).toBeNull();
    expect(screen.queryByTestId("network-message-toolbar-react-m1")).toBeNull();
  });

  it("Should not render a message toolbar when no toolbar handlers are wired", () => {
    render(<MessageTimeline messages={[makeMessage({ message_id: "m1" })]} />);

    expect(screen.queryByTestId("network-message-toolbar-m1")).toBeNull();
    expect(screen.queryByRole("button", { name: /copy link/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /copy message text/i })).toBeNull();
  });

  it("Should expose only the wired Copy actions and invoke them on click", async () => {
    const user = userEvent.setup();
    const onCopyLink = vi.fn();
    const onCopyText = vi.fn();
    render(
      <MessageTimeline
        messages={[makeMessage({ message_id: "m1", text: "Body" })]}
        toolbarHandlers={() => ({ onCopyLink, onCopyText })}
      />
    );

    // The removed, never-implemented actions must be gone.
    expect(screen.queryByRole("button", { name: /reply in thread/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /pin to capability/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /fork thread/i })).toBeNull();

    const copyLink = screen.getByTestId("network-message-toolbar-copy-link-m1");
    const copyText = screen.getByTestId("network-message-toolbar-copy-text-m1");
    await user.click(copyLink);
    await user.click(copyText);

    expect(onCopyLink).toHaveBeenCalledTimes(1);
    expect(onCopyText).toHaveBeenCalledTimes(1);
  });

  it("Should omit the Copy text action when only a link handler is wired", () => {
    render(
      <MessageTimeline
        messages={[makeMessage({ message_id: "m1", text: "Body" })]}
        toolbarHandlers={() => ({ onCopyLink: vi.fn() })}
      />
    );

    expect(screen.getByTestId("network-message-toolbar-copy-link-m1")).toBeInTheDocument();
    expect(screen.queryByTestId("network-message-toolbar-copy-text-m1")).toBeNull();
  });

  it("Should load older messages and preserve the reader's scroll anchor", async () => {
    const user = userEvent.setup();
    let resolveOlder: (() => void) | undefined;
    const onLoadOlder = vi.fn(
      () =>
        new Promise<void>(resolve => {
          resolveOlder = resolve;
        })
    );
    const newer = makeMessage({ message_id: "newer", timestamp: "2026-04-17T14:33:00Z" });
    const older = makeMessage({ message_id: "older", timestamp: "2026-04-17T14:32:00Z" });
    const { rerender } = render(
      <MessageTimeline hasOlder messages={[newer]} onLoadOlder={onLoadOlder} />
    );
    const timeline = screen.getByTestId("network-timeline");
    let scrollHeight = 400;
    Object.defineProperty(timeline, "scrollHeight", {
      configurable: true,
      get: () => scrollHeight,
    });
    timeline.scrollTop = 120;

    await user.click(screen.getByRole("button", { name: "Load older messages" }));
    expect(onLoadOlder).toHaveBeenCalledTimes(1);

    scrollHeight = 640;
    rerender(
      <MessageTimeline hasOlder={false} messages={[older, newer]} onLoadOlder={onLoadOlder} />
    );
    resolveOlder?.();

    expect(timeline.scrollTop).toBe(360);
  });

  it("Should exclude a concurrent live-tail append from the older-page anchor adjustment", async () => {
    const user = userEvent.setup();
    const onLoadOlder = vi.fn().mockResolvedValue(undefined);
    const newer = makeMessage({ message_id: "newer", timestamp: "2026-04-17T14:33:00Z" });
    const older = makeMessage({ message_id: "older", timestamp: "2026-04-17T14:32:00Z" });
    const live = makeMessage({ message_id: "live", timestamp: "2026-04-17T14:34:00Z" });
    const { rerender } = render(
      <MessageTimeline hasOlder messages={[newer]} onLoadOlder={onLoadOlder} />
    );
    const timeline = screen.getByTestId("network-timeline");
    let scrollHeight = 400;
    let anchorTop = 120;
    Object.defineProperty(timeline, "scrollHeight", {
      configurable: true,
      get: () => scrollHeight,
    });
    const rectSpy = vi
      .spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function getTimelineRect(this: HTMLElement) {
        if (this.dataset.messageId === "newer") {
          return { top: anchorTop, bottom: anchorTop + 20 } as DOMRect;
        }
        return { top: 0, bottom: 400 } as DOMRect;
      });
    const anchor = timeline.querySelector<HTMLElement>('[data-message-id="newer"]');
    expect(anchor).not.toBeNull();
    timeline.scrollTop = 120;

    await user.click(screen.getByRole("button", { name: "Load older messages" }));
    scrollHeight = 680;
    anchorTop = 320;
    rerender(
      <MessageTimeline hasOlder={false} messages={[older, newer, live]} onLoadOlder={onLoadOlder} />
    );

    expect(timeline.scrollTop).toBe(320);
    rectSpy.mockRestore();
  });

  it("Should not declare any box-shadow on the timeline subtree", () => {
    render(
      <MessageTimeline
        messages={[
          makeMessage({ message_id: "m1" }),
          makeMessage({ message_id: "m2", text: "next", timestamp: "2026-04-17T14:32:20Z" }),
          makeMessage({ kind: "trace", message_id: "m3", text: "trace" }),
        ]}
      />
    );

    const root = screen.getByTestId("network-timeline");
    const allElements = root.querySelectorAll("*");
    for (const element of [root, ...allElements]) {
      expect(element.getAttribute("style") ?? "").not.toContain("box-shadow");
    }
  });
});
