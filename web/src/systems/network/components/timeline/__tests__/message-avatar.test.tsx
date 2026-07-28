// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MessageAvatar } from "../message-avatar";

describe("MessageAvatar", () => {
  it("Should NOT inline the legacy `font-mono uppercase tracking-mono` tuple", () => {
    render(<MessageAvatar initialFrom="Codex" seed="codex" sizePx={36} />);
    const avatar = screen.getByTestId("network-message-avatar");
    expect(avatar.className).not.toContain("font-mono");
    expect(avatar.className).not.toContain("uppercase");
    expect(avatar.className).not.toContain("tracking-mono");
  });

  it("Should render the initial inside an <Eyebrow> primitive (`.eyebrow` utility)", () => {
    render(<MessageAvatar initialFrom="Codex" seed="codex" sizePx={36} />);
    const eyebrow = screen
      .getByTestId("network-message-avatar")
      .querySelector('[data-slot="eyebrow"]');
    expect(eyebrow).not.toBeNull();
    expect(eyebrow?.textContent).toBe("C");
    expect(eyebrow?.className).toContain("eyebrow");
  });

  it("Should announce `{Role} {Name}` when `role` is provided", () => {
    render(
      <MessageAvatar initialFrom="codex" name="Codex" ownerRole="agent" seed="codex" sizePx={32} />
    );
    const avatar = screen.getByTestId("network-message-avatar");
    expect(avatar).toHaveAttribute("role", "img");
    expect(avatar).toHaveAttribute("aria-label", "Agent Codex");
    expect(avatar).toHaveAttribute("data-owner-role", "agent");
    expect(avatar.hasAttribute("aria-hidden")).toBe(false);
  });

  it("Should default to aria-hidden when no role is provided", () => {
    render(<MessageAvatar initialFrom="Codex" seed="codex" sizePx={20} />);
    const avatar = screen.getByTestId("network-message-avatar");
    expect(avatar).toHaveAttribute("aria-hidden", "true");
    expect(avatar.hasAttribute("role")).toBe(false);
  });

  it("Should render a 36 / 32 / 20 px square via inline style", () => {
    const { rerender } = render(<MessageAvatar initialFrom="C" seed="c" sizePx={36} />);
    expect(screen.getByTestId("network-message-avatar").style.width).toBe("36px");
    rerender(<MessageAvatar initialFrom="C" seed="c" sizePx={32} />);
    expect(screen.getByTestId("network-message-avatar").style.width).toBe("32px");
    rerender(<MessageAvatar initialFrom="C" seed="c" sizePx={20} />);
    expect(screen.getByTestId("network-message-avatar").style.width).toBe("20px");
  });
});
