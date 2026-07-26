import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";

import { UIProvider } from "@agh/ui";

import { DesignSystemShowcase } from "@/components/design-system-showcase";
import { SECTIONS } from "@/components/design-system-showcase-sections";
import { TOKEN_GROUPS } from "@/components/design-system-showcase-tokens";

function renderShowcase() {
  return render(
    <UIProvider reducedMotion="always">
      <DesignSystemShowcase />
    </UIProvider>
  );
}

describe("DesignSystemShowcase", () => {
  describe("rendering", () => {
    it("renders the page header, filter toolbar, and search input", () => {
      renderShowcase();
      expect(screen.getByTestId("design-system-showcase")).toBeInTheDocument();
      expect(screen.getByText("AGH design system")).toBeInTheDocument();
      expect(screen.getByRole("toolbar", { name: /showcase filters/i })).toBeInTheDocument();
      expect(screen.getByPlaceholderText(/search primitives/i)).toBeInTheDocument();
    });

    it("links the top-level DESIGN.md shortcut to the spec", () => {
      renderShowcase();
      const link = screen.getByTestId("showcase-open-design-md");
      expect(link.getAttribute("href")).toBe("https://github.com/compozy/agh/blob/main/DESIGN.md");
    });

    it("renders a dedicated section for every primitive grouping", () => {
      renderShowcase();
      for (const section of SECTIONS) {
        expect(screen.getByTestId(`section-${section.id}`)).toBeInTheDocument();
      }
    });

    it("section headers link to the DESIGN.md anchor for that group", () => {
      renderShowcase();
      for (const section of SECTIONS) {
        const link = screen.getByTestId(`section-link-${section.id}`);
        expect(link.getAttribute("href")).toBe(
          `https://github.com/compozy/agh/blob/main/DESIGN.md${section.anchor}`
        );
        expect(link.getAttribute("data-section-id")).toBe(section.id);
        expect(link.getAttribute("data-section-anchor")).toBe(section.anchor);
      }
    });

    it("renders button and pill primitives", () => {
      renderShowcase();
      const buttons = screen.getByTestId("section-buttons");
      expect(within(buttons).getByRole("button", { name: "Primary" })).toBeInTheDocument();
      expect(within(buttons).getByRole("button", { name: "Secondary" })).toBeInTheDocument();
      expect(within(buttons).getByRole("button", { name: "Destructive" })).toBeInTheDocument();
      expect(within(buttons).getByRole("button", { name: "Outline" })).toBeInTheDocument();
      expect(within(buttons).getByText("Action")).toBeInTheDocument();
      expect(within(buttons).getByText("Stable")).toBeInTheDocument();
    });

    it("renders input, select, toggle, switch, and search primitives", () => {
      renderShowcase();
      const inputs = screen.getByTestId("section-inputs");
      expect(within(inputs).getByLabelText("Display name")).toBeInTheDocument();
      expect(within(inputs).getByLabelText("Notes")).toBeInTheDocument();
      expect(within(inputs).getByLabelText("Environment")).toBeInTheDocument();
      expect(within(inputs).getByPlaceholderText(/filter sessions/i)).toBeInTheDocument();
      expect(within(inputs).getByRole("switch")).toBeInTheDocument();
      expect(within(inputs).getByRole("button", { name: "Tasks" })).toBeInTheDocument();
      expect(within(inputs).getByRole("button", { name: "Sessions" })).toBeInTheDocument();
    });

    it("renders status primitives, metric cards, mono badges, and kind chips", () => {
      renderShowcase();
      const status = screen.getByTestId("section-status");
      expect(within(status).getByText("Active sessions")).toBeInTheDocument();
      expect(within(status).getByText("RUNNING")).toBeInTheDocument();
      const kindChips = status.querySelectorAll('[data-slot="kind-chip"]');
      expect(kindChips.length).toBeGreaterThanOrEqual(7);
      expect(status.querySelectorAll('[data-slot="connection-indicator"]').length).toBe(3);
    });

    it("renders Alert + Empty feedback primitives", () => {
      renderShowcase();
      const feedback = screen.getByTestId("section-feedback");
      expect(within(feedback).getAllByRole("alert").length).toBe(2);
      expect(feedback.querySelector('[data-slot="empty"]')).toBeInTheDocument();
    });

    it("renders Dialog, Sheet, Popover, Tooltip, Dropdown, Tabs, Accordion, Collapsible triggers", () => {
      renderShowcase();
      expect(screen.getByTestId("showcase-dialog-trigger")).toBeInTheDocument();
      expect(screen.getByTestId("showcase-sheet-trigger")).toBeInTheDocument();
      expect(screen.getByTestId("showcase-popover-trigger")).toBeInTheDocument();
      expect(screen.getByTestId("showcase-tooltip-trigger")).toBeInTheDocument();
      expect(screen.getByTestId("showcase-menu-trigger")).toBeInTheDocument();
      expect(screen.getByTestId("showcase-collapsible-trigger")).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /overview/i })).toBeInTheDocument();
    });

    it("renders CodeBlock + ChatMessageBubble + ToolCallRow in the session shells block", () => {
      renderShowcase();
      const block = screen.getByTestId("section-code-chat");
      expect(block.querySelector('[data-slot="code-block"]')).toBeInTheDocument();
      expect(block.querySelectorAll('[data-slot="chat-message"]').length).toBeGreaterThanOrEqual(4);
      expect(block.querySelector('[data-slot="tool-call-row"]')).toBeInTheDocument();
    });

    it("renders Sidebar + SplitPane layout primitives", () => {
      renderShowcase();
      const layout = screen.getByTestId("section-layout");
      expect(layout.querySelector('[data-slot="sidebar"]')).toBeInTheDocument();
      expect(layout.querySelector('[data-slot="split-pane"]')).toBeInTheDocument();
    });
  });

  describe("token swatch wall", () => {
    it("renders one group per token category defined in TOKEN_GROUPS", () => {
      renderShowcase();
      for (const group of TOKEN_GROUPS) {
        expect(screen.getByTestId(`token-group-${group.id}`)).toBeInTheDocument();
      }
    });

    it("renders every showcased swatch with its token and value", () => {
      renderShowcase();
      for (const group of TOKEN_GROUPS) {
        const groupElement = screen.getByTestId(`token-group-${group.id}`);
        for (const swatch of group.swatches) {
          const card = within(groupElement).getByTestId(`token-${swatch.token}`);
          expect(card).toHaveAttribute("data-token", swatch.token);
          expect(card).toHaveAttribute("data-kind", swatch.kind);
          expect(within(card).getByText(swatch.token)).toBeInTheDocument();
          expect(within(card).getAllByText(swatch.value).length).toBeGreaterThan(0);
        }
      }
    });

    it("renders the expected range of color, radius, and motion swatches", () => {
      renderShowcase();
      const kinds = new Set(
        TOKEN_GROUPS.flatMap(group => group.swatches.map(swatch => swatch.kind))
      );
      expect(kinds.has("color")).toBe(true);
      expect(kinds.has("radius")).toBe(true);
      expect(kinds.has("duration")).toBe(true);
      expect(kinds.has("easing")).toBe(true);
      expect(kinds.has("tracking")).toBe(true);
    });
  });
});
