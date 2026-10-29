// Suite: terminal aggregate location
// Invariant: the All projects Terminal window is the launcher — new-terminal destinations stay live
// while the catalog loads, the remembered project is preselected so one Enter opens it, a project
// whose folder is gone stays listed but unavailable, an all-profiles read offers no destination,
// and only running terminals are listed.
// Boundary IN: TerminalAggregateLocation's launcher states.
// Boundary OUT: catalog streaming and window retargeting (use-terminal-window-controller-state.test.ts).
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TERMINAL_FIXTURES } from "@/systems/terminal/mocks/terminal-fixtures";

import { TerminalAggregateLocation } from "../terminal-aggregate-location";

const WORKSPACES = [
  { id: "ws-atlas", name: "atlas" },
  { id: "ws-beacon", name: "beacon" },
];

function renderLocation(
  overrides: Partial<Parameters<typeof TerminalAggregateLocation>[0]> = {},
  catalog: Partial<Parameters<typeof TerminalAggregateLocation>[0]["catalog"]> = {}
) {
  const props = {
    catalog: { data: undefined, error: null, isPending: true, refetch: vi.fn(), ...catalog },
    coordinator: { userRetarget: vi.fn() },
    profile: "default",
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
  it("Should offer every destination while the catalog loads, the remembered one selected", () => {
    renderLocation({ rememberedWorkspaceId: "ws-beacon" });

    expect(screen.getByTestId("terminal-launcher-new-ws-atlas")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-launcher-new-@user")).toHaveTextContent("User");
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-selected",
      "true"
    );
    expect(screen.queryByText("Running")).not.toBeInTheDocument();
  });

  it("Should show a project whose folder is gone as unavailable and select the next one", () => {
    renderLocation({
      rememberedWorkspaceId: "ws-atlas",
      workspaces: [{ ...WORKSPACES[0], root_missing: true }, WORKSPACES[1]],
    });

    const missing = screen.getByTestId("terminal-launcher-new-ws-atlas");
    expect(missing).toHaveAttribute("aria-disabled", "true");
    expect(missing).toHaveTextContent("Folder not found");
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-selected",
      "true"
    );
  });

  it("Should hold the loading shape while a requested terminal waits for its project", () => {
    renderLocation({ requestedId: "term-unknown" });

    expect(screen.getByRole("status")).toHaveTextContent("Loading terminals");
    expect(screen.queryByTestId("terminal-launcher")).not.toBeInTheDocument();
  });

  it("Should offer Retry when a requested terminal's catalog fails to load", () => {
    const { props } = renderLocation(
      { requestedId: "term-unknown" },
      { error: new Error("daemon unavailable"), isPending: false }
    );

    expect(screen.getByText("Couldn't load Terminal")).toBeInTheDocument();
    screen.getByRole("button", { name: "Retry" }).click();
    expect(props.catalog.refetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("terminal-launcher")).not.toBeInTheDocument();
  });

  it("Should leave out new-terminal destinations and name each owner for an all-profiles read", () => {
    renderLocation({ readOnly: true }, { data: [...TERMINAL_FIXTURES], isPending: false });

    expect(screen.queryByText("New terminal")).not.toBeInTheDocument();
    const running = TERMINAL_FIXTURES.find(terminal => terminal.state === "running");
    expect(screen.getByTestId(`terminal-launcher-running-${running?.id}`)).toHaveTextContent(
      running?.profile_name ?? ""
    );
  });

  it("Should list only the running terminals once the catalog arrives", () => {
    const terminals = [...TERMINAL_FIXTURES];
    renderLocation({}, { data: terminals, isPending: false });

    for (const terminal of terminals) {
      const row = screen.queryByTestId(`terminal-launcher-running-${terminal.id}`);
      if (terminal.state === "running") expect(row).toBeInTheDocument();
      else expect(row).not.toBeInTheDocument();
    }
  });
});
