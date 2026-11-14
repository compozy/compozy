// Suite: terminal aggregate location
// Invariant: the All projects Terminal window is the launcher — new-terminal destinations stay live
// while the catalog loads, the remembered project is preselected so one Enter opens it, a project
// whose folder is gone stays listed but unavailable, an all-profiles read offers no destination,
// only running terminals are listed, and only ready worktrees of the acting profile are offered,
// folded under their project until the pointer or ←/→ unfolds it or a search matches one, each
// opening its terminal in that worktree. A requested terminal the settled catalog does not list
// reads as gone, with the launcher one secondary action away.
// Boundary IN: TerminalAggregateLocation's launcher states.
// Boundary OUT: catalog streaming and window retargeting (use-terminal-window-controller-state.test.ts).
import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { createTerminal, terminalKeys } from "@/systems/terminal";
import { TERMINAL_FIXTURES } from "@/systems/terminal/mocks/terminal-fixtures";

import { terminalWindowCreateKey } from "../../../lib/terminal-window-close";
import { TerminalAggregateLocation } from "../terminal-aggregate-location";

vi.mock("@/systems/terminal", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/terminal")>()),
  createTerminal: vi.fn(),
}));

const WORKTREE = {
  id: "wt-feature",
  workspace_id: "ws-atlas",
  name: "feature-login",
  branch: "feat/login",
  state: "ready",
  profile_name: "default",
};

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
  const client = new QueryClient();
  const view = render(<TerminalAggregateLocation {...props} />, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
  return { ...view, props, client };
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

  it("Should follow a default that settles late but keep the operator's search", () => {
    const { rerender, props } = renderLocation({ workspaces: [] });
    expect(screen.getByTestId("terminal-launcher-new-@user")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    rerender(
      <TerminalAggregateLocation
        {...props}
        rememberedWorkspaceId="ws-beacon"
        workspaces={WORKSPACES}
      />
    );
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "atl" },
    });
    rerender(
      <TerminalAggregateLocation
        {...props}
        rememberedWorkspaceId="ws-beacon"
        workspaces={[...WORKSPACES]}
      />
    );
    expect(screen.getByPlaceholderText("Open a terminal in…")).toHaveValue("atl");
    expect(screen.getByTestId("terminal-launcher-new-ws-atlas")).toHaveAttribute(
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

  it("Should say a requested terminal is gone and keep the launcher one action away", () => {
    // #1712: an exited, reaped terminal opened from a transcript must not silently
    // become the new-terminal launcher.
    const { props } = renderLocation(
      { requestedId: "term-reaped" },
      { data: [...TERMINAL_FIXTURES], isPending: false }
    );

    expect(screen.getByTestId("terminal-not-found")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-launcher")).not.toBeInTheDocument();
    expect(props.coordinator.userRetarget).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Open a new terminal" }));
    expect(screen.getByTestId("terminal-launcher")).toBeInTheDocument();
  });

  it("Should offer no new-terminal action for a gone terminal in an all-profiles read", () => {
    renderLocation(
      { readOnly: true, requestedId: "term-reaped" },
      { data: [...TERMINAL_FIXTURES], isPending: false }
    );

    expect(screen.getByTestId("terminal-not-found")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Open a new terminal" })).not.toBeInTheDocument();
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

  it("Should fold the acting profile's ready worktrees under their project until unfolded", () => {
    renderLocation({
      worktrees: [
        WORKTREE,
        { ...WORKTREE, id: "wt-pending", name: "pending", state: "pending" },
        { ...WORKTREE, id: "wt-other", name: "other-profile", profile_name: "work" },
        { ...WORKTREE, id: "wt-gone", workspace_id: "ws-beacon", state: "removed" },
      ],
    });

    const project = screen.getByTestId("terminal-launcher-new-ws-atlas");
    expect(project).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).not.toHaveAttribute(
      "aria-expanded"
    );
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Show 1 worktree of atlas" }));

    expect(project).toHaveAttribute("aria-expanded", "true");
    const row = screen.getByTestId("terminal-launcher-new-ws-atlas/wt-feature");
    expect(row).toHaveTextContent("feature-login");
    expect(row.closest("[data-slot='tree-branch']")).toHaveAttribute(
      "id",
      project.getAttribute("aria-controls")
    );
    const rows = screen.getAllByTestId(/^terminal-launcher-new-/).map(item => item.dataset.testid);
    expect(rows).toEqual([
      "terminal-launcher-new-ws-atlas",
      "terminal-launcher-new-ws-atlas/wt-feature",
      "terminal-launcher-new-ws-beacon",
      "terminal-launcher-new-@user",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Hide 1 worktree of atlas" }));
    expect(project).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeNull();
  });

  it("Should unfold and fold the selected project with the arrow keys, climbing from a worktree", () => {
    renderLocation({ worktrees: [WORKTREE, { ...WORKTREE, id: "wt-fix", name: "fix-retry" }] });
    const input = screen.getByPlaceholderText("Open a terminal in…");
    const project = screen.getByTestId("terminal-launcher-new-ws-atlas");
    expect(project).toHaveAttribute("aria-selected", "true");

    fireEvent.keyDown(input, { key: "ArrowRight" });
    expect(project).toHaveAttribute("aria-expanded", "true");
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(screen.getByTestId("terminal-launcher-new-ws-atlas/wt-fix")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    fireEvent.keyDown(input, { key: "ArrowLeft" });
    expect(project).toHaveAttribute("aria-selected", "true");
    expect(project).toHaveAttribute("aria-expanded", "true");
    fireEvent.keyDown(input, { key: "ArrowLeft" });
    expect(project).toHaveAttribute("aria-expanded", "false");
    expect(project).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-fix")).toBeNull();
  });

  it("Should match worktrees in a search and unfold only the projects holding a match", async () => {
    renderLocation({
      worktrees: [
        WORKTREE,
        { ...WORKTREE, id: "wt-fix", name: "fix-retry" },
        { ...WORKTREE, id: "wt-beacon", workspace_id: "ws-beacon", name: "beacon-docs" },
      ],
    });

    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "login" },
    });

    const project = screen.getByTestId("terminal-launcher-new-ws-atlas");
    expect(project).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-fix")).toBeNull();
    expect(screen.queryByTestId("terminal-launcher-new-ws-beacon")).toBeNull();
    expect(screen.queryByTestId("terminal-launcher-new-@user")).toBeNull();
    // The project is context for its match, not a result: Enter opens the worktree searched for.
    expect(project).toHaveAttribute("aria-disabled", "true");
    await waitFor(() =>
      expect(screen.getByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toHaveAttribute(
        "aria-selected",
        "true"
      )
    );

    // The context row stays unselectable, but its fold control still answers the pointer.
    const user = userEvent.setup();
    const fold = screen.getByTestId("terminal-launcher-fold-ws-atlas");
    expect(fold.className).toContain("pointer-events-auto");
    await user.click(fold);
    expect(project).toHaveAttribute("aria-expanded", "false");
    await user.click(fold);
    expect(project).toHaveAttribute("aria-expanded", "true");
    expect(createTerminal).not.toHaveBeenCalled();

    // Words combine across a worktree's name and its project's.
    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "login atlas" },
    });
    expect(screen.getByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-fix")).toBeNull();

    // A project matched only by name stays folded.
    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "atlas" },
    });
    expect(project).toHaveAttribute("aria-expanded", "false");
    expect(project).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "docs" },
    });
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-expanded",
      "true"
    );
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon/wt-beacon")).toBeInTheDocument();

    // While searching, the arrows keep the caret: the fold control is reached with Tab instead,
    // and its Enter folds without opening a terminal.
    screen.getByPlaceholderText("Open a terminal in…").focus();
    await user.tab();
    expect(screen.getByTestId("terminal-launcher-fold-ws-beacon")).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(screen.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-expanded",
      "false"
    );
    expect(createTerminal).not.toHaveBeenCalled();
    expect(screen.getByPlaceholderText("Open a terminal in…")).toHaveValue("docs");

    // Clearing the search folds every project the search unfolded.
    fireEvent.change(screen.getByPlaceholderText("Open a terminal in…"), {
      target: { value: "" },
    });
    for (const id of ["ws-atlas", "ws-beacon"]) {
      expect(screen.getByTestId(`terminal-launcher-new-${id}`)).toHaveAttribute(
        "aria-expanded",
        "false"
      );
    }
    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeNull();
  });

  it("Should leave out the worktrees of a project whose folder is gone", () => {
    renderLocation({
      workspaces: [{ ...WORKSPACES[0], root_missing: true }, WORKSPACES[1]],
      worktrees: [WORKTREE],
    });

    expect(screen.queryByTestId("terminal-launcher-new-ws-atlas/wt-feature")).toBeNull();
  });

  it("Should show a created terminal, already in its cached catalog, while refreshes stall", async () => {
    // #1712 review: refreshing the lists must not gate showing the new terminal, nor keep
    // its creation pending — the close guard holds the window while it is.
    const terminal = { ...TERMINAL_FIXTURES[0], id: "term-new", workspace_id: "ws-atlas" };
    vi.mocked(createTerminal).mockResolvedValueOnce(terminal);
    const { props, client } = renderLocation({}, { data: [], isPending: false });
    const stalled = new QueryObserver(client, {
      queryKey: terminalKeys.aggregate("", "default"),
      queryFn: () => new Promise<never>(() => {}),
      initialData: [],
      staleTime: Number.POSITIVE_INFINITY,
    });
    const unsubscribe = stalled.subscribe(() => {});
    const ownerCatalog = terminalKeys.catalog({ workspaceId: "ws-atlas", profileKey: "default" });
    client.setQueryData(ownerCatalog, []);

    fireEvent.click(screen.getByTestId("terminal-launcher-new-ws-atlas"));

    await waitFor(() =>
      expect(props.coordinator.userRetarget).toHaveBeenCalledWith(
        "win-1",
        expect.objectContaining({ instanceKey: "term-new" })
      )
    );
    await waitFor(() =>
      expect(client.isMutating({ mutationKey: terminalWindowCreateKey("win-1") })).toBe(0)
    );
    expect(client.getQueryState(terminalKeys.aggregate("", "default"))?.fetchStatus).toBe(
      "fetching"
    );
    // The bound window's cached catalog already names it, before any read lands.
    expect(client.getQueryData(ownerCatalog)).toEqual([terminal]);
    unsubscribe();
  });

  it("Should open a worktree row's terminal in that worktree and bind the window to its project", async () => {
    const terminal = { ...TERMINAL_FIXTURES[0], id: "term-wt", workspace_id: "ws-atlas" };
    vi.mocked(createTerminal).mockResolvedValueOnce(terminal);
    const { props } = renderLocation({ worktrees: [WORKTREE] });

    fireEvent.click(screen.getByTestId("terminal-launcher-fold-ws-atlas"));
    expect(createTerminal).not.toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("terminal-launcher-new-ws-atlas/wt-feature"));

    expect(createTerminal).toHaveBeenCalledWith(
      "ws-atlas",
      { worktree: "wt-feature" },
      { profile: "default" },
      props.viewer
    );
    await waitFor(() =>
      expect(props.coordinator.userRetarget).toHaveBeenCalledWith(
        "win-1",
        expect.objectContaining({ instanceKey: "term-wt" })
      )
    );
  });
});
