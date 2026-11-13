import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DropdownMenu, DropdownMenuContent, FileTree, UIProvider } from "@compozy/ui";

import type { CompozyShellBridge, OpenTarget } from "@/systems/os";
import { workspaceDetailOptions } from "@/systems/workspace";

import type { FilesRootRef } from "../../adapters/files-api";
import { FilesTreeRowMenu } from "../files-tree-row-menu";
import { OpenInButton } from "../open-in-button";
import { FileMenuItems } from "../open-in-menu-items";

const navigate = vi.fn();
const notifyUser = vi.fn();
vi.mock("@/lib/user-feedback", () => ({ notifyUser: (...args: unknown[]) => notifyUser(...args) }));
vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));

// Suite: the shared file menu (file-explorer US-006, `_uiux.md` S3/S5, `app-menus` File · Open in).
// Invariant: one tree on every file surface — the configured editor (else the first detected one)
// and the reveal target at the top, the other editors with Refresh list and Editor settings
// behind Open in, then Copy path; directories open without a line; a vanished editor is named and
// the list re-detected; a browser keeps only the surface's own verbs. Boundary: the desktop bridge
// (stubbed).
const ROOT: FilesRootRef = { kind: "workspace", workspaceId: "ws_1" };
const CURSOR: OpenTarget = { id: "cursor", label: "Cursor", kind: "editor", supportsLine: true };
const FINDER: OpenTarget = { id: "reveal", label: "Finder", kind: "reveal", supportsLine: false };

function client() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(workspaceDetailOptions("ws_1").queryKey, {
    workspace: {
      id: "ws_1",
      root_dir: "/repo",
      name: "Project",
      add_dirs: [],
      created_at: "2026-11-07T00:00:00Z",
      updated_at: "2026-11-07T00:00:00Z",
    },
  });
  return queryClient;
}

function renderWith(ui: ReactNode) {
  return render(
    <QueryClientProvider client={client()}>
      <UIProvider>{ui}</UIProvider>
    </QueryClientProvider>
  );
}

function bridge(targets: () => OpenTarget[], openWith = vi.fn(async () => ({ status: "opened" }))) {
  const listOpenTargets = vi.fn(async () => targets());
  window.compozyShell = {
    platform: "darwin",
    files: { openInEditor: vi.fn(), openWith, listOpenTargets },
  } as unknown as CompozyShellBridge;
  return { listOpenTargets, openWith };
}

/** The item's label without decorative (aria-hidden) icons such as a logo's SVG title. */
function visibleLabel(item: HTMLElement): string {
  const copy = item.cloneNode(true) as HTMLElement;
  for (const hidden of copy.querySelectorAll('[aria-hidden="true"]')) hidden.remove();
  return copy.textContent ?? "";
}

async function openMenu() {
  fireEvent.click(await screen.findByRole("button", { name: "Choose application" }));
  return await screen.findByRole("menu");
}

/** Opens `Open in ▸` and returns its submenu. */
async function openSubmenu(menu: HTMLElement) {
  await userEvent.click(within(menu).getByRole("menuitem", { name: "Open in" }));
  const refresh = await screen.findByRole("menuitem", { name: "Refresh list" });
  return refresh.closest<HTMLElement>('[role="menu"]')!;
}

function labels(menu: HTMLElement): string[] {
  return within(menu)
    .getAllByRole("menuitem")
    .map(item => visibleLabel(item));
}

describe("OpenInButton", () => {
  beforeEach(() => localStorage.removeItem("compozy.open-target"));
  afterEach(() => {
    delete window.compozyShell;
    navigate.mockReset();
    localStorage.removeItem("compozy.open-target");
  });

  it("Should lead with the configured editor and Finder, the rest behind Open in (US-006.AC-2)", async () => {
    const configured: OpenTarget = { ...CURSOR, id: "configured", label: "Configured editor" };
    const { openWith } = bridge(() => [configured, CURSOR, FINDER]);
    renderWith(<OpenInButton root={ROOT} path="src/a.ts" line={4} />);
    const menu = await openMenu();
    expect(labels(menu)).toEqual([
      "Open in Configured editor",
      "Open in",
      "Reveal in Finder",
      "Copy path",
    ]);
    expect(within(menu).getAllByRole("separator")).toHaveLength(1);
    const submenu = await openSubmenu(menu);
    expect(labels(submenu)).toEqual(["Cursor", "Refresh list", "Editor settings…"]);
    await userEvent.click(within(submenu).getByRole("menuitem", { name: "Cursor" }));
    await waitFor(() =>
      expect(openWith).toHaveBeenCalledWith({
        target: "cursor",
        path: "/repo/src/a.ts",
        line: 4,
        column: 1,
        roots: ["/repo", "/repo"],
      })
    );
  });

  it("Should open Editor settings from Open in", async () => {
    bridge(() => [CURSOR, FINDER]);
    renderWith(<OpenInButton root={ROOT} path="src/a.ts" />);
    const submenu = await openSubmenu(await openMenu());
    expect(labels(submenu)).toEqual(["Refresh list", "Editor settings…"]);
    await userEvent.click(within(submenu).getByRole("menuitem", { name: "Editor settings…" }));
    expect(navigate).toHaveBeenCalledWith({ to: "/settings/general" });
  });

  it("Should lead with the first detected editor while no editor command is set (US-006.AC-1)", async () => {
    bridge(() => [CURSOR, FINDER]);
    renderWith(<OpenInButton root={ROOT} path="src/a.ts" />);
    expect(await screen.findByRole("button", { name: "Open in Cursor" })).toBeInTheDocument();
  });

  it("Should offer Set an editor in Settings with no editor and reveal from the primary (US-006.AC-5)", async () => {
    const { openWith } = bridge(() => [FINDER]);
    renderWith(<OpenInButton root={ROOT} path="src/a.ts" line={9} />);
    const menu = await openMenu();
    expect(within(menu).getByRole("menuitem", { name: "Set an editor in Settings" })).toBeVisible();
    expect(within(menu).queryByText("Configured editor")).toBeNull();
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Set an editor in Settings" }));
    expect(navigate).toHaveBeenCalledWith({ to: "/settings/general" });

    fireEvent.click(await screen.findByRole("button", { name: "Reveal in Finder" }));
    await waitFor(() =>
      expect(openWith).toHaveBeenCalledWith({
        target: "reveal",
        path: "/repo/src/a.ts",
        roots: ["/repo", "/repo"],
      })
    );
  });

  it("Should re-detect the list on Refresh list and after an editor vanished (US-006.EC-1)", async () => {
    let targets = [CURSOR, FINDER];
    const openWith = vi.fn(async () => ({
      status: "error",
      code: "target_unavailable",
      message: "Cursor is no longer available.",
    }));
    const { listOpenTargets } = bridge(() => targets, openWith);
    renderWith(<OpenInButton root={ROOT} path="src/a.ts" />);
    const submenu = await openSubmenu(await openMenu());
    await userEvent.click(within(submenu).getByRole("menuitem", { name: "Refresh list" }));
    await waitFor(() => expect(listOpenTargets).toHaveBeenCalledTimes(2));

    targets = [FINDER];
    fireEvent.click(await screen.findByRole("button", { name: "Open in Cursor" }));
    await waitFor(() =>
      expect(notifyUser).toHaveBeenCalledWith({
        tone: "error",
        message: "Cursor is no longer available.",
      })
    );
    expect(await screen.findByRole("button", { name: "Reveal in Finder" })).toBeInTheDocument();
    expect(listOpenTargets).toHaveBeenCalledTimes(3);
  });
});

describe("FilesTreeRowMenu", () => {
  afterEach(() => {
    delete window.compozyShell;
  });

  function treeRow(container: HTMLElement, path: string): HTMLElement {
    const element = container
      .querySelector("file-tree-container")
      ?.shadowRoot?.querySelector<HTMLElement>(`[data-type="item"][data-item-path="${path}"]`);
    if (!element) throw new Error(`row ${path} is not rendered`);
    return element;
  }

  function renderTree() {
    return renderWith(
      <div style={{ height: 300 }}>
        <FileTree
          aria-label="Project files"
          paths={["web/", "README.md"]}
          rowMenu={item => <FilesTreeRowMenu root={ROOT} item={item} />}
        />
      </div>
    );
  }

  it("Should open a directory without a line from its row menu (US-006.AC-4)", async () => {
    const { openWith } = bridge(() => [CURSOR, FINDER]);
    const { container } = renderTree();
    await waitFor(() => treeRow(container, "web/"));
    fireEvent.contextMenu(treeRow(container, "web/"));
    const menu = await screen.findByRole("menu", { name: "Actions for web" });
    expect(await within(menu).findByRole("menuitem", { name: "Reveal in Finder" })).toBeVisible();
    expect(labels(menu)).toEqual(["Open in Cursor", "Open in", "Reveal in Finder", "Copy path"]);
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Open in Cursor" }));
    await waitFor(() =>
      expect(openWith).toHaveBeenCalledWith({
        target: "cursor",
        path: "/repo/web",
        roots: ["/repo", "/repo"],
      })
    );
  });

  it("Should keep only Copy path in the browser (US-006.AC-6)", async () => {
    const { container } = renderTree();
    await waitFor(() => treeRow(container, "README.md"));
    fireEvent.contextMenu(treeRow(container, "README.md"));
    const menu = await screen.findByRole("menu", { name: "Actions for README.md" });
    expect(labels(menu)).toEqual(["Copy path"]);
  });
});

describe("FileMenuItems", () => {
  afterEach(() => {
    delete window.compozyShell;
  });

  function renderMenu(onOpenInFiles: () => void) {
    renderWith(
      <DropdownMenu open>
        <DropdownMenuContent aria-label="File">
          <FileMenuItems
            host="dropdown"
            root={ROOT}
            request={{ path: "src/a.ts", line: 3 }}
            copyPath="src/a.ts"
            leading={[
              { kind: "item", id: "files", label: "Open in Files", onSelect: onOpenInFiles },
            ]}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    );
    return screen.findByRole("menu", { name: "File" });
  }

  it("Should keep a surface's own verb ahead of the shared tree (Changes: Open in Files)", async () => {
    bridge(() => [CURSOR, FINDER]);
    const onOpenInFiles = vi.fn();
    const menu = await renderMenu(onOpenInFiles);
    await within(menu).findByRole("menuitem", { name: "Open in Cursor" });
    expect(labels(menu)).toEqual([
      "Open in Files",
      "Open in Cursor",
      "Open in",
      "Reveal in Finder",
      "Copy path",
    ]);
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Open in Files" }));
    expect(onOpenInFiles).toHaveBeenCalledOnce();
  });

  it("Should keep only the surface's verbs and Copy path in the browser", async () => {
    const menu = await renderMenu(vi.fn());
    expect(labels(menu)).toEqual(["Open in Files", "Copy path"]);
  });
});
