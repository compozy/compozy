import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ChatMessageBubble } from "../chat-message-bubble";

describe("ChatMessageBubble", () => {
  it("Should render a right-aligned bubble for role='user'", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="user" meta="YOU · 12:02">
        Find the event mapper.
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    const body = container.querySelector<HTMLElement>('[data-slot="chat-message-body"]');
    const meta = container.querySelector<HTMLElement>('[data-slot="chat-message-meta"]');
    expect(root?.getAttribute("data-role")).toBe("user");
    expect(root?.getAttribute("data-align")).toBe("right");
    expect(body?.textContent).toContain("Find the event mapper.");
    expect(meta?.textContent).toBe("YOU · 12:02");
  });

  it("Should render role='agent' left-aligned with no bubble wrapper", () => {
    const { container } = render(
      <ChatMessageBubble
        messageRole="agent"
        meta={
          <>
            <span data-testid="dot" />
            <span data-testid="name">claude</span>
          </>
        }
      >
        I can see two candidates.
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    const meta = container.querySelector<HTMLElement>('[data-slot="chat-message-meta"]');
    expect(root?.getAttribute("data-role")).toBe("agent");
    expect(root?.getAttribute("data-align")).toBe("left");
    expect(meta?.querySelector('[data-testid="dot"]')).not.toBeNull();
    expect(meta?.querySelector('[data-testid="name"]')?.textContent).toBe("claude");
  });

  it("Should render role='system' as a divider row with hairlines flanking the body", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="system">Session resumed</ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    const body = container.querySelector<HTMLElement>('[data-slot="chat-message-body"]');
    expect(root?.getAttribute("data-role")).toBe("system");
    const dividers = root?.querySelectorAll('span[aria-hidden="true"]') ?? [];
    expect(dividers.length).toBe(2);
    expect(body?.textContent).toBe("Session resumed");
  });

  it("Should place the meta slot above the body for role='user'", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="user" meta="YOU · 12:02">
        hello
      </ChatMessageBubble>
    );
    const inner = container.querySelector<HTMLElement>('[data-slot="chat-message-inner"]');
    const slots = inner ? Array.from(inner.children).map(el => el.getAttribute("data-slot")) : [];
    expect(slots[0]).toBe("chat-message-meta");
    expect(slots[1]).toBe("chat-message-body");
  });

  it("Should place the meta slot beside the agent name (inline row) for role='agent'", () => {
    const { container } = render(
      <ChatMessageBubble
        messageRole="agent"
        meta={
          <>
            <span data-testid="dot" />
            <span data-testid="name">claude</span>
          </>
        }
      >
        body
      </ChatMessageBubble>
    );
    const meta = container.querySelector<HTMLElement>('[data-slot="chat-message-meta"]');
    const children = Array.from(meta?.children ?? []);
    expect(children.length).toBeGreaterThanOrEqual(2);
    expect(children[0]?.getAttribute("data-testid")).toBe("dot");
    expect(children[1]?.getAttribute("data-testid")).toBe("name");
  });

  it.each(["tool", "diff"] as const)(
    "Should render role='%s' as a left-aligned pass-through container",
    role => {
      const { container } = render(
        <ChatMessageBubble messageRole={role}>
          <div data-testid="inner-card">payload</div>
        </ChatMessageBubble>
      );
      const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
      const body = container.querySelector<HTMLElement>('[data-slot="chat-message-body"]');
      expect(root?.getAttribute("data-role")).toBe(role);
      expect(root?.getAttribute("data-align")).toBe("left");
      expect(body?.querySelector('[data-testid="inner-card"]')?.textContent).toBe("payload");
    }
  );

  it("Should honour an explicit align override", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="user" align="left">
        override
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    expect(root?.getAttribute("data-align")).toBe("left");
  });

  it.each(["agent", "tool", "diff"] as const)("Should honour align='right' for role='%s'", role => {
    const { container } = render(
      <ChatMessageBubble messageRole={role} align="right" meta="META">
        payload
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    expect(root?.getAttribute("data-align")).toBe("right");
  });

  it("Should keep role='system' centered even when align is overridden", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="system" align="right">
        Session resumed
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    expect(root?.getAttribute("data-align")).toBe("right");
  });

  it("Should forward extra HTML props to the root", () => {
    const { container } = render(
      <ChatMessageBubble messageRole="agent" data-testid="m1">
        body
      </ChatMessageBubble>
    );
    const root = container.querySelector<HTMLElement>('[data-slot="chat-message"]');
    expect(root?.getAttribute("data-testid")).toBe("m1");
  });
});
