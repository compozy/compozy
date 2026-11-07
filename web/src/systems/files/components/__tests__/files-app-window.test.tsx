import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { filesAppPlaceStorageKey, filesAppPlaceStore } from "../../hooks/use-files-app-place";
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
      if (url.pathname.endsWith("/files/entries")) {
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
});
