import { useState } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ResizableDrawer } from "../resizable-drawer";
import { UIProvider } from "../ui-provider";

// Invariant: opening/closing detail preserves host state and returns keyboard focus.
// Owner and canonical suite: the shared ResizableDrawer primitive.
function Harness() {
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
        returnFocus={trigger}
        title="Child session"
        content={<button type="button">Read activity</button>}
      >
        <div>
          <input aria-label="Parent draft" />
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
});
