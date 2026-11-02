// Invariant (#1076, #1086): Move to Later sends only what the operator wrote — the
// reason as trimmed Markdown within the daemon's limit and a picked return date as
// an RFC 3339 instant — and refuses a return date that is no longer in the future.
// Owning layer: the dialog component (one suite per dialog).
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { laterInThreeHours, SESSION_LATER_REASON_MAX } from "../../lib/session-later";
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

function tomorrowAtNine(): Date {
  const date = new Date();
  date.setDate(date.getDate() + 1);
  date.setHours(9, 0, 0, 0);
  return date;
}

async function pickPreset(user: ReturnType<typeof userEvent.setup>, label: string) {
  // The dialog focuses the reason editor after mount; that late focus move would
  // dismiss a popover opened before it lands.
  await waitFor(() => expect(screen.getByRole("textbox", { name: "Reason" })).toHaveFocus());
  await user.click(screen.getByRole("button", { name: /Pick a date/ }));
  await user.click(await screen.findByRole("button", { name: label }));
}

// Presets resolve against the clock when picked; a frozen date keeps the expected
// "Tomorrow" equal to the picked one even across midnight.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2027, 2, 10, 14, 0));
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("SessionLaterDialog", () => {
  it("Should send nothing but the confirm when both fields are left empty", () => {
    const { onConfirm } = renderDialog();
    expect(screen.getByRole("heading", { name: "Move to Later" })).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("session-later-confirm"));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith({});
  });

  it("Should send the formatted reason as Markdown and a picked date as an RFC 3339 instant", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderDialog();
    const editor = await screen.findByRole("textbox", { name: "Reason" });
    await waitFor(() => expect(editor).toHaveFocus());
    // Typing into a contenteditable is unreliable in jsdom: the note lands as a DOM
    // edit, and the toolbar mark is the formatting path under test.
    editor.textContent = "  After the release  ";
    await waitFor(() => expect(editor).toHaveTextContent("After the release"));
    await user.click(editor);
    await user.keyboard("{Control>}a{/Control}");
    await user.click(screen.getByRole("button", { name: "Bold" }));
    await pickPreset(user, "Tomorrow");
    await user.click(screen.getByTestId("session-later-confirm"));
    await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1));
    const request = onConfirm.mock.calls[0]?.[0] as { reason?: string; return_at?: string };
    expect(request.return_at).toBe(
      tomorrowAtNine()
        .toISOString()
        .replace(/\.\d{3}Z$/, "Z")
    );
    expect(request.reason).toBe("**After the release**");
  });

  it("Should submit from the reason editor with Cmd/Ctrl+Enter", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderDialog();
    await user.click(await screen.findByRole("textbox", { name: "Reason" }));
    await user.keyboard("{Control>}{Enter}{/Control}");
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith({});
  });

  it("Should hold a reason past the daemon limit", async () => {
    const { onConfirm } = renderDialog();
    const editor = await screen.findByRole("textbox", { name: "Reason" });
    editor.textContent = "a".repeat(SESSION_LATER_REASON_MAX + 1);
    await waitFor(() =>
      expect(screen.getByText(`Use ${SESSION_LATER_REASON_MAX} characters or fewer.`)).toBeVisible()
    );
    expect(screen.getByTestId("session-later-confirm")).toBeDisabled();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("Should refuse a return date that passed while the dialog was open", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderDialog();
    await pickPreset(user, "Tomorrow");
    vi.setSystemTime(tomorrowAtNine().getTime() + 60_000);
    await user.click(screen.getByTestId("session-later-confirm"));
    expect(screen.getByTestId("session-later-return-error")).toHaveTextContent(
      "Choose a time in the future."
    );
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('Should never return "In 3 hours" earlier than three hours out', () => {
    const zone = process.env.TZ;
    process.env.TZ = "America/New_York";
    try {
      // 23:10 EDT the night clocks go back: three hours later is 01:10 EST, and the
      // next half hour is 01:30 EST, not the earlier 01:30 EDT.
      const now = Date.parse("2026-11-01T03:10:00Z");
      const at = laterInThreeHours(now);
      expect(at.toISOString()).toBe("2026-11-01T06:30:00.000Z");
      expect(at.getTime() - now).toBeGreaterThanOrEqual(3 * 60 * 60_000);
    } finally {
      if (zone === undefined) delete process.env.TZ;
      else process.env.TZ = zone;
    }
  });

  it("Should clear a picked return date", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderDialog();
    await pickPreset(user, "Tomorrow");
    await user.click(screen.getByRole("button", { name: "Clear date" }));
    await user.click(screen.getByTestId("session-later-confirm"));
    expect(onConfirm).toHaveBeenCalledExactlyOnceWith({});
  });

  it("Should quote a Markdown title as plain text", () => {
    renderDialog({ sessions: [{ ...primarySessionFixture, name: "**Notice:** fix the gate" }] });
    expect(screen.getByText(/“Notice: fix the gate” stops/)).toBeInTheDocument();
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

  it("Should keep the reason and date when a partial failure narrows the sessions", async () => {
    const user = userEvent.setup();
    const other = { ...primarySessionFixture, id: "other" };
    const onConfirm = vi.fn();
    const props = { isMoving: false, onConfirm, onOpenChange: vi.fn(), open: true };
    const { rerender } = render(
      <SessionLaterDialog {...props} sessions={[primarySessionFixture, other]} />
    );
    const editor = await screen.findByRole("textbox", { name: "Reason" });
    // Same entry path as the formatting test: a DOM edit, then a toolbar transaction
    // that reports the whole note to the dialog.
    editor.textContent = "After the release";
    await waitFor(() => expect(editor).toHaveTextContent("After the release"));
    await user.click(editor);
    await user.keyboard("{Control>}a{/Control}");
    await user.click(screen.getByRole("button", { name: "Bold" }));
    await pickPreset(user, "Tomorrow");
    rerender(<SessionLaterDialog {...props} sessions={[other]} />);
    expect(screen.getByRole("heading", { name: "Move to Later" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Reason" })).toHaveTextContent("After the release");
    await user.click(screen.getByTestId("session-later-confirm"));
    await waitFor(() =>
      expect(onConfirm).toHaveBeenCalledExactlyOnceWith({
        reason: "**After the release**",
        return_at: tomorrowAtNine()
          .toISOString()
          .replace(/\.\d{3}Z$/, "Z"),
      })
    );
  });
});
