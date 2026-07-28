import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EditorFooter } from "../editor-footer";

describe("EditorFooter", () => {
  it("Should render meta, secondary, and primary slots", () => {
    render(
      <EditorFooter
        meta={<span>Last saved 5m ago</span>}
        secondary={<button type="button">Discard</button>}
        primary={<button type="button">Save</button>}
      />
    );
    expect(screen.getByText("Last saved 5m ago")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /discard/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /save/i })).toBeInTheDocument();
  });

  it("Should mount sticky", () => {
    const { container } = render(<EditorFooter primary={<span />} />);
    const root = container.querySelector<HTMLElement>('[data-slot="editor-footer"]');
    expect(root?.className).toContain("sticky");
  });

  it("Should leave Escape unhandled by default", () => {
    const onEscape = vi.fn();
    render(<EditorFooter primary={<button type="button">Save</button>} onEscape={onEscape} />);
    fireEvent.keyDown(screen.getByRole("button"), { key: "Escape" });
    expect(onEscape).not.toHaveBeenCalled();
  });

  it("Should call onEscape when escapeHandler is true", () => {
    const onEscape = vi.fn();
    render(
      <EditorFooter
        escapeHandler
        onEscape={onEscape}
        primary={<button type="button">Save</button>}
      />
    );
    fireEvent.keyDown(screen.getByRole("button"), { key: "Escape" });
    expect(onEscape).toHaveBeenCalledTimes(1);
  });
});
