// Suite: session-parent-picker
// Invariant: Move under… is keyboard-first — typing searches, Enter moves under the
// selected parent, never a row left over from a previous search — and an ineligible
// session shows the reason (with a focused Make top-level where the daemon allows it)
// instead of parents it would refuse.
// Owning layer: component (systems/session/components)
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import { SessionParentPicker, type SessionParentPickerProps } from "../session-parent-picker";

function session(id: string, lineage: Partial<NonNullable<SessionPayload["lineage"]>> = {}) {
  return {
    ...primarySessionFixture,
    id,
    name: `Session ${id}`,
    type: "user",
    archived_at: null,
    lineage: {
      ...primarySessionFixture.lineage!,
      parent_session_id: "",
      root_session_id: id,
      spawn_depth: 0,
      spawn_role: "",
      kind: "",
      ...lineage,
    },
  } as SessionPayload;
}

function renderPicker(props: Partial<SessionParentPickerProps> = {}) {
  const handlers = {
    onChoose: vi.fn(),
    onMakeTopLevel: vi.fn(),
    onSearchChange: vi.fn(),
    onOpenChange: vi.fn(),
    onRetry: vi.fn(),
    onNextPage: vi.fn(),
    onPreviousPage: vi.fn(),
  };
  render(
    <SessionParentPicker
      open
      source={session("source")}
      sourceFailed={false}
      candidates={[]}
      search=""
      loading={false}
      stale={false}
      hasNext={false}
      hasPrevious={false}
      failed={false}
      pending={false}
      {...handlers}
      {...props}
    />
  );
  return handlers;
}

describe("SessionParentPicker", () => {
  it("Should search and move under the selected parent with the keyboard", async () => {
    const user = userEvent.setup();
    const first = session("first");
    const second = session("second");
    const handlers = renderPicker({ candidates: [first, second] });
    const search = screen.getByRole("combobox", { name: "Search sessions" });
    expect(search).toHaveFocus();
    await user.keyboard("sec");
    expect(handlers.onSearchChange).toHaveBeenLastCalledWith("c");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(handlers.onChoose).toHaveBeenCalledWith(second);
    expect(handlers.onMakeTopLevel).not.toHaveBeenCalled();
  });

  it("Should explain why a forked session keeps its place", () => {
    const forked = session("forked", { parent_session_id: "origin", kind: "fork" });
    renderPicker({ source: forked, candidates: [session("peer")] });
    expect(screen.getByTestId("session-parent-picker-refusal")).toHaveTextContent(
      "Continued and forked sessions stay linked to the session they came from"
    );
    expect(screen.queryByTestId("session-parent-picker-peer")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-parent-picker-top-level")).not.toBeInTheDocument();
  });

  it("Should offer a focused Make top-level to an adopted child", async () => {
    const user = userEvent.setup();
    const child = session("child", { parent_session_id: "parent", kind: "provenance" });
    const handlers = renderPicker({ source: child });
    expect(screen.getByTestId("session-parent-picker-refusal")).toHaveTextContent(
      "Make this session top-level first"
    );
    const makeTopLevel = screen.getByRole("button", { name: "Make top-level" });
    expect(makeTopLevel).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(handlers.onMakeTopLevel).toHaveBeenCalledTimes(1);
  });

  it("Should not choose a row from a previous search while the next one loads", async () => {
    const user = userEvent.setup();
    const handlers = renderPicker({ candidates: [session("old")], stale: true });
    await user.keyboard("{Enter}");
    await user.click(screen.getByTestId("session-parent-picker-old"));
    expect(handlers.onChoose).not.toHaveBeenCalled();
  });

  it("Should page through parents beyond the first page", async () => {
    const user = userEvent.setup();
    const handlers = renderPicker({ candidates: [session("p1")], hasNext: true });
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(handlers.onNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should page, not choose the highlighted parent, on Enter from a page button", async () => {
    const user = userEvent.setup();
    const handlers = renderPicker({ candidates: [session("p1")], hasNext: true });
    screen.getByRole("button", { name: "Next" }).focus();
    await user.keyboard("{Enter}");
    expect(handlers.onNextPage).toHaveBeenCalledTimes(1);
    expect(handlers.onChoose).not.toHaveBeenCalled();
  });

  it("Should report a failed parent read with a retry", async () => {
    const user = userEvent.setup();
    const handlers = renderPicker({ failed: true });
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't load sessions.");
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(handlers.onRetry).toHaveBeenCalledTimes(1);
  });
});
