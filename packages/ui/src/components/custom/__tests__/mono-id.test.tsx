import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MonoId } from "../mono-id";

describe("MonoId", () => {
  it("Should render lowercase mono identifier value", () => {
    const { container } = render(<MonoId value="run_ABC123" />);
    const root = container.querySelector<HTMLElement>('[data-slot="mono-id"]');
    expect(root?.dataset.size).toBe("default");

    const value = container.querySelector<HTMLElement>('[data-slot="mono-id-value"]');
    expect(value?.textContent).toBe("run_abc123");
  });

  it("Should expose sm size data attribute", () => {
    const { container } = render(<MonoId value="run_abc" size="sm" />);
    const root = container.querySelector<HTMLElement>('[data-slot="mono-id"]');
    expect(root?.dataset.size).toBe("sm");
  });

  it("Should render an inline copy button only when copy is true", () => {
    const { rerender } = render(<MonoId value="run_abc" />);
    expect(screen.queryByRole("button", { name: /copy/i })).toBeNull();

    rerender(<MonoId value="run_abc" copy />);
    expect(screen.getByRole("button", { name: /copy/i })).toBeInTheDocument();
  });

  it("Should write the lowercased value to the clipboard when copy is pressed", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<MonoId value="RUN_ABC" copy />);
    fireEvent.click(screen.getByRole("button", { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith("run_abc");
    // Settle the async copied-state update inside act (aria-label flips to copiedLabel).
    await screen.findByRole("button", { name: /copied/i });
  });

  it("Should preserve casing in the rendered value when preserveCase is set", () => {
    const ref = "vault:mcp/ws/ws-platform/github-local/env/QA_TYPED_TOKEN";
    const { container } = render(<MonoId value={ref} preserveCase />);
    const value = container.querySelector<HTMLElement>('[data-slot="mono-id-value"]');
    expect(value?.textContent).toBe(ref);
  });

  it("Should copy the case-preserved value when preserveCase and copy are set", async () => {
    const ref = "vault:mcp/ws/ws-platform/github-local/env/QA_TYPED_TOKEN";
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<MonoId value={ref} preserveCase copy />);
    fireEvent.click(screen.getByRole("button", { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith(ref);
    // Settle the async copied-state update inside act (aria-label flips to copiedLabel).
    await screen.findByRole("button", { name: /copied/i });
  });
});
