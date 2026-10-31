// Invariant (#1076): Move to Later sends only what the operator typed — a trimmed one-line
// reason and a return date read in the operator's own time zone — and refuses a return date
// that is not in the future. Owning layer: the dialog component (one suite per dialog).
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { laterInputValue } from "../../lib/session-later";
import { SessionLaterDialog } from "../session-later-dialog";

function renderDialog(overrides: Partial<Parameters<typeof SessionLaterDialog>[0]> = {}) {
  const onConfirm = vi.fn();
  render(
    <SessionLaterDialog
      isMoving={false}
      onConfirm={onConfirm}
      onOpenChange={vi.fn()}
      open
      sessions={[primarySessionFixture]}
      {...overrides}
    />
  );
  return { onConfirm };
}

describe("SessionLaterDialog", () => {
  it("Should send nothing but the confirm when both fields are left empty", () => {
    const { onConfirm } = renderDialog();
    expect(screen.getByRole("heading", { name: "Move to Later" })).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("session-later-confirm"));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith({});
  });

  it("Should send a trimmed reason and the local return time as an RFC 3339 instant", () => {
    const { onConfirm } = renderDialog();
    const local = new Date(Date.now() + 3 * 24 * 60 * 60_000);
    local.setSeconds(0, 0);
    fireEvent.change(screen.getByTestId("session-later-reason"), {
      target: { value: "  After the release  " },
    });
    fireEvent.change(screen.getByTestId("session-later-return"), {
      target: { value: laterInputValue(local) },
    });
    fireEvent.click(screen.getByTestId("session-later-confirm"));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith({
      reason: "After the release",
      return_at: local.toISOString().replace(/\.\d{3}Z$/, "Z"),
    });
  });

  it("Should refuse a return date in the past", () => {
    const { onConfirm } = renderDialog();
    fireEvent.change(screen.getByTestId("session-later-return"), {
      target: { value: laterInputValue(new Date(Date.now() - 60 * 60_000)) },
    });
    fireEvent.click(screen.getByTestId("session-later-confirm"));
    expect(screen.getByTestId("session-later-return-error")).toHaveTextContent(
      "Choose a time in the future."
    );
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("Should name a bulk move and hold the confirm while the sessions stop", () => {
    renderDialog({
      isMoving: true,
      sessions: [primarySessionFixture, { ...primarySessionFixture, id: "other" }],
    });
    expect(screen.getByRole("heading", { name: "Move 2 sessions to Later" })).toBeInTheDocument();
    expect(screen.getByTestId("session-later-confirm")).toBeDisabled();
    expect(screen.getByTestId("session-later-confirm")).toHaveTextContent("Moving to Later");
  });
});
