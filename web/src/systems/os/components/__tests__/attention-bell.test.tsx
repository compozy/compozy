// Suite: OS attention bell rows
// Invariant: the bell renders Needs you and Finished as separate, populated-only
// sections; finished-unseen work stays visible without a dismiss control; muted
// and stale rows stay listed and activatable; loop-node rows render only when
// supplied and hand their deep-link state to the host.
// Boundary IN: AttentionBell props, section shape, and row union.
// Boundary OUT: menubar navigation and the cross-workspace jump.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Popover } from "@compozy/ui";

import type {
  OsAttentionSections,
  OsLoopNodeAttentionRow,
  OsLoopRequestAttentionRow,
  OsSessionAttentionRow,
} from "../../lib/attention-model";
import { AttentionBell } from "../attention-bell";

const WAITING: OsLoopNodeAttentionRow = {
  kind: "loop-node",
  id: "waiting",
  title: "Loop nodes waiting on you",
  state: "waiting",
};

const ATTENTION: OsLoopNodeAttentionRow = {
  kind: "loop-node",
  id: "attention",
  title: "Loop nodes needing attention",
  state: "attention",
};

const LOOP_REQUEST: OsLoopRequestAttentionRow = {
  kind: "loop-request",
  id: "ws-release:run-1:publish:0",
  title: "publish",
  workspaceId: "ws-release",
  workspaceLabel: "release",
  runId: "run-1",
  loopName: "release-train",
  nodeId: "publish",
  itemIndex: 0,
  requestKind: "review",
  openedAt: "2026-07-20T12:00:00Z",
  expiresAt: "2026-07-20T12:04:00Z",
  stale: false,
};

function sessionRow(overrides: Partial<OsSessionAttentionRow> = {}): OsSessionAttentionRow {
  return {
    kind: "session",
    id: "sess-1",
    title: "Refactor session store",
    agentName: "claude",
    workspaceId: "ws-compozy",
    workspaceLabel: "compozy",
    badge: "waiting-for-input",
    reason: "Should I drop the legacy column?",
    changedAt: "2026-07-20T12:00:00Z",
    muted: false,
    stale: false,
    ...overrides,
  };
}

function renderBell(sections: Partial<OsAttentionSections> = {}, onSelect = vi.fn()) {
  render(
    <Popover open>
      <AttentionBell
        loading={false}
        onSelect={onSelect}
        sections={{ needsYou: [], finished: [], ...sections }}
        sessionsDisconnected={false}
        tasksDisconnected={false}
      />
    </Popover>
  );
  return onSelect;
}

describe("AttentionBell sections", () => {
  it("Should render only populated sections — no empty headers", () => {
    renderBell({ needsYou: [sessionRow()] });

    expect(screen.getByTestId("os-bell-needs-you")).toBeInTheDocument();
    expect(screen.queryByTestId("os-bell-finished")).not.toBeInTheDocument();
  });

  it("Should keep finished-unseen work in its own section", () => {
    renderBell({
      needsYou: [sessionRow()],
      finished: [sessionRow({ id: "sess-done", badge: "done", title: "Release notes draft" })],
    });

    expect(screen.getByTestId("os-bell-needs-you")).toBeInTheDocument();
    expect(screen.getByTestId("os-bell-finished")).toBeInTheDocument();
    expect(screen.getByText("Release notes draft")).toBeInTheDocument();
  });

  it("Should offer no manual dismiss — viewing a session is the only way to clear it", () => {
    renderBell({
      finished: [sessionRow({ id: "sess-done", badge: "done" })],
    });

    expect(screen.queryByRole("button", { name: /mark .*seen/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /dismiss/i })).not.toBeInTheDocument();
  });

  it("Should name the workspace and the reason on a row", () => {
    renderBell({ needsYou: [sessionRow()] });

    expect(screen.getByText("compozy")).toBeInTheDocument();
    expect(screen.getByText("claude — Should I drop the legacy column?")).toBeInTheDocument();
  });

  it("Should keep a muted workspace's row listed and mark the silence (US-015.AC-1)", () => {
    renderBell({ needsYou: [sessionRow({ muted: true })] });

    const row = screen.getByTestId("os-attention-session-sess-1");
    expect(row).toHaveAttribute("data-muted", "true");
    expect(screen.getByLabelText("Notifications muted")).toBeInTheDocument();
  });

  it("Should keep a stale row activatable as a fallback jump (US-005.EC-2)", async () => {
    const user = userEvent.setup();
    const row = sessionRow({ stale: true });
    const onSelect = renderBell({ needsYou: [row] });

    const rendered = screen.getByTestId("os-attention-session-sess-1");
    expect(rendered).toHaveAttribute("data-stale", "true");

    await user.click(rendered);
    expect(onSelect).toHaveBeenCalledWith(row);
  });

  it("Should render the quiet state at zero attention (US-005.EC-1)", () => {
    renderBell();

    expect(screen.getByTestId("os-bell-empty")).toBeInTheDocument();
    expect(screen.getByText("All quiet")).toBeInTheDocument();
    expect(screen.queryByTestId("os-attention-loop-node-waiting")).not.toBeInTheDocument();
  });

  it("Should state that a disconnected source is frozen and uncounted", () => {
    render(
      <Popover open>
        <AttentionBell
          loading={false}
          onSelect={vi.fn()}
          sections={{ needsYou: [sessionRow({ stale: true })], finished: [] }}
          sessionsDisconnected
          tasksDisconnected={false}
        />
      </Popover>
    );

    expect(screen.getByTestId("os-bell-disconnected")).toHaveTextContent(
      "Session attention is unavailable. Frozen rows do not count."
    );
  });

  it("Should render probed loop-node rows and deep-link through onSelect", async () => {
    const user = userEvent.setup();
    const onSelect = renderBell({ needsYou: [WAITING, ATTENTION] });

    await user.click(screen.getByTestId("os-attention-loop-node-waiting"));
    expect(onSelect).toHaveBeenCalledWith(WAITING);

    await user.click(screen.getByTestId("os-attention-loop-node-attention"));
    expect(onSelect).toHaveBeenCalledWith(ATTENTION);
  });

  it("Should render and activate a workspace-labeled loop request with kind, loop, and age", async () => {
    const user = userEvent.setup();
    const onSelect = renderBell({ needsYou: [LOOP_REQUEST] });

    const row = screen.getByTestId(`os-attention-loop-request-${LOOP_REQUEST.id}`);
    expect(row).toHaveTextContent("publish");
    expect(row).toHaveTextContent("release-train — review");
    expect(row).toHaveTextContent("release");
    expect(within(row).getAllByRole("time")).toHaveLength(2);

    await user.click(row);
    expect(onSelect).toHaveBeenCalledWith(LOOP_REQUEST);
  });
});
