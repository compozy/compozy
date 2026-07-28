import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Check } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { DIALOG_TOUCH_TARGET_CLASS } from "../../../lib/dialog-shell";
import { Dialog, DialogContent } from "../../dialog";
import { EntityDialogFooter } from "../entity-dialog-footer";
import { UIProvider } from "../ui-provider";

function renderFooter(props: Partial<React.ComponentProps<typeof EntityDialogFooter>> = {}) {
  const onCancel = vi.fn();
  const onPrimary = vi.fn();
  render(
    <UIProvider reducedMotion="always">
      <Dialog open>
        <DialogContent data-testid="host" showCloseButton={false} unframed>
          <EntityDialogFooter
            cancelTestId="footer-cancel"
            onCancel={onCancel}
            onPrimary={onPrimary}
            primaryLabel="Create task"
            primaryTestId="footer-primary"
            {...props}
          />
        </DialogContent>
      </Dialog>
    </UIProvider>
  );
  return { onCancel, onPrimary };
}

describe("EntityDialogFooter", () => {
  it("Should render the hint slot and exactly one primary action", async () => {
    renderFooter({
      hint: "The contract is durable.",
      hintTestId: "footer-hint",
      primaryIcon: Check,
    });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const footer = screen.getByTestId("host").querySelector('[data-slot="dialog-footer"]');
    expect(footer).toHaveAttribute("data-variant", "ruled");
    expect(footer?.querySelector('[data-slot="entity-dialog-footer-primary"]')).not.toBeNull();
    expect(screen.getByTestId("footer-hint")).toHaveTextContent("The contract is durable.");

    const scope = within(footer as HTMLElement);
    expect(scope.getAllByRole("button")).toHaveLength(2);
    expect(scope.getByRole("button", { name: /create task/i })).toBe(
      screen.getByTestId("footer-primary")
    );
  });

  it("Should call onCancel and onPrimary from their controls", async () => {
    const user = userEvent.setup();
    const { onCancel, onPrimary } = renderFooter();

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    await user.click(screen.getByTestId("footer-cancel"));
    await user.click(screen.getByTestId("footer-primary"));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onPrimary).toHaveBeenCalledTimes(1);
  });

  it("Should show a spinner and block duplicate submits while saving", async () => {
    const user = userEvent.setup();
    const { onPrimary } = renderFooter({ isSaving: true });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const primary = screen.getByTestId("footer-primary");
    expect(primary).toBeDisabled();
    expect(within(primary).getByRole("status", { name: "Loading" })).toBeInTheDocument();

    await user.click(primary);
    expect(onPrimary).not.toHaveBeenCalled();
  });

  it("Should raise both actions to the touch target below 760px", async () => {
    renderFooter({ hint: "Consequence note." });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    expect(screen.getByTestId("footer-cancel")).toHaveClass(DIALOG_TOUCH_TARGET_CLASS);
    expect(screen.getByTestId("footer-primary")).toHaveClass(DIALOG_TOUCH_TARGET_CLASS);
  });

  it("Should keep the consequence note ahead of its action when stacked", async () => {
    renderFooter({ hint: "Consequence note.", hintTestId: "footer-hint" });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const footer = screen.getByTestId("host").querySelector('[data-slot="dialog-footer"]');
    // Source order is note -> cancel -> primary, and the stacked layout must not
    // reverse it: a note printed after its action explains nothing.
    const order = Array.from(footer?.children ?? []).map(el => el.getAttribute("data-slot"));
    expect(order).toEqual([
      "entity-dialog-footer-hint",
      "entity-dialog-footer-cancel",
      "entity-dialog-footer-primary",
    ]);
    expect(footer?.className).toContain("max-[760px]:flex-col");
    expect(footer?.className).not.toContain("max-[760px]:flex-col-reverse");
  });

  it("Should keep both actions reachable by keyboard in visual order", async () => {
    const user = userEvent.setup();
    renderFooter({ hint: "Consequence note." });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    screen.getByTestId("footer-cancel").focus();
    expect(screen.getByTestId("footer-cancel")).toHaveFocus();
    await user.tab();
    expect(screen.getByTestId("footer-primary")).toHaveFocus();
  });

  it("Should submit its owning form when primaryType is submit", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <UIProvider reducedMotion="always">
        <form onSubmit={onSubmit}>
          <EntityDialogFooter
            onCancel={vi.fn()}
            primaryLabel="Save changes"
            primaryTestId="submit-primary"
            primaryType="submit"
          />
        </form>
      </UIProvider>
    );

    await user.click(screen.getByTestId("submit-primary"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });
});
