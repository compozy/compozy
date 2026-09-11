// Suite: OS traffic lights
// Invariant: window controls are identifiable without hover (touch has none)
// and the compact tier keeps every control a phone needs reachable.
// Boundary IN: OsTrafficLights presentation, accessibility, compact glyphs.
// Boundary OUT: the owning frame's command dispatch (os-window-frame suite).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { OsTrafficLights } from "../os-traffic-lights";

describe("OsTrafficLights", () => {
  it("Should keep all three controls with distinct labels on the floating tier", () => {
    render(<OsTrafficLights onSelect={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Close window" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimize window" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zoom window" })).toBeInTheDocument();
  });

  // T2 ratified: compact hides zoom (meaningless in a stack), never close —
  // a phone must be able to close a window from its own bar.
  it("Should hide only zoom at the compact tier, keeping close reachable", () => {
    render(<OsTrafficLights compact onSelect={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Close window" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Minimize window" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Zoom window" })).not.toBeInTheDocument();
  });

  // T10 real-device delta: hover tones are the only desktop identification and
  // touch has no hover, so compact controls carry their glyph at rest.
  it("Should identify close and minimize by glyph at rest in compact mode", () => {
    render(<OsTrafficLights compact onSelect={vi.fn()} />);

    const close = screen.getByRole("button", { name: "Close window" });
    expect(close.querySelector("svg")).toBeInTheDocument();
    const minimize = screen.getByRole("button", { name: "Minimize window" });
    expect(minimize.querySelector("svg")).toBeInTheDocument();
  });

  it("Should keep the floating tier glyph-free outside zoom", () => {
    render(<OsTrafficLights onSelect={vi.fn()} />);

    // Desktop rendering stays pixel-identical: only zoom carries a glyph.
    expect(screen.getByRole("button", { name: "Close window" }).querySelector("svg")).toBeNull();
    expect(screen.getByRole("button", { name: "Minimize window" }).querySelector("svg")).toBeNull();
    expect(screen.getByRole("button", { name: "Zoom window" }).querySelector("svg")).not.toBeNull();
  });

  it("Should keep inert compact chrome glyphless — no implied controls", () => {
    const { container } = render(<OsTrafficLights compact />);

    // Inert chrome: no buttons, no glyphs, just the two separated circles.
    expect(container.querySelector("button")).toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelectorAll("[data-action]")).toHaveLength(2);
  });

  it("Should dispatch the compact actions through the owning frame", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<OsTrafficLights compact onSelect={onSelect} />);

    await user.click(screen.getByRole("button", { name: "Close window" }));
    await user.click(screen.getByRole("button", { name: "Minimize window" }));
    expect(onSelect).toHaveBeenNthCalledWith(1, "close");
    expect(onSelect).toHaveBeenNthCalledWith(2, "minimize");
  });
});
