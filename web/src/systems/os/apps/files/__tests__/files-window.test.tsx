import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Suite: the Files app window controller (file-explorer US-002, EC-1).
// Invariant: a bound window shows its project; an unbound window binds itself to the remembered
// (else first) project through the window manager, and offers Add project when none exists.
// Owning layer: FilesWindow. Boundaries: the desktop store, the shell coordinator, the
// workspace catalog, and the Files app body (stubbed).
const state = {
  route: { pathname: "/files", search: {} },
  catalog: {
    pending: false,
    rememberedWorkspace: null as { id: string } | null,
    workspaces: [] as { id: string }[],
  },
};
const userRetarget = vi.fn(async () => undefined);
const useActiveWorkspace = vi.fn((_options?: { enabled?: boolean }) => state.catalog);

vi.mock("../../../hooks/use-desktop", () => ({
  useDesktop: (selector: (desktop: unknown) => unknown) =>
    selector({ windows: { "win-1": { route: state.route } } }),
}));
vi.mock("../../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userRetarget } }),
}));
vi.mock("../../../hooks/use-workspace-setup-defaults", () => ({
  useWorkspaceSetupDefaults: () => ({}),
}));
vi.mock("../../../components/desktop-shell-dialogs", () => ({
  WorkspaceSetupDialogBoundary: () => null,
}));
vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: (options?: { enabled?: boolean }) => useActiveWorkspace(options),
}));
vi.mock("@/systems/files", () => ({
  FilesAppWindow: ({ workspaceId }: { workspaceId: string }) => (
    <p data-testid="files-app">{workspaceId}</p>
  ),
  FilesAppNoProjects: () => <p data-testid="files-no-projects" />,
}));

import { FilesWindow } from "../files-window";

describe("FilesWindow", () => {
  beforeEach(() => {
    userRetarget.mockClear();
    useActiveWorkspace.mockClear();
    state.route = { pathname: "/files", search: {} };
    state.catalog = { pending: false, rememberedWorkspace: null, workspaces: [] };
  });

  it("Should show a bound window's project without reading the catalog", () => {
    state.route = { pathname: "/files/ws_2", search: {} };
    render(<FilesWindow windowId="win-1" />);
    expect(screen.getByTestId("files-app")).toHaveTextContent("ws_2");
    expect(useActiveWorkspace).toHaveBeenLastCalledWith({ enabled: false });
    expect(userRetarget).not.toHaveBeenCalled();
  });

  it("Should bind an unbound window to the remembered project, else the first", () => {
    state.catalog.workspaces = [{ id: "ws_1" }, { id: "ws_2" }];
    state.catalog.rememberedWorkspace = { id: "ws_2" };
    const view = render(<FilesWindow windowId="win-1" />);
    expect(screen.getByTestId("files-app-binding")).toBeInTheDocument();
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith("win-1", {
      app: "files",
      instanceKey: "ws_2",
      route: { pathname: "/files/ws_2", search: {} },
    });
    view.unmount();

    userRetarget.mockClear();
    state.catalog.rememberedWorkspace = null;
    render(<FilesWindow windowId="win-1" />);
    expect(userRetarget).toHaveBeenCalledExactlyOnceWith(
      "win-1",
      expect.objectContaining({ instanceKey: "ws_1" })
    );
  });

  it("Should offer Add project when no project exists (US-002.EC-1)", () => {
    render(<FilesWindow windowId="win-1" />);
    expect(screen.getByTestId("files-no-projects")).toBeInTheDocument();
    expect(userRetarget).not.toHaveBeenCalled();
  });
});
