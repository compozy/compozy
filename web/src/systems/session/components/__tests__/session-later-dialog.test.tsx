// Invariant (#1076): Move to Later sends only what the operator typed — a trimmed one-line
// reason and a return date read in the operator's own time zone — and refuses a return date
// that is not in the future. Owning layer: the dialog component (one suite per dialog).
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { laterInputValue, laterReturnAtFromInput } from "../../lib/session-later";
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

  // `Date` would roll a wall-clock time that does not exist (a DST gap, Feb 30)
  // onto another instant; the parser refuses it instead. The browser input
  // sanitizes Feb 30 away, so the parser is exercised directly; a DST gap reads
  // the same field mismatch in zones that have one.
  it("Should refuse a local time that does not exist instead of shifting it", () => {
    expect(laterReturnAtFromInput("2027-02-30T09:00", 0)).toEqual({
      returnAt: null,
      error: "Enter a date and time.",
    });
    expect(laterReturnAtFromInput("2027-02-28T25:00", 0).returnAt).toBeNull();
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

  it("Should keep the typed reason when a partial failure narrows the sessions", () => {
    const other = { ...primarySessionFixture, id: "other" };
    const props = { isMoving: false, onConfirm: vi.fn(), onOpenChange: vi.fn(), open: true };
    const { rerender } = render(
      <SessionLaterDialog {...props} sessions={[primarySessionFixture, other]} />
    );
    fireEvent.change(screen.getByTestId("session-later-reason"), {
      target: { value: "After the release" },
    });
    rerender(<SessionLaterDialog {...props} sessions={[other]} />);
    expect(screen.getByRole("heading", { name: "Move to Later" })).toBeInTheDocument();
    expect(screen.getByTestId("session-later-reason")).toHaveValue("After the release");
  });
});
