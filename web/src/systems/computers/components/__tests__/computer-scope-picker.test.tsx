// Suite: ComputerScopePicker (T-F8)
// Invariant: Hidden when no remote computers are enabled (N6); reads facet counts as props;
//            marks an active one-computer scope with border-brand-dim (R21); supports searching
//            at scale (C6) and selecting All computers, This computer, or any remote computer.
// Owning layer: component (systems/computers/components)
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  LOCAL_COMPUTER_FIXTURE,
  makeComputerFixture,
} from "../../mocks/remote-computers-fixtures";
import { ComputerScopePicker, type ComputerScopeCount } from "../computer-scope-picker";

const DEVBOX = makeComputerFixture({ id: "cmp_devbox", name: "devbox", status: "connected" });
const WORKSTATION = makeComputerFixture({
  id: "cmp_ws",
  name: "workstation",
  status: "offline",
  lastConnectedLabel: "2h ago",
});

describe("ComputerScopePicker (T-F8)", () => {
  it("Should render nothing when no remote computer is enabled (N6)", () => {
    const { container } = render(
      <ComputerScopePicker
        computers={[]}
        countsByComputer={new Map()}
        local={LOCAL_COMPUTER_FIXTURE}
        onChange={vi.fn()}
        value="all"
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("Should display facet counts, active-scope border, offline indicator, and emit selection", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const counts = new Map<string, ComputerScopeCount>([
      ["local", { count: 4, needsYou: 1 }],
      ["cmp_devbox", { count: 7, needsYou: 2 }],
      ["cmp_ws", { count: null, needsYou: 0 }],
    ]);

    const { rerender } = render(
      <ComputerScopePicker
        computers={[DEVBOX, WORKSTATION]}
        countsByComputer={counts}
        defaultOpen
        local={LOCAL_COMPUTER_FIXTURE}
        onChange={onChange}
        value="all"
      />
    );

    const trigger = screen.getByTestId("computer-scope-trigger");
    expect(trigger).toHaveTextContent("All computers");
    expect(trigger.className).not.toContain("border-brand-dim");

    // Aggregate count sums answered computers only (4 + 7 = 11); unknown stays absent.
    expect(screen.getByTestId("computer-scope-item-all")).toHaveTextContent("11");
    expect(screen.getByTestId("computer-scope-item-local")).toHaveTextContent("4");
    expect(screen.getByTestId("computer-scope-item-cmp_devbox")).toHaveTextContent("7");

    await user.click(screen.getByTestId("computer-scope-item-cmp_devbox"));
    expect(onChange).toHaveBeenCalledWith("cmp_devbox");

    // Re-render scoped to an offline computer: active border + Offline label (`R21`, `S8`).
    rerender(
      <ComputerScopePicker
        computers={[DEVBOX, WORKSTATION]}
        countsByComputer={counts}
        local={LOCAL_COMPUTER_FIXTURE}
        onChange={onChange}
        value="cmp_ws"
      />
    );
    const scopedTrigger = screen.getByTestId("computer-scope-trigger");
    expect(scopedTrigger.className).toContain("border-brand-dim");
    expect(scopedTrigger).toHaveTextContent("workstation");
    expect(scopedTrigger).toHaveTextContent("· Offline");
  });
});
