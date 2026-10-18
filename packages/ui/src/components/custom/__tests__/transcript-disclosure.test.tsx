// Suite: TranscriptDisclosure behavior
// Invariant: the trigger is a controlled, keyboard-operable toggle that reports
// its state and forwards button attributes; the caller owns the body.
// Boundary IN: TranscriptDisclosure. Boundary OUT: the revealed body.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TranscriptDisclosure } from "../transcript-disclosure";

describe("TranscriptDisclosure", () => {
  it("Should report its expanded state and toggle from click and keyboard", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    const { rerender } = render(
      <TranscriptDisclosure
        aria-controls="body"
        data-testid="fold"
        expanded={false}
        icon={<span />}
        label="Ran command"
        onToggle={onToggle}
      />
    );
    const trigger = screen.getByTestId("fold");
    expect(trigger).toHaveAttribute("type", "button");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveAttribute("aria-controls", "body");

    await user.click(trigger);
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(onToggle).toHaveBeenCalledTimes(2);

    rerender(
      <TranscriptDisclosure
        data-testid="fold"
        expanded
        icon={<span />}
        label="Ran command"
        onToggle={onToggle}
      />
    );
    expect(trigger).toHaveAttribute("aria-expanded", "true");
  });

  it("Should render the trailing slot on a row and omit the icon well on a turn fold", () => {
    const { rerender } = render(
      <TranscriptDisclosure
        expanded={false}
        icon={<span data-testid="icon" />}
        label="Edited file"
        onToggle={vi.fn()}
        trailing={<span data-testid="stat">+2</span>}
      />
    );
    expect(screen.getByTestId("icon")).toBeInTheDocument();
    expect(screen.getByTestId("stat")).toBeInTheDocument();

    rerender(
      <TranscriptDisclosure
        expanded={false}
        icon={<span data-testid="icon" />}
        label="Worked for 2m"
        onToggle={vi.fn()}
        variant="turn"
      />
    );
    expect(screen.queryByTestId("icon")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Worked for 2m" })).toBeInTheDocument();
  });
});
