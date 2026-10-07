import { describe, expect, it } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";

import { UIProvider } from "@compozy/ui";

import { DesignSystemShowcase } from "@/components/design-system-showcase";
import { SECTIONS } from "@/components/design-system-showcase-sections";
import { TOKEN_GROUPS } from "@/components/design-system-showcase-tokens";

async function renderShowcase() {
  const view = render(
    <UIProvider reducedMotion="never" skipAnimations>
      <DesignSystemShowcase />
    </UIProvider>
  );

  await waitFor(() => {
    expect(
      screen.getByTestId("section-code-chat").querySelector('[data-slot="code-block"]')
    ).not.toHaveAttribute("data-highlight-state", "loading");
  });

  return view;
}

describe("DesignSystemShowcase", () => {
  it("renders the static showcase sections, links, primitives, and token swatches", async () => {
    await renderShowcase();

    expect(screen.getByTestId("design-system-showcase")).toBeInTheDocument();
    expect(screen.getByText("CompozyOS design system")).toBeInTheDocument();
    expect(screen.getByRole("toolbar", { name: /showcase filters/i })).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/search primitives/i)).toBeInTheDocument();

    const link = screen.getByTestId("showcase-open-design-md");
    expect(link.getAttribute("href")).toBe(
      "https://github.com/compozy/compozy/blob/main/DESIGN.md"
    );

    for (const section of SECTIONS) {
      expect(screen.getByTestId(`section-${section.id}`)).toBeInTheDocument();
    }

    for (const section of SECTIONS) {
      const link = screen.getByTestId(`section-link-${section.id}`);
      expect(link.getAttribute("href")).toBe(
        `https://github.com/compozy/compozy/blob/main/DESIGN.md${section.anchor}`
      );
      expect(link.getAttribute("data-section-id")).toBe(section.id);
      expect(link.getAttribute("data-section-anchor")).toBe(section.anchor);
    }

    const buttons = screen.getByTestId("section-buttons");
    expect(within(buttons).getByRole("button", { name: "Primary" })).toBeInTheDocument();
    expect(within(buttons).getByRole("button", { name: "Secondary" })).toBeInTheDocument();
    expect(within(buttons).getByRole("button", { name: "Destructive" })).toBeInTheDocument();
    expect(within(buttons).getByRole("button", { name: "Outline" })).toBeInTheDocument();
    expect(within(buttons).getByText("Needs you")).toBeInTheDocument();
    expect(within(buttons).getByText("Stable")).toBeInTheDocument();

    const inputs = screen.getByTestId("section-inputs");
    expect(within(inputs).getByLabelText("Display name")).toBeInTheDocument();
    expect(within(inputs).getByLabelText("Notes")).toBeInTheDocument();
    expect(within(inputs).getByLabelText("Environment")).toBeInTheDocument();
    expect(within(inputs).getByPlaceholderText(/filter sessions/i)).toBeInTheDocument();
    expect(within(inputs).getByRole("switch")).toBeInTheDocument();
    expect(within(inputs).getByRole("button", { name: "Tasks" })).toBeInTheDocument();
    expect(within(inputs).getByRole("button", { name: "Sessions" })).toBeInTheDocument();

    const status = screen.getByTestId("section-status");
    expect(within(status).getByText("Active sessions")).toBeInTheDocument();
    expect(within(status).getByText("RUNNING")).toBeInTheDocument();
    expect(status.querySelectorAll('[data-slot="connection-indicator"]').length).toBe(3);
    const glyphs = within(status).getByTestId("showcase-state-glyphs");
    expect(
      Array.from(glyphs.querySelectorAll('[data-slot="state-glyph"]'), glyph =>
        glyph.getAttribute("data-state")
      )
    ).toEqual(["running", "queued", "done", "attention", "failed", "stopped", "idle"]);

    const feedback = screen.getByTestId("section-feedback");
    expect(within(feedback).getAllByRole("alert").length).toBe(2);
    expect(feedback.querySelector('[data-slot="empty"]')).toBeInTheDocument();

    expect(screen.getByTestId("showcase-dialog-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("showcase-sheet-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("showcase-popover-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("showcase-tooltip-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("showcase-menu-trigger")).toBeInTheDocument();
    expect(screen.getByTestId("showcase-collapsible-trigger")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /overview/i })).toBeInTheDocument();

    const block = screen.getByTestId("section-code-chat");
    expect(block.querySelector('[data-slot="code-block"]')).toBeInTheDocument();
    expect(block.querySelectorAll('[data-slot="chat-message"]').length).toBeGreaterThanOrEqual(4);
    expect(block.querySelector('[data-slot="tool-call-row"]')).toBeInTheDocument();

    const layout = screen.getByTestId("section-layout");
    expect(layout.querySelector('[data-slot="sidebar"]')).toBeInTheDocument();
    expect(layout.querySelector('[data-slot="split-pane"]')).toBeInTheDocument();

    for (const group of TOKEN_GROUPS) {
      expect(screen.getByTestId(`token-group-${group.id}`)).toBeInTheDocument();
    }

    for (const group of TOKEN_GROUPS) {
      const groupElement = screen.getByTestId(`token-group-${group.id}`);
      for (const swatch of group.swatches) {
        const card = within(groupElement).getByTestId(`token-${swatch.token}`);
        expect(card).toHaveAttribute("data-token", swatch.token);
        expect(card).toHaveAttribute("data-kind", swatch.kind);
        expect(within(card).getByText(swatch.token)).toBeInTheDocument();
        expect(card.querySelector('[data-slot="token-value"]')).toBeInTheDocument();
      }
    }

    const kinds = new Set(TOKEN_GROUPS.flatMap(group => group.swatches.map(swatch => swatch.kind)));
    expect(kinds.has("color")).toBe(true);
    expect(kinds.has("radius")).toBe(true);
    expect(kinds.has("duration")).toBe(true);
    expect(kinds.has("easing")).toBe(true);
    expect(kinds.has("tracking")).toBe(true);
  });

  it("shows the active theme's value and follows a theme switch", async () => {
    const root = document.documentElement;
    root.style.setProperty("--color-canvas", "#1a1a1a");
    try {
      await renderShowcase();
      const value = () =>
        screen.getByTestId("token---color-canvas").querySelector('[data-slot="token-value"]')
          ?.textContent;
      expect(value()).toBe("#1a1a1a");

      // A theme switch re-declares the token on <html>; the card re-reads it.
      root.style.setProperty("--color-canvas", "#ffffff");
      root.setAttribute("data-theme", "light");
      await waitFor(() => expect(value()).toBe("#ffffff"));
    } finally {
      cleanup();
      root.style.removeProperty("--color-canvas");
      root.removeAttribute("data-theme");
    }
  });
});
