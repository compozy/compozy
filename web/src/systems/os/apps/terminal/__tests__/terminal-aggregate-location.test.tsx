// Suite: terminal aggregate location
// Invariant: while the all-projects catalog has no answer the window holds one named busy region in
// the list's own frame, and the Project filter and the new-terminal row stay live; a refetch with
// terminals in hand keeps the list.
// Boundary IN: TerminalAggregateLocation's loading branch and its list.
// Boundary OUT: catalog streaming and window retargeting (use-terminal-window-controller-state.test.ts).
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TERMINAL_FIXTURES } from "@/systems/terminal/mocks/terminal-fixtures";
import { projectFilterField } from "@/systems/workspace";

import { TerminalAggregateLocation } from "../terminal-aggregate-location";

const WORKSPACES = [{ id: "ws-atlas", name: "atlas" }];

function renderLocation(
  overrides: Partial<Parameters<typeof TerminalAggregateLocation>[0]> = {},
  catalog: Partial<Parameters<typeof TerminalAggregateLocation>[0]["catalog"]> = {}
) {
  const props = {
    catalog: { data: undefined, error: null, isPending: true, refetch: vi.fn(), ...catalog },
    coordinator: { userRetarget: vi.fn() },
    profile: "default",
    projectFilter: { field: projectFilterField(WORKSPACES), values: [], setValues: vi.fn() },
    readOnly: false,
    requestedId: null,
    viewer: { id: "viewer", attachmentToken: "token" },
    windowId: "win-1",
    workspaces: WORKSPACES,
    ...overrides,
  };
  return { ...render(<TerminalAggregateLocation {...props} />), props };
}

describe("TerminalAggregateLocation", () => {
  it("Should wait in one busy region with the filter and the new-terminal row live", () => {
    renderLocation();

    expect(screen.getByRole("status")).toHaveTextContent("Loading terminals");
    expect(screen.getByRole("button", { name: "Filter terminals" })).toBeEnabled();
    expect(screen.getByTestId("terminal-aggregate-new")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-aggregate-empty")).not.toBeInTheDocument();
  });

  it("Should leave out the new-terminal row for an all-profiles read", () => {
    renderLocation({ readOnly: true });

    expect(screen.getByRole("status")).toHaveTextContent("Loading terminals");
    expect(screen.queryByTestId("terminal-aggregate-new")).not.toBeInTheDocument();
  });

  it("Should keep the list through a refetch and drop the loading shape once rows arrive", () => {
    const { rerender, props } = renderLocation();

    const terminals = [...TERMINAL_FIXTURES];
    rerender(
      <TerminalAggregateLocation
        {...props}
        catalog={{ ...props.catalog, data: terminals, isPending: false }}
      />
    );

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByTestId("terminal-aggregate-list").children).toHaveLength(terminals.length);
  });
});
