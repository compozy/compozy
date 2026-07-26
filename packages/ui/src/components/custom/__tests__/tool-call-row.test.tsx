import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FileEditIcon } from "lucide-react";
import { describe, expect, it } from "vitest";

import { ToolCallRow, type ToolCallStatus } from "../tool-call-row";

const GLYPH_STATUSES: Array<{ status: ToolCallStatus; label: string; tone: string }> = [
  { status: "running", label: "Running", tone: "text-muted" },
  { status: "failed", label: "Error", tone: "text-danger" },
  { status: "success", label: "Done", tone: "text-success" },
  { status: "empty", label: "Empty", tone: "text-subtle" },
];

function classesOf(root: Element): string[] {
  const classes: string[] = [];
  for (const node of root.querySelectorAll<HTMLElement>("*")) {
    if (node.className && typeof node.className === "string") classes.push(node.className);
  }
  if (root instanceof HTMLElement && typeof root.className === "string")
    classes.push(root.className);
  return classes;
}

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
        status="success"
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

  it("Should render no status glyph for pending (the muted preparing-input state)", () => {
    const { container } = render(<ToolCallRow toolName="Bash" status="pending" />);
    expect(
      container.querySelector('[data-slot="tool-call-row"]')?.getAttribute("data-status")
    ).toBe("pending");
    expect(statusGlyph(container)).toBeNull();
  });

  it("Should tone the heading danger only for a true runtime error, never for error-shaped output", () => {
    const { container: runtime } = render(
      <ToolCallRow toolName="Read" status="failed" runtimeError errorMessage="ENOENT" />
    );
    expect(heading(runtime)?.className).toContain("text-danger");

    const { container: shaped } = render(
      <ToolCallRow toolName="Read" status="failed" errorMessage="exit status 1" />
    );
    expect(heading(shaped)?.className).not.toContain("text-danger");
    expect(heading(shaped)?.className).toContain("text-muted");
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
      <ToolCallRow toolName="Read" preview="agh.config.toml" status="success">
        <ToolCallRow.Input source='{"file_path":"agh.config.toml"}' format="code" language="json" />
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

  it("Should hover with the neutral glaze and never render an accent class in the row DOM", () => {
    const { container } = render(
      <ToolCallRow toolName="Edit" status="success" icon={FileEditIcon}>
        <ToolCallRow.Output>
          <pre>selectable output</pre>
        </ToolCallRow.Output>
      </ToolCallRow>
    );
    const trigger = screen.getByRole("button");
    expect(
      container.querySelector('[data-slot="tool-call-row-trigger"]')?.getAttribute("class")
    ).toContain("hover:bg-hover");

    fireEvent.click(trigger);
    const row = container.querySelector('[data-slot="tool-call-row"]');
    expect(row).not.toBeNull();
    for (const className of classesOf(row!)) {
      expect(className).not.toMatch(/\baccent\b/);
    }
  });
});
