import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ToolCallRow, type ToolCallStatus } from "../tool-call-row";

// Calm-transcript status budget: only the failure × carries a signal hue —
// the running spinner and the absorbed × stay grey, and every resting state
// (success, empty, pending, stopped) carries no glyph at all.
const GLYPH_STATUSES: Array<{ status: ToolCallStatus; label: string; tone: string }> = [
  { status: "running", label: "Running", tone: "text-subtle" },
  { status: "failed", label: "Error", tone: "text-danger" },
  { status: "absorbed", label: "Failed", tone: "text-subtle" },
];
const RESTING_STATUSES: ToolCallStatus[] = ["pending", "stopped", "success", "empty"];

function statusGlyph(container: HTMLElement): HTMLElement | null {
  return container.querySelector<HTMLElement>('[data-slot="tool-call-row-status"]');
}

function heading(container: HTMLElement): HTMLElement | null {
  return container.querySelector<HTMLElement>('[data-slot="tool-call-row-tool"]');
}

describe("ToolCallRow", () => {
  it("Should render icon, heading, mono preview, and status glyph in one compact row", () => {
    const { container } = render(
      <ToolCallRow
        toolName="Read"
        preview="packages/runtime/src/session/stream.ts"
        status="failed"
      />
    );

    const row = container.querySelector('[data-slot="tool-call-row"]');
    expect(row).not.toBeNull();
    expect(container.querySelector('[data-slot="tool-call-row-icon-well"]')?.className).toContain(
      "size-5"
    );
    expect(
      container.querySelector('[data-slot="tool-call-row-icon"]')?.getAttribute("class")
    ).toContain("size-3.5");
    expect(heading(container)?.textContent).toBe("Read");
    const preview = container.querySelector('[data-slot="tool-call-row-preview"]');
    expect(preview?.textContent).toBe("packages/runtime/src/session/stream.ts");
    expect(preview?.className).toContain("font-mono");
    expect(preview?.className).toContain("truncate");
    expect(statusGlyph(container)?.getAttribute("class")).toContain("size-3");
  });

  it("Should render each running/resolved status glyph with its signal tone and label", () => {
    for (const { status, label, tone } of GLYPH_STATUSES) {
      const { container, unmount } = render(<ToolCallRow toolName="Bash" status={status} />);
      const row = container.querySelector<HTMLElement>('[data-slot="tool-call-row"]');
      const indicator = statusGlyph(container);
      expect(row?.getAttribute("data-status")).toBe(status);
      expect(indicator?.getAttribute("data-status")).toBe(status);
      expect(indicator?.getAttribute("aria-label")).toBe(label);
      expect(indicator?.getAttribute("class")).toContain(tone);
      unmount();
    }
  });

  it("Should render no status glyph for a resting state — completion is not an event", () => {
    for (const status of RESTING_STATUSES) {
      const { container, unmount } = render(<ToolCallRow toolName="Bash" status={status} />);
      expect(
        container.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")
      ).toBe(status);
      expect(statusGlyph(container)).toBeNull();
      unmount();
    }
  });

  it("Should render the object as a chip and drop the icon well on a rail", () => {
    const { container } = render(
      <ToolCallRow
        toolName="Read"
        preview="a.ts"
        previewVariant="chip"
        icon={null}
        status="success"
      />
    );
    expect(container.querySelector('[data-slot="tool-call-row-icon-well"]')).toBeNull();
    expect(container.querySelector('[data-slot="tool-call-row-preview"]')).toHaveAttribute(
      "data-variant",
      "chip"
    );
  });

  it("Should render no glyph for a stopped call — its word carries the state", () => {
    const { container } = render(<ToolCallRow toolName="Bash" status="stopped" />);
    expect(
      container.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")
    ).toBe("stopped");
    expect(statusGlyph(container)).toBeNull();
  });

  it("Should render the per-file diff stat slot between text and trailing glyphs", () => {
    const { container } = render(
      <ToolCallRow
        toolName="Edited"
        preview="src/session.tsx"
        status="absorbed"
        statLabel="28 additions, 104 deletions"
        stat={
          <>
            <b>+28</b>
            <i>−104</i>
          </>
        }
      >
        <ToolCallRow.Output source="updated" format="code" />
      </ToolCallRow>
    );
    const preview = container.querySelector('[data-slot="tool-call-row-preview"]');
    const stat = container.querySelector('[data-slot="tool-call-row-stat"]');
    const status = statusGlyph(container);
    expect(preview).not.toBeNull();
    expect(stat).not.toBeNull();
    expect(status).not.toBeNull();
    expect(stat?.textContent).toBe("+28−104");
    expect(preview?.compareDocumentPosition(stat as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(stat?.compareDocumentPosition(status as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.getByRole("button")).toHaveAccessibleName(
      "Edited 28 additions, 104 deletions Toggle tool call (absorbed)"
    );
  });

  it("Should set role/tabIndex and the expand chevron only when an expandable body exists", () => {
    const { container, rerender } = render(<ToolCallRow toolName="Read" status="success" />);
    const staticRow = container.querySelector('[data-slot="tool-call-row-static"]');
    expect(staticRow?.getAttribute("role")).toBeNull();
    expect(staticRow?.getAttribute("tabindex")).toBeNull();
    expect(container.querySelector('[data-slot="tool-call-row-chevron"]')).toBeNull();

    rerender(
      <ToolCallRow toolName="Read" status="success">
        <ToolCallRow.Output source="ok" format="code" />
      </ToolCallRow>
    );
    const trigger = screen.getByRole("button");
    expect(trigger.tabIndex).toBe(0);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(container.querySelector('[data-slot="tool-call-row-chevron"]')).not.toBeNull();
  });

  it("Should toggle the inline body by click, Enter, and Space", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <ToolCallRow toolName="Read" preview="compozy.config.toml" status="success">
        <ToolCallRow.Input
          source='{"file_path":"compozy.config.toml"}'
          format="code"
          language="json"
        />
        <ToolCallRow.Output source="[runtime]\nmode = local" format="code" language="toml" />
      </ToolCallRow>
    );
    const trigger = screen.getByRole("button");

    expect(container.querySelector('[data-slot="tool-call-row-body"]')).toBeNull();
    await user.click(trigger);
    expect(container.querySelector('[data-slot="tool-call-row-body"]')).not.toBeNull();
    await user.keyboard("{Enter}");
    expect(container.querySelector('[data-slot="tool-call-row-body"]')).toBeNull();
    await user.keyboard(" ");
    expect(container.querySelector('[data-slot="tool-call-row-body"]')).not.toBeNull();
  });

  it("Should derive the trigger name from a rendered non-string tool name", () => {
    render(
      <ToolCallRow toolName={<span>Read workspace file</span>} status="success">
        <ToolCallRow.Output source="ok" format="code" />
      </ToolCallRow>
    );

    expect(
      screen.getByRole("button", { name: "Read workspace file Toggle tool call (success)" })
    ).toBeInTheDocument();
  });
});
