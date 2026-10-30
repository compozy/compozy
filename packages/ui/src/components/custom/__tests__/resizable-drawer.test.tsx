import { useState } from "react";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ResizableDrawer } from "../resizable-drawer";
import { UIProvider } from "../ui-provider";

// Invariant: opening/closing detail preserves host state and returns keyboard focus.
// Owner and canonical suite: the shared ResizableDrawer primitive.
function Harness({
  basis,
  header,
  focusTarget,
}: {
  basis?: "viewport" | "container";
  header?: boolean;
  focusTarget?: () => HTMLElement | null;
} = {}) {
  const [open, setOpen] = useState(false);
  const [trigger, setTrigger] = useState<HTMLElement | null>(null);
  const [width, setWidth] = useState(480);
  return (
    <UIProvider>
      <ResizableDrawer
        open={open}
        onOpenChange={setOpen}
        width={width}
        onWidthChange={setWidth}
        returnFocus={focusTarget ?? trigger}
        breakpointBasis={basis}
        inlineBreakpoint={basis === "container" ? 760 : undefined}
        header={
          header ? (
            <div>
              <span>Custom head</span>
              <button type="button" onClick={() => setOpen(false)}>
                Dismiss
              </button>
            </div>
          ) : undefined
        }
        title="Child session"
        content={<button type="button">Read activity</button>}
      >
        <div>
          <input aria-label="Parent draft" />
          <button type="button">Landing card</button>
          <button
            type="button"
            onClick={event => {
              setTrigger(event.currentTarget);
              setOpen(true);
            }}
          >
            Open child
          </button>
        </div>
      </ResizableDrawer>
    </UIProvider>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("ResizableDrawer", () => {
  // Invariant: the keyboard separator respects its advertised width limits.
  // Owner: this primitive; jsdom's absent layout is the mocked browser boundary.
  it("Should resize with the keyboard within the minimum and maximum", async () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }))
    );
    vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1600);
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "Open child" }));
    const separator = await screen.findByRole("separator", { name: "Resize drawer" });
    separator.focus();
    await user.keyboard("{End}");
    expect(separator).toHaveAttribute("aria-valuenow", "800");
    await user.keyboard("{ArrowRight}");
    expect(separator).toHaveAttribute("aria-valuenow", "784");
    await user.keyboard("{Home}{ArrowRight}");
    expect(separator).toHaveAttribute("aria-valuenow", "360");
  });
  it.each([true, false])(
    "Should preserve the parent and restore focus after Escape (inline=%s)",
    async inline => {
      vi.stubGlobal(
        "matchMedia",
        vi.fn(() => ({ matches: inline, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
      );
      const user = userEvent.setup();
      render(<Harness />);
      fireEvent.change(screen.getByLabelText("Parent draft"), {
        target: { value: "Unsent draft" },
      });
      const trigger = screen.getByRole("button", { name: "Open child" });
      await user.click(trigger);
      await waitFor(() =>
        expect(screen.getByRole("dialog", { name: "Child session" })).toBeVisible()
      );
      await user.keyboard("{Escape}");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(screen.getByLabelText("Parent draft")).toHaveValue("Unsent draft");
      await waitFor(() => expect(trigger).toHaveFocus());
    }
  );

  // Invariant: a contained drawer follows its host's width, not the viewport's, and covers a
  // narrow host edge to edge. Owner: this primitive; the measured width is the browser boundary.
  it.each([
    [1200, "inline"],
    [600, "overlay"],
  ])("Should choose the layout from a %ipx host (container basis)", async (hostWidth, mode) => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    );
    vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(hostWidth);
    const user = userEvent.setup();
    render(<Harness basis="container" />);
    await user.click(screen.getByRole("button", { name: "Open child" }));
    const dialog = await screen.findByRole("dialog", { name: "Child session" });
    expect(dialog).toHaveAttribute("data-mode", mode);
    if (mode === "overlay") expect(dialog.style.width).toBe("100%");
  });

  // Invariant: a replacement head keeps the dialog named by `title`, owns closing, and focus
  // returns to the target resolved at close time.
  it("Should name a custom head by title and return focus to the resolved target", async () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    );
    const user = userEvent.setup();
    render(<Harness header focusTarget={() => screen.getByText("Landing card")} />);
    await user.click(screen.getByRole("button", { name: "Open child" }));
    const dialog = await screen.findByRole("dialog", { name: "Child session" });
    expect(within(dialog).getByText("Custom head")).toBeVisible();
    expect(within(dialog).queryByRole("button", { name: "Close drawer" })).toBeNull();
    await user.click(within(dialog).getByRole("button", { name: "Dismiss" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole("button", { name: "Landing card" })).toHaveFocus());
  });

  // Invariant: a host that loses its layout box (a hidden window) keeps the drawer where it
  // was instead of remounting it into the overlay, and stays quiet until first measured.
  it("Should keep the inline layout while its host has no layout box", async () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    );
    const width = vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1200);
    const user = userEvent.setup();
    const { rerender } = render(<Harness basis="container" />);
    await user.click(screen.getByRole("button", { name: "Open child" }));
    const dialog = await screen.findByRole("dialog", { name: "Child session" });
    expect(dialog).toHaveAttribute("data-mode", "inline");

    width.mockReturnValue(0);
    rerender(<Harness basis="container" />);
    expect(screen.getByRole("dialog", { name: "Child session" })).toBe(dialog);
  });

  // Invariant: the contained layout follows its host live — crossing the breakpoint while open
  // moves the drawer between inline and edge to edge. Owner: this primitive's host observation.
  it("Should follow its host across the breakpoint while open", async () => {
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    );
    const observed: Array<() => void> = [];
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          observed.push(callback);
        }
        observe() {}
        unobserve() {}
        disconnect() {}
      }
    );
    const width = vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(1200);
    const user = userEvent.setup();
    render(<Harness basis="container" />);
    await user.click(screen.getByRole("button", { name: "Open child" }));
    expect(await screen.findByRole("dialog", { name: "Child session" })).toHaveAttribute(
      "data-mode",
      "inline"
    );

    width.mockReturnValue(600);
    act(() => observed.forEach(callback => callback()));
    await waitFor(() =>
      expect(screen.getByRole("dialog", { name: "Child session" })).toHaveAttribute(
        "data-mode",
        "overlay"
      )
    );
  });
});
