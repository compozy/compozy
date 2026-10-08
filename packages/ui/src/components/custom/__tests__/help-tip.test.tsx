import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Field, FieldHeader, FieldLabel } from "../../field";
import { Dialog, DialogContent, DialogTitle } from "../../dialog";
import { Input } from "../../input";
import { TooltipProvider } from "../../tooltip";
import { HelpTip } from "../help-tip";

function renderTip(ui: React.ReactNode) {
  return render(<TooltipProvider delay={0}>{ui}</TooltipProvider>);
}

describe("HelpTip", () => {
  afterEach(() => {
    vi.useRealTimers();
  });
  it("Should expose a named button that keyboard users can reach", async () => {
    const user = userEvent.setup();
    renderTip(<HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>);

    await user.tab();

    expect(screen.getByRole("button", { name: "About category path" })).toHaveFocus();
  });

  it.each(["mouse", "touch"] as const)(
    "Should keep prose readable after %s activation until dismissal",
    async pointer => {
      vi.useFakeTimers({
        // RTL drains userEvent through a zero-delay timer outside advanceTimers.
        shouldAdvanceTime: true,
        toFake: [
          "setTimeout",
          "clearTimeout",
          "setInterval",
          "clearInterval",
          "Date",
          "performance",
          "requestAnimationFrame",
          "cancelAnimationFrame",
        ],
      });
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
      const { unmount } = renderTip(
        <HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>
      );
      try {
        const trigger = screen.getByRole("button", { name: "About category path" });
        expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument();

        if (pointer === "mouse") await user.click(trigger);
        else {
          await user.pointer({ keys: "[TouchA]", target: trigger });
          // Chrome can leave the emulated mouse hover after a completed touch tap.
          fireEvent.mouseLeave(trigger, { relatedTarget: document.body });
        }
        // Advance the readable interval and animation clock: an exit-retained
        // node must not make a flash pass.
        await act(() => vi.advanceTimersByTimeAsync(1000));
        expect(screen.getByText("Slash-separated catalog grouping.")).toBeInTheDocument();

        await user.keyboard("{Escape}");
        await act(() => vi.advanceTimersByTimeAsync(1000));
        expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument();
      } finally {
        unmount();
        vi.useRealTimers();
      }
    }
  );

  it("Should retain outside and hover dismissal after touch guidance", async () => {
    const user = userEvent.setup();
    renderTip(
      <>
        <HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>
        <button type="button">Continue editing</button>
      </>
    );
    const trigger = screen.getByRole("button", { name: "About category path" });

    await user.pointer({ keys: "[TouchA]", target: trigger });
    await screen.findByText("Slash-separated catalog grouping.");
    await user.click(screen.getByRole("button", { name: "Continue editing" }));
    await waitFor(() =>
      expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument()
    );

    await user.hover(trigger);
    await screen.findByText("Slash-separated catalog grouping.");
    await user.unhover(trigger);
    await waitFor(() =>
      expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument()
    );
  });

  it.each(["hover", "focus", "click"] as const)(
    "Should preserve the dialog draft when Escape dismisses a tip opened by %s",
    async opening => {
      const user = userEvent.setup();
      renderTip(
        <Dialog defaultOpen>
          <DialogContent showCloseButton={false}>
            <DialogTitle>Create task</DialogTitle>
            <Input aria-label="Title" />
            <HelpTip label="About title">Describe the work to hand to an agent.</HelpTip>
          </DialogContent>
        </Dialog>
      );
      await waitFor(() => expect(screen.getByRole("dialog")).toHaveFocus());
      const title = screen.getByRole("textbox", { name: "Title" });
      const tip = screen.getByRole("button", { name: "About title" });
      await user.type(title, "Prepare release notes");

      if (opening === "hover") await user.hover(tip);
      else if (opening === "focus") await user.tab();
      else await user.click(tip);
      await screen.findByText("Describe the work to hand to an agent.");

      await user.keyboard("{Escape}");

      await waitFor(() =>
        expect(screen.queryByText("Describe the work to hand to an agent.")).not.toBeInTheDocument()
      );
      expect(screen.getByRole("dialog", { name: "Create task" })).toBeInTheDocument();
      expect(title).toHaveValue("Prepare release notes");
      expect(opening === "hover" ? title : tip).toHaveFocus();

      await user.keyboard("{Escape}");
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    }
  );

  it("Should keep the trigger out of the label's accessible name", () => {
    renderTip(
      <Field>
        <FieldHeader>
          <FieldLabel htmlFor="agent-name">Agent name</FieldLabel>
          <HelpTip label="About agent name">Lowercase slug sessions launch from.</HelpTip>
        </FieldHeader>
        <Input id="agent-name" />
      </Field>
    );

    // Nesting the trigger inside the `<label>` would fold "About agent name"
    // into the input's name-from-content computation.
    expect(screen.getByLabelText("Agent name")).toBe(screen.getByRole("textbox"));
  });

  it("Should preserve Escape ownership in a peer dialog", async () => {
    const user = userEvent.setup();
    renderTip(
      <>
        <Dialog defaultOpen modal={false} disablePointerDismissal>
          <DialogContent showCloseButton={false}>
            <DialogTitle>Task editor</DialogTitle>
            <Input aria-label="Task title" />
            <HelpTip label="About title">Describe the work to hand to an agent.</HelpTip>
          </DialogContent>
        </Dialog>
        <Dialog defaultOpen modal={false} disablePointerDismissal>
          <DialogContent showCloseButton={false}>
            <DialogTitle>Peer editor</DialogTitle>
            <Input aria-label="Peer title" />
          </DialogContent>
        </Dialog>
      </>
    );
    await waitFor(() => expect(screen.getByRole("dialog", { name: "Peer editor" })).toHaveFocus());
    await user.type(screen.getByRole("textbox", { name: "Peer title" }), "Other work");
    await user.hover(screen.getByRole("button", { name: "About title" }));
    await screen.findByText("Describe the work to hand to an agent.");

    await user.keyboard("{Escape}");

    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: "Peer editor" })).not.toBeInTheDocument()
    );
    expect(screen.getByRole("dialog", { name: "Task editor" })).toBeInTheDocument();
  });
});
