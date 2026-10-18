import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Button } from "../button";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "../hover-card";

// Suite: HoverCard primitive.
// Invariant: rich read-only preview that opens after a 200 ms hover dwell or at once on focus,
// and closes on Escape and on pointer leave (UT-W15).
// Owning layer: @compozy/ui primitive. Canonical suite: this file.
function HoverCardExample() {
  return (
    <HoverCard>
      <HoverCardTrigger render={<Button>Audit webhooks</Button>} />
      <HoverCardContent>Opus 5.5 · high</HoverCardContent>
    </HoverCard>
  );
}

describe("HoverCard", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should open only after the 200 ms hover dwell and close on pointer leave (UT-W15)", async () => {
    vi.useFakeTimers();
    render(<HoverCardExample />);
    const trigger = screen.getByRole("button", { name: "Audit webhooks" });

    fireEvent.pointerEnter(trigger, { pointerType: "mouse" });
    fireEvent.mouseEnter(trigger);
    fireEvent.mouseMove(trigger);
    await act(async () => {
      vi.advanceTimersByTime(150);
    });
    expect(screen.queryByText("Opus 5.5 · high")).not.toBeInTheDocument();

    await act(async () => {
      vi.advanceTimersByTime(100);
    });
    expect(screen.getByText("Opus 5.5 · high")).toBeInTheDocument();

    fireEvent.pointerLeave(trigger, { pointerType: "mouse" });
    fireEvent.mouseLeave(trigger);
    await act(async () => {
      vi.advanceTimersByTime(1_000);
    });
    expect(screen.queryByText("Opus 5.5 · high")).not.toBeInTheDocument();
  });

  it("Should open on keyboard focus and close on Escape (UT-W15)", async () => {
    const user = userEvent.setup();
    render(<HoverCardExample />);

    await user.tab();
    expect(screen.getByRole("button", { name: "Audit webhooks" })).toHaveFocus();
    await waitFor(() => expect(screen.getByText("Opus 5.5 · high")).toBeInTheDocument());

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByText("Opus 5.5 · high")).not.toBeInTheDocument());
  });
});
