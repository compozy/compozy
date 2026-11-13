// Suite: in-place confirm delete button
// Invariant: the bin opens a cancel/confirm pair without acting; only the check confirms, and only
// when confirmation is available; the cross and Escape cancel and return focus to the bin; each
// phase is announced; a button rests again after the confirmed hold, a controlled one by telling its
// owner; a button mounted open asks for confirmation and takes focus only when its host asks; the
// host can reach the bin through a ref.
// Boundary IN: DeleteButton phases, focus, keyboard, callbacks, live region.
// Boundary OUT: what the host deletes and how it reacts.
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DeleteButton } from "../delete-button";

function setup(props: Partial<React.ComponentProps<typeof DeleteButton>> = {}) {
  const callbacks = { onConfirm: vi.fn(), onCancel: vi.fn(), onPhaseChange: vi.fn() };
  render(<DeleteButton label="Close Review" {...callbacks} {...props} />);
  return callbacks;
}

describe("DeleteButton", () => {
  it("Should open into cancel and confirm without acting, then confirm on the check", async () => {
    const user = userEvent.setup();
    const callbacks = setup();

    await user.click(screen.getByRole("button", { name: "Close Review" }));
    expect(callbacks.onConfirm).not.toHaveBeenCalled();
    expect(callbacks.onPhaseChange).toHaveBeenLastCalledWith("open");
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    expect(screen.getByRole("status")).toHaveTextContent("Close Review: confirm or cancel");

    await user.click(screen.getByRole("button", { name: "Confirm: Close Review" }));
    expect(callbacks.onConfirm).toHaveBeenCalledTimes(1);
    expect(callbacks.onPhaseChange).toHaveBeenLastCalledWith("confirmed");
    expect(screen.getByRole("group", { name: "Close Review" })).toHaveAttribute(
      "data-phase",
      "confirmed"
    );
    expect(screen.getByRole("status")).toHaveTextContent("Close Review: confirmed");
  });

  it("Should cancel with the cross or Escape and return focus to the bin", async () => {
    const user = userEvent.setup();
    const callbacks = setup();

    await user.click(screen.getByRole("button", { name: "Close Review" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(callbacks.onCancel).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Close Review" })).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.keyboard("{Escape}");
    expect(callbacks.onCancel).toHaveBeenCalledTimes(2);
    expect(callbacks.onConfirm).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Close Review" })).toHaveFocus();
  });

  it("Should keep the check unavailable until confirmation is allowed", async () => {
    const user = userEvent.setup();
    const callbacks = setup({ canConfirm: false });

    await user.click(screen.getByRole("button", { name: "Close Review" }));
    const confirm = screen.getByRole("button", { name: "Confirm: Close Review" });
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    expect(callbacks.onConfirm).not.toHaveBeenCalled();
  });

  it.each([
    { mode: "uncontrolled", props: { defaultPhase: "open" as const } },
    { mode: "controlled", props: { phase: "open" as const } },
  ])("Should mount $mode open without taking focus", ({ props }) => {
    const callbacks = setup(props);

    expect(screen.getByRole("group", { name: "Close Review" })).toHaveAttribute(
      "data-phase",
      "open"
    );
    expect(screen.getByRole("button", { name: "Cancel" })).not.toHaveFocus();
    expect(document.body).toHaveFocus();
    expect(callbacks.onPhaseChange).not.toHaveBeenCalled();
  });

  it.each([
    { mode: "uncontrolled", props: { defaultPhase: "open" as const } },
    { mode: "controlled", props: { phase: "open" as const } },
  ])("Should mount $mode open with focus on the cross when the host asks", async ({ props }) => {
    const user = userEvent.setup();
    const callbacks = setup({ ...props, autoFocusCancel: true });

    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    expect(callbacks.onPhaseChange).not.toHaveBeenCalled();

    await user.keyboard("{Escape}");
    expect(callbacks.onCancel).toHaveBeenCalledTimes(1);
    expect(callbacks.onPhaseChange).toHaveBeenLastCalledWith("rest");
  });

  it("Should hand the host its bin to return focus to", () => {
    const binRef = { current: null as HTMLButtonElement | null };
    setup({ binRef });

    expect(binRef.current).toBe(screen.getByRole("button", { name: "Close Review" }));
  });

  it("Should return an uncontrolled button to rest after the confirmed hold", async () => {
    vi.useFakeTimers();
    try {
      const callbacks = setup({ confirmedDuration: 500 });
      act(() => screen.getByRole("button", { name: "Close Review" }).click());
      act(() => screen.getByRole("button", { name: "Confirm: Close Review" }).click());
      expect(screen.getByRole("group", { name: "Close Review" })).toHaveAttribute(
        "data-phase",
        "confirmed"
      );
      act(() => vi.advanceTimersByTime(500));
      expect(screen.getByRole("group", { name: "Close Review" })).toHaveAttribute(
        "data-phase",
        "rest"
      );
      expect(callbacks.onPhaseChange).toHaveBeenLastCalledWith("rest");
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should tell a controlled owner to rest after the confirmed hold", () => {
    vi.useFakeTimers();
    try {
      const onPhaseChange = vi.fn();
      render(
        <DeleteButton
          confirmedDuration={500}
          label="Close Review"
          phase="confirmed"
          onConfirm={vi.fn()}
          onPhaseChange={onPhaseChange}
        />
      );
      expect(onPhaseChange).not.toHaveBeenCalled();
      act(() => vi.advanceTimersByTime(500));
      expect(onPhaseChange).toHaveBeenCalledExactlyOnceWith("rest");
      // The owner decides: until it passes "rest", the controlled button keeps its phase.
      expect(screen.getByRole("group", { name: "Close Review" })).toHaveAttribute(
        "data-phase",
        "confirmed"
      );
    } finally {
      vi.useRealTimers();
    }
  });
});
