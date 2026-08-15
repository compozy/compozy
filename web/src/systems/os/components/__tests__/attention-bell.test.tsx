// Suite: OS attention bell rows
// Invariant: loop-node rows render only when supplied, and selecting one
// hands the deep-link state (`waiting` | `attention`) to the host.
// Boundary IN: AttentionBell props and row union.
// Boundary OUT: menubar navigation (`/loop-runs?nodes=`).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Popover } from "@compozy/ui";

import type { OsAttentionRow, OsLoopNodeAttentionRow } from "../../lib/attention-model";
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

function renderBell(rows: readonly OsAttentionRow[], onSelect = vi.fn()) {
  render(
    <Popover open>
      <AttentionBell
        loading={false}
        onSelect={onSelect}
        rows={rows}
        sessionsDisconnected={false}
        tasksDisconnected={false}
      />
    </Popover>
  );
  return onSelect;
}

describe("AttentionBell", () => {
  it("Should hide loop-node rows when the presence probes are empty", () => {
    renderBell([]);
    expect(screen.queryByTestId("os-attention-loop-node-waiting")).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-attention-loop-node-attention")).not.toBeInTheDocument();
    expect(screen.getByTestId("os-bell-empty")).toBeInTheDocument();
  });

  it("Should render probed loop-node rows and deep-link through onSelect", async () => {
    const user = userEvent.setup();
    const onSelect = renderBell([WAITING, ATTENTION]);

    expect(screen.getByTestId("os-attention-loop-node-waiting")).toBeInTheDocument();
    expect(screen.getByTestId("os-attention-loop-node-attention")).toBeInTheDocument();

    await user.click(screen.getByTestId("os-attention-loop-node-waiting"));
    expect(onSelect).toHaveBeenCalledWith(WAITING);

    await user.click(screen.getByTestId("os-attention-loop-node-attention"));
    expect(onSelect).toHaveBeenCalledWith(ATTENTION);
  });
});
