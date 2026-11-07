import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Topbar, TopbarSlotProvider, UIProvider } from "@compozy/ui";

import {
  filesAppPlaceStorageKey,
  filesAppPlaceStore,
  requestFilesAppPlace,
} from "../../hooks/use-files-app-place";
import { fileViewerTarget } from "../../lib/file-viewer-target";
import { FilesExplorer } from "../files-explorer";
import { filesAppProjectStatus, filesSwitcherProjects } from "../../lib/files-app-roots";
import { FilesAppWindow } from "../files-app-window";

// The highlight worker is a Vite `?worker` bundle that jsdom cannot construct: an inert worker
// stands in at that boundary, since no test here renders highlighted text.
vi.mock("@compozy/ui/lib/code-highlight-worker", () => ({
  createCodeHighlightWorker: () =>
    ({
      postMessage: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      terminate: () => {},
    }) as unknown as Worker,
}));

// Suite: Files OS app window (file-explorer US-002; UT-047).
// Invariant: the window never strands the operator: a gone worktree offers "Switch to project
// root", and archived projects leave the switcher while an open window says so.
// Owning layer: FilesAppWindow over the workspace, worktree and files adapters. Boundary: `fetch`.
const WORKSPACE = { id: "ws_1", name: "compozy", root_dir: "/r", archived_at: null };
const ARCHIVED = {
  id: "ws_old",
  name: "legacy",
  root_dir: "/o",
  archived_at: "2026-11-01T00:00:00Z",
};

function stubDaemon(options: { archivedIds?: string[] } = {}) {
  const calls: URL[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const request = input instanceof Request ? input : new Request(String(input));
      const url = new URL(request.url);
      calls.push(url);
      if (url.pathname === "/api/workspaces") {
        const archived = url.searchParams.get("archived") === "true";
        const active = [WORKSPACE].filter(w => !options.archivedIds?.includes(w.id));
        return Response.json({
          workspaces: archived
            ? [ARCHIVED, ...[WORKSPACE].filter(w => options.archivedIds?.includes(w.id))]
            : active,
        });
      }
      if (url.pathname.endsWith("/worktrees")) {
        return Response.json({ worktrees: [], discovered: [], repo: { git_backed: true } });
      }
      if (url.pathname.endsWith("/files/content")) {
        return Response.json({
          root: { kind: "workspace", workspace_id: "ws_1", name: "compozy" },
          path: url.searchParams.get("path"),
          kind: "text",
          size: 10,
          text: "# compozy\n",
          truncated: false,
        });
      }
      if (url.pathname.endsWith("/files/entries")) {
        if (url.pathname.includes("/sessions/")) {
          return Response.json({
            root: {
              kind: "worktree",
              workspace_id: "ws_1",
              worktree_id: "wt-1",
              name: "compozy",
              branch: "feat/x",
            },
            entries: [{ name: "README.md", path: "README.md", kind: "file", size: 10 }],
            truncated: false,
          });
        }
        if (url.searchParams.get("worktree_id") === "wt-gone") {
          return Response.json(
            { code: "root_unavailable", error: "this worktree is no longer on disk" },
            { status: 409 }
          );
        }
        return Response.json({
          root: { kind: "workspace", workspace_id: "ws_1", name: "compozy", branch: "main" },
          entries: [{ name: "README.md", path: "README.md", kind: "file", size: 10 }],
          truncated: false,
        });
      }
      return new Response("not found", { status: 404 });
    })
  );
  return calls;
}

function renderWindow() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UIProvider>
        <div style={{ height: 500 }}>
          <FilesAppWindow workspaceId="ws_1" onSwitchProject={() => {}} />
        </div>
      </UIProvider>
    </QueryClientProvider>
  );
}

describe("FilesAppWindow", () => {
  beforeEach(() => {
    window.localStorage.clear();
    filesAppPlaceStore.trigger.placed({
      workspaceId: "ws_1",
      place: { worktreeId: null, path: null },
    });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  // Operator review: the window and its tab read the open file, else the project, and the
  // project and root pickers sit in the head row instead of a row of their own.
  it("Should title the window with the open file, else the project, and keep the pickers in the head", async () => {
    stubDaemon();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const frame = (workspaceId: string) => (
      <QueryClientProvider client={client}>
        <UIProvider>
          <TopbarSlotProvider>
            <Topbar title="Files" />
            <div style={{ height: 500 }}>
              <FilesAppWindow
                key={workspaceId}
                workspaceId={workspaceId}
                onSwitchProject={() => {}}
              />
            </div>
          </TopbarSlotProvider>
        </UIProvider>
      </QueryClientProvider>
    );
    const view = render(frame("ws_1"));
    const title = () => screen.getByTestId("topbar-title-text");
    await waitFor(() => expect(title()).toHaveTextContent("compozy"));
    const head = view.container.querySelector('[data-slot="topbar-leading"]');
    expect(head?.querySelector('[data-testid="files-project-switcher"]')).not.toBeNull();
    expect(head?.querySelector('[data-testid="files-root-picker"]')).not.toBeNull();

    act(() => requestFilesAppPlace("ws_1", { worktreeId: null, path: "docs/README.md" }));
    await waitFor(() => expect(title()).toHaveTextContent("README.md"));
    expect(title()).not.toHaveTextContent("docs/");

    // A root switch keeps the open file, so the title stays on it.
    act(() =>
      filesAppPlaceStore.trigger.placed({
        workspaceId: "ws_1",
        place: { worktreeId: "wt-1", path: "docs/README.md" },
      })
    );
    await waitFor(() =>
      expect(screen.getByTestId("files-root-picker")).toHaveTextContent("Worktree unavailable")
    );
    expect(title()).toHaveTextContent("README.md");

    // Another project: its own place, so the title falls back to that project's name.
    view.rerender(frame("ws_old"));
    await waitFor(() => expect(title()).toHaveTextContent("legacy"));
  });

  it("Should offer Switch to project root when the worktree is gone and reload the checkout (UT-047)", async () => {
    filesAppPlaceStore.trigger.placed({
      workspaceId: "ws_1",
      place: { worktreeId: "wt-gone", path: null },
    });
    const calls = stubDaemon();
    renderWindow();
    fireEvent.click(await screen.findByRole("button", { name: "Switch to project root" }));
    await waitFor(() => expect(screen.getByTestId("files-root-name")).toHaveTextContent("compozy"));
    expect(
      calls.some(
        url => url.pathname.endsWith("/files/entries") && !url.searchParams.has("worktree_id")
      )
    ).toBe(true);
    expect(
      JSON.parse(window.localStorage.getItem(filesAppPlaceStorageKey("ws_1")) ?? "{}")
    ).toEqual({
      worktreeId: null,
      path: null,
    });
  });

  it("Should drop archived projects from the switcher and mark an archived window (UT-047)", async () => {
    expect(filesSwitcherProjects([WORKSPACE, ARCHIVED] as never).map(w => w.id)).toEqual(["ws_1"]);
    expect(
      filesAppProjectStatus("ws_old", {
        active: [WORKSPACE] as never,
        archived: [ARCHIVED] as never,
      })
    ).toBe("archived");
    stubDaemon({ archivedIds: ["ws_1"] });
    renderWindow();
    expect(await screen.findByText("Project archived")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByTestId("files-explorer")).toBeNull());
  });

  it("Should hand the session root and open path to Open in Files window (US-002.AC-6)", async () => {
    stubDaemon();
    const onOpenInFilesWindow = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <div style={{ height: 500 }}>
            <FilesExplorer
              root={{ kind: "session", workspaceId: "ws_1", sessionId: "sess-1" }}
              target={fileViewerTarget("README.md")}
              onOpenFile={() => {}}
              onOpenInFilesWindow={onOpenInFilesWindow}
            />
          </div>
        </UIProvider>
      </QueryClientProvider>
    );
    fireEvent.click(await screen.findByRole("button", { name: "Open in Files window" }));
    expect(onOpenInFilesWindow).toHaveBeenCalledWith(
      expect.objectContaining({ workspace_id: "ws_1", worktree_id: "wt-1" }),
      "README.md"
    );
  });

  it("Should re-root an open Files window and open the requested path (US-002.AC-6)", async () => {
    const calls = stubDaemon();
    renderWindow();
    await waitFor(() => expect(screen.getByTestId("files-root-name")).toHaveTextContent("compozy"));
    act(() => requestFilesAppPlace("ws_1", { worktreeId: "wt-1", path: "README.md" }));
    await waitFor(() =>
      expect(
        calls.some(
          url =>
            url.pathname.endsWith("/files/entries") &&
            url.searchParams.get("worktree_id") === "wt-1"
        )
      ).toBe(true)
    );
    await waitFor(() =>
      expect(screen.getByTestId("file-viewer-header")).toHaveTextContent("README.md")
    );
  });

  // US-001: a folder whose listing fails says so with Retry, instead of looking empty, and
  // asks again on Retry or on the next expansion (`retry: false` never would).
  it("Should show a failed folder with Retry and list it once the retry succeeds", async () => {
    let srcFails = true;
    const srcCalls: number[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const request = input instanceof Request ? input : new Request(String(input));
        const url = new URL(request.url);
        if (!url.pathname.endsWith("/files/entries")) return new Response("{}", { status: 404 });
        const root = { kind: "workspace", workspace_id: "ws_1", name: "compozy" };
        if (url.searchParams.get("path") === "src") {
          srcCalls.push(srcCalls.length);
          return srcFails
            ? Response.json({ code: "internal", error: "listing failed" }, { status: 500 })
            : Response.json({
                root,
                entries: [{ name: "main.go", path: "src/main.go", kind: "file", size: 1 }],
                truncated: false,
              });
        }
        return Response.json({
          root,
          entries: [{ name: "src", path: "src", kind: "directory" }],
          truncated: false,
        });
      })
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <div style={{ height: 500 }}>
            <FilesExplorer
              root={{ kind: "workspace", workspaceId: "ws_1" }}
              target={null}
              onOpenFile={() => {}}
            />
          </div>
        </UIProvider>
      </QueryClientProvider>
    );
    const treeRow = (path: string) =>
      container
        .querySelector("file-tree-container")
        ?.shadowRoot?.querySelector<HTMLElement>(`[data-type="item"][data-item-path="${path}"]`) ??
      null;
    await waitFor(() => expect(treeRow("src/")).not.toBeNull());

    act(() => treeRow("src/")!.click());
    expect(await screen.findByTestId("files-tree-folder-error")).toHaveTextContent(
      "src: Couldn't load this folder"
    );
    expect(srcCalls).toHaveLength(1);

    // Collapsing and re-expanding asks again.
    act(() => treeRow("src/")!.click());
    act(() => treeRow("src/")!.click());
    await waitFor(() => expect(srcCalls).toHaveLength(2));
    expect(await screen.findByTestId("files-tree-folder-error")).toBeInTheDocument();

    srcFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(treeRow("src/main.go")).not.toBeNull());
    expect(screen.queryByTestId("files-tree-folder-error")).toBeNull();
    expect(srcCalls).toHaveLength(3);
  });

  // Perf audit P-5 and the status-snapshot contract. Invariant: Refresh refetches only the
  // folders on screen, each with `Cache-Control: no-cache`; a collapsed folder issues no request
  // (it is only marked stale), and a plain expansion never bypasses the snapshot.
  it("Should refresh only shown folders, fresh, and leave collapsed ones idle", async () => {
    const requests: { path: string; fresh: boolean }[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const request = input instanceof Request ? input : new Request(String(input));
        const url = new URL(request.url);
        if (!url.pathname.endsWith("/files/entries")) return new Response("{}", { status: 404 });
        const path = url.searchParams.get("path") ?? "";
        requests.push({ path, fresh: request.headers.get("cache-control") === "no-cache" });
        const root = { kind: "workspace", workspace_id: "ws_1", name: "compozy" };
        const entries =
          path === ""
            ? [
                { name: "docs", path: "docs", kind: "directory" },
                { name: "src", path: "src", kind: "directory" },
              ]
            : [{ name: "a.txt", path: `${path}/a.txt`, kind: "file", size: 1 }];
        return Response.json({ root, entries, truncated: false });
      })
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <div style={{ height: 500 }}>
            <FilesExplorer
              root={{ kind: "workspace", workspaceId: "ws_1" }}
              target={null}
              onOpenFile={() => {}}
            />
          </div>
        </UIProvider>
      </QueryClientProvider>
    );
    const treeRow = (path: string) =>
      container
        .querySelector("file-tree-container")
        ?.shadowRoot?.querySelector<HTMLElement>(`[data-type="item"][data-item-path="${path}"]`) ??
      null;
    await waitFor(() => expect(treeRow("src/")).not.toBeNull());

    // docs stays open; src is expanded, loaded, then collapsed.
    act(() => treeRow("docs/")!.click());
    act(() => treeRow("src/")!.click());
    await waitFor(() => expect(treeRow("src/a.txt")).not.toBeNull());
    await waitFor(() => expect(treeRow("docs/a.txt")).not.toBeNull());
    act(() => treeRow("src/")!.click());
    expect(requests).toEqual([
      { path: "", fresh: false },
      { path: "docs", fresh: false },
      { path: "src", fresh: false },
    ]);

    requests.length = 0;
    fireEvent.click(screen.getByTestId("files-refresh"));
    await waitFor(() => expect(requests).toHaveLength(2));
    expect(requests).toEqual(
      expect.arrayContaining([
        { path: "", fresh: true },
        { path: "docs", fresh: true },
      ])
    );

    // Re-expanding the collapsed folder reads it again (it was marked stale), from the snapshot.
    act(() => treeRow("src/")!.click());
    await waitFor(() => expect(requests).toHaveLength(3));
    expect(requests[2]).toEqual({ path: "src", fresh: false });
  });

  // Greptile follow-up on revealKey. Invariant: choosing a folder in Go to file reveals it every
  // time, even when it is the same folder and the user collapsed it since.
  it("Should reveal the same Go to file folder again after the user collapsed it", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const request = input instanceof Request ? input : new Request(String(input));
        const url = new URL(request.url);
        const root = { kind: "workspace", workspace_id: "ws_1", name: "compozy" };
        if (url.pathname.endsWith("/files/find")) {
          return Response.json({
            root,
            query: url.searchParams.get("q"),
            results: [{ path: "web/src", kind: "directory", score: 10, match_ranges: [[4, 7]] }],
            indexed_files: 3,
            index_truncated: false,
          });
        }
        if (!url.pathname.endsWith("/files/entries")) return new Response("{}", { status: 404 });
        const path = url.searchParams.get("path") ?? "";
        const entries: Record<string, unknown[]> = {
          "": [{ name: "web", path: "web", kind: "directory" }],
          web: [{ name: "src", path: "web/src", kind: "directory" }],
          "web/src": [{ name: "main.tsx", path: "web/src/main.tsx", kind: "file", size: 1 }],
        };
        return Response.json({ root, entries: entries[path] ?? [], truncated: false });
      })
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={client}>
        <UIProvider>
          <div style={{ height: 500 }}>
            <FilesExplorer
              root={{ kind: "workspace", workspaceId: "ws_1" }}
              target={null}
              onOpenFile={() => {}}
            />
          </div>
        </UIProvider>
      </QueryClientProvider>
    );
    const treeRow = (path: string) =>
      container
        .querySelector("file-tree-container")
        ?.shadowRoot?.querySelector<HTMLElement>(`[data-type="item"][data-item-path="${path}"]`) ??
      null;
    const chooseFolder = async () => {
      fireEvent.click(screen.getByTestId("files-go-to-file"));
      fireEvent.change(await screen.findByTestId("go-to-file-input"), {
        target: { value: "src" },
      });
      const rows = await screen.findAllByTestId("go-to-file-row", {}, { timeout: 3000 });
      fireEvent.click(rows.find(row => row.dataset.path === "web/src")!);
      await waitFor(() => expect(screen.queryByTestId("go-to-file")).toBeNull());
    };
    await waitFor(() => expect(treeRow("web/")).not.toBeNull());

    await chooseFolder();
    await waitFor(() => expect(treeRow("web/src/main.tsx")).not.toBeNull());
    expect(treeRow("web/")).toHaveAttribute("aria-expanded", "true");

    act(() => treeRow("web/")!.click());
    await waitFor(() => expect(treeRow("web/")).toHaveAttribute("aria-expanded", "false"));

    await chooseFolder();
    await waitFor(() => expect(treeRow("web/")).toHaveAttribute("aria-expanded", "true"));
    await waitFor(() => expect(treeRow("web/src/")).toHaveAttribute("aria-selected", "true"));
    expect(treeRow("web/src/main.tsx")).not.toBeNull();
  });
});
