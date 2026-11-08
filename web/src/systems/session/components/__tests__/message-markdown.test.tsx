import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UIProvider, toast } from "@compozy/ui";

const navigate = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => navigate,
}));

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import { filesAppPlace, filesAppPlaceStore } from "@/systems/files/hooks/use-files-app-place";
import { OsShellContext, type OsShellHandle } from "@/systems/os";
import type { CompozyShellBridge, OpenInEditorResult } from "@/systems/os/lib/desktop-shell-bridge";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";

import { sessionReviewPanelStore } from "../../hooks/use-session-review-panel-state";
import { sessionKeys } from "../../lib/query-keys";
import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
import type { SessionPayload } from "../../types";
import { MessageMarkdown } from "../message-markdown";

describe("MessageMarkdown", () => {
  it("Should keep an unfinished streaming image pending until its authored path arrives", () => {
    const { container, rerender } = render(<MessageMarkdown streaming content="![Preview](" />);
    expect(container).not.toHaveTextContent("streamdown:incomplete-image");
    expect(container.querySelector("img")).toBeNull();
    rerender(<MessageMarkdown content="![Preview](/tmp/demo/complete.png)" />);
    expect(screen.getByText("/tmp/demo/complete.png")).toBeInTheDocument();
  });

  it("Should preserve authored image references without requesting filesystem paths from the app", () => {
    const { container, rerender } = render(
      <MessageMarkdown content="![Preview](/tmp/demo/screen.png)" />
    );
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByText("/tmp/demo/screen.png")).toBeInTheDocument();
    rerender(<MessageMarkdown reveal content="![Preview](/tmp/demo/screen.png)" />);
    expect(container.querySelector("img")).toBeNull();
    expect(screen.getByText("/tmp/demo/screen.png")).toBeInTheDocument();
  });

  it("renders fenced code through the shared CodeBlock primitive", async () => {
    const { container } = render(
      <MessageMarkdown content={"```typescript\nconst value = 1;\n```"} />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveAttribute("data-language", "typescript");

    await waitFor(
      () => {
        expect(codeBlock).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
    await waitFor(() => {
      expect(codeBlock).toHaveAttribute("data-theme", "vitesse-light");
    });

    expect(screen.getByRole("button", { name: "Copy to clipboard" })).toBeInTheDocument();
  });

  it("renders fenced code without a language as a plain shared CodeBlock", () => {
    const { container } = render(
      <MessageMarkdown content={["```", "compozy start", "```"].join("\n")} />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveAttribute("data-highlight-state", "plain");
    expect(codeBlock).not.toHaveAttribute("data-language");
    expect(codeBlock?.textContent).toContain("compozy start");
  });

  it("ends a streaming Mermaid code fence before subsequent Markdown prose", () => {
    const { container } = render(
      <MessageMarkdown
        streaming
        content={[
          "```mermaid",
          "flowchart LR",
          "  A --> B",
          "```",
          "",
          "## After",
          "",
          "Plain text.",
        ].join("\n")}
      />
    );

    const codeBlocks = container.querySelectorAll<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlocks).toHaveLength(1);
    expect(codeBlocks[0]).toHaveTextContent("flowchart LR");
    expect(codeBlocks[0]).not.toHaveTextContent("After");
    expect(screen.getByRole("heading", { name: "After" })).toBeInTheDocument();
    expect(screen.getByText("Plain text.")).toBeInTheDocument();
  });

  it("keeps inline code as compact inline prose", () => {
    const { container } = render(
      <MessageMarkdown content={"Use `compozy start` from the shell."} />
    );

    expect(container.querySelector('[data-slot="code-block"]')).toBeNull();
    expect(container.querySelector("code")?.textContent).toBe("compozy start");
  });

  it("forwards compact to the prose root on both render paths", () => {
    const direct = render(<MessageMarkdown content="# Plan" compact />);
    expect(direct.container.querySelector('[data-slot="stream-markdown"]')).toHaveAttribute(
      "data-compact",
      "true"
    );
    direct.unmount();

    const revealed = render(<MessageMarkdown content="# Plan" compact reveal />);
    expect(revealed.container.querySelector('[data-slot="stream-markdown"]')).toHaveAttribute(
      "data-compact",
      "true"
    );
  });

  it("renders incomplete fenced code while streaming", async () => {
    const { container } = render(
      <MessageMarkdown content={"Plan:\n\n```ts\nconst value = 1"} streaming />
    );

    const codeBlock = container.querySelector<HTMLElement>('[data-slot="code-block"]');
    expect(codeBlock).toBeInTheDocument();
    expect(codeBlock).toHaveTextContent("const value = 1");
    await waitFor(
      () => {
        expect(codeBlock).toHaveAttribute("data-highlight-state", "highlighted");
      },
      { timeout: 5_000 }
    );
  });

  it("renders GFM tables without raw HTML elements", () => {
    const { container } = render(
      <MessageMarkdown
        content={[
          "| Area | Status |",
          "| --- | --- |",
          "| Stream | Fixed |",
          "",
          "<script>alert('xss')</script>",
        ].join("\n")}
      />
    );

    expect(container.querySelector("table")).toBeInTheDocument();
    expect(screen.getByText("Stream")).toBeInTheDocument();
    expect(screen.getByText("Fixed")).toBeInTheDocument();
    expect(container.querySelector("script")).toBeNull();
  });
});

// Suite: transcript file paths (spec #1167 S5; UT-040..UT-046, UT-048; file-explorer US-005).
// Invariant: this session's assistant markdown turns path inline code and path links into links:
// a plain click opens the file in the project's Files window (as a tab of the focused window) on
// every client with a session root, and on the desktop app Cmd/Ctrl+click calls the shell bridge
// with the click-time resolved path and inside roots. Without either action (user scope on the
// web, other sessions' prose) paths render as before. Owning layer: MessageMarkdown and the
// session path-link hooks. Boundary: `window.compozyShell` (stubbed), the window manager's open
// (`coordinator.userOpen`, stubbed) and `fetch` (stubbed for the detail reread).
const WS = "ws_paths";
const SID = "sess-paths";

function stubShell(platform = "darwin", result: OpenInEditorResult = { status: "opened" }) {
  const openInEditor = vi.fn(async () => result);
  window.compozyShell = { platform, files: { openInEditor } } as unknown as CompozyShellBridge;
  return openInEditor;
}

function seededClient(
  session: Partial<SessionPayload>,
  worktrees: { id: string; path: string }[] = []
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(sessionKeys.detail(WS, SID), { id: SID, workspace_id: WS, ...session });
  client.setQueryData(workspaceKeys.detail(WS), { workspace: { id: WS, root_dir: "/repo" } });
  client.setQueryData(workspaceKeys.worktrees(WS), { worktrees });
  return client;
}

const userOpen = vi.fn(async () => undefined);
const shellHandle = {
  coordinator: { userOpen },
} as unknown as OsShellHandle;

function renderTranscript(
  ui: ReactNode,
  client = seededClient({ working_dir: "/repo" }),
  workspaceId = WS
) {
  return render(
    <QueryClientProvider client={client}>
      <UIProvider>
        <OsShellContext value={shellHandle}>
          <SessionRuntimeRenderProvider sessionId={SID} workspaceId={workspaceId}>
            {ui}
          </SessionRuntimeRenderProvider>
        </OsShellContext>
      </UIProvider>
    </QueryClientProvider>
  );
}

/** What the last plain click asked the project's Files window to show. */
function viewerTarget() {
  return filesAppPlaceStore.getSnapshot().context.targets[WS] ?? null;
}

function blockedSpans(container: HTMLElement) {
  return Array.from(
    container.querySelectorAll("span[title^='Blocked URL']"),
    node => node.outerHTML
  );
}

describe("MessageMarkdown file paths", () => {
  afterEach(() => {
    delete window.compozyShell;
    navigate.mockReset();
    userOpen.mockClear();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("Should Cmd-click directly into the last-used target without opening the Files window", async () => {
    stubShell("darwin");
    const openWith = vi.fn(async () => ({ status: "opened" as const }));
    window.compozyShell!.files!.openWith = openWith;
    localStorage.setItem("compozy.open-target", "cursor");
    renderTranscript(<MessageMarkdown openPaths content="See `src/a.ts:12:3`." />);
    const previous = viewerTarget();
    fireEvent.click(screen.getByText("src/a.ts:12:3"), { metaKey: true });
    await waitFor(() =>
      expect(openWith).toHaveBeenCalledWith({
        target: "cursor",
        path: "/repo/src/a.ts",
        line: 12,
        column: 3,
        roots: ["/repo"],
      })
    );
    expect(viewerTarget()).toEqual(previous);
    localStorage.removeItem("compozy.open-target");
  });

  // US-005.AC-5: the tooltip names the target the modifier click opens in.
  it("Should name the last-used Open in target in the path tooltip", async () => {
    stubShell("darwin");
    window.compozyShell!.files!.listOpenTargets = async () => [
      { id: "cursor", label: "Cursor", kind: "editor", supportsLine: true },
    ];
    localStorage.setItem("compozy.open-target", "cursor");
    renderTranscript(<MessageMarkdown openPaths content="See `src/a.ts:12:3`." />);
    fireEvent.focus(screen.getByRole("button", { name: "Open a.ts in Files" }));
    expect(await screen.findByText(/opens in Cursor/)).toBeInTheDocument();
    localStorage.removeItem("compozy.open-target");
  });

  it("Should open an inline path on Cmd+click with the resolved path and roots (UT-040)", async () => {
    const openInEditor = stubShell("darwin");
    renderTranscript(<MessageMarkdown openPaths content="Changed `internal/a.go:42` here." />);
    expect(screen.getByRole("button", { name: "Open a.go in Files" })).toBeInTheDocument();
    expect(openInEditor).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("internal/a.go:42"), { ctrlKey: true });
    expect(openInEditor).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("internal/a.go:42"), { metaKey: true });
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(1));
    expect(openInEditor).toHaveBeenCalledWith({
      path: "/repo/internal/a.go",
      line: 42,
      roots: ["/repo"],
    });
  });

  it("Should open on Ctrl+click off macOS", async () => {
    const openInEditor = stubShell("linux");
    renderTranscript(<MessageMarkdown openPaths content="See `internal/a.go:42`." />);
    fireEvent.click(screen.getByText("internal/a.go:42"), { metaKey: true });
    expect(openInEditor).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("internal/a.go:42"), { ctrlKey: true });
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(1));
  });

  it("Should send the bound worktree and the workspace root as roots (UT-041)", async () => {
    const openInEditor = stubShell();
    renderTranscript(
      <MessageMarkdown openPaths content="See `internal/a.go`." />,
      seededClient({ working_dir: "/wt/web", worktree_id: "wt-1" }, [{ id: "wt-1", path: "/wt" }])
    );
    fireEvent.click(screen.getByText("internal/a.go"), { metaKey: true });
    await waitFor(() =>
      expect(openInEditor).toHaveBeenCalledWith({
        path: "/wt/web/internal/a.go",
        roots: ["/wt", "/repo"],
      })
    );

    // A plain click opens the Files window on the bound worktree, not the checkout.
    fireEvent.click(screen.getByText("internal/a.go"));
    await waitFor(() =>
      expect(filesAppPlace(WS)).toEqual({ worktreeId: "wt-1", path: "web/internal/a.go" })
    );
  });

  // Invariant: a session bound to a worktree opens the Files window on that worktree even when the
  // worktree listing can't name its path; it never silently falls back to the project checkout.
  it("Should keep the bound worktree when its listing entry is missing", async () => {
    stubShell();
    renderTranscript(
      <MessageMarkdown openPaths content="See `internal/a.go:3`." />,
      seededClient({ working_dir: "/repo", worktree_id: "wt-gone" })
    );
    fireEvent.click(screen.getByText("internal/a.go:3"));
    await waitFor(() => expect(userOpen).toHaveBeenCalled());
    expect(filesAppPlace(WS).worktreeId).toBe("wt-gone");
  });

  // US-005.AC-1/AC-2 (file-explorer). A plain click opens the Files window on every client, including
  // the browser without a desktop bridge.
  it("Should open the Files window at the path and line on a plain click (UT-042)", async () => {
    const openInEditor = stubShell();
    renderTranscript(<MessageMarkdown openPaths content="See `internal/a.go:42`." />);
    const before = viewerTarget()?.nonce;
    fireEvent.click(screen.getByText("internal/a.go:42"));
    await waitFor(() => expect(viewerTarget()?.nonce).not.toBe(before));
    expect(viewerTarget()).toMatchObject({ kind: "file", path: "internal/a.go", line: 42 });
    // The project's Files window opens on the session root; the Review panel stays as it was.
    expect(filesAppPlace(WS)).toEqual({ worktreeId: null, path: "internal/a.go" });
    expect(userOpen).toHaveBeenLastCalledWith({
      app: "files",
      instanceKey: WS,
      route: { pathname: `/files/${WS}`, search: {} },
    });
    expect(sessionReviewPanelStore.getSnapshot().context.bySession[SID]?.open).not.toBe(true);
    expect(openInEditor).not.toHaveBeenCalled();

    delete window.compozyShell;
    renderTranscript(<MessageMarkdown openPaths content="See `/tmp/report.md`." />);
    fireEvent.click(screen.getByRole("button", { name: "Open report.md in Files" }));
    await waitFor(() =>
      expect(viewerTarget()).toMatchObject({ kind: "outside", path: "/tmp/report.md" })
    );
    for (const modifier of [{ metaKey: true }, { ctrlKey: true }]) {
      const previous = viewerTarget()?.nonce;
      fireEvent.click(screen.getByRole("button", { name: "Open report.md in Files" }), modifier);
      await waitFor(() => expect(viewerTarget()?.nonce).not.toBe(previous));
      expect(viewerTarget()).toMatchObject({ kind: "outside", path: "/tmp/report.md" });
    }
  });

  it("Should reach the path link by keyboard and open on Enter and Space (UT-043)", async () => {
    stubShell();
    const user = userEvent.setup();
    renderTranscript(<MessageMarkdown openPaths content="See `internal/a.go:42`." />);
    const link = screen.getByRole("button", { name: "Open a.go in Files" });
    await user.tab();
    expect(link).toHaveFocus();
    const before = viewerTarget()?.nonce;
    await user.keyboard("{Enter}");
    await waitFor(() => expect(viewerTarget()?.nonce).not.toBe(before));
    expect(viewerTarget()).toMatchObject({ path: "internal/a.go", line: 42 });
    const first = viewerTarget()?.nonce;
    await user.keyboard(" ");
    await waitFor(() => expect(viewerTarget()?.nonce).not.toBe(first));
  });

  // UT-054 (file-explorer). Invariant: a user-scope session has no root, so on the web its
  // transcript paths stay plain code; on desktop only the modifier click remains.
  it("Should render plain code outside assistant prose and for a user-scope session (UT-044, UT-054)", async () => {
    const content = "See `internal/a.go:42` and [x](internal/a.go).";
    const plain = render(<MessageMarkdown content={content} />);
    const plainHtml = plain.container.innerHTML;
    plain.unmount();

    const userScope = renderTranscript(
      <MessageMarkdown openPaths content={content} />,
      seededClient({ working_dir: "/home/d" }),
      ""
    );
    expect(userScope.container.querySelector("[data-slot='stream-markdown']")?.outerHTML).toBe(
      new DOMParser().parseFromString(plainHtml, "text/html").body.firstElementChild?.outerHTML
    );
    expect(screen.queryByRole("button")).toBeNull();
    expect(userScope.container).toHaveTextContent("x [blocked]");
    userScope.unmount();

    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({}, { status: 404 }))
    );
    const openInEditor = stubShell();
    const desktopUserScope = renderTranscript(
      <MessageMarkdown openPaths content="See `/tmp/a.md`." />,
      seededClient({ working_dir: "/home/d" }),
      ""
    );
    expect(screen.queryByRole("button")).toBeNull();
    fireEvent.click(screen.getByText("/tmp/a.md"), { metaKey: true });
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(1));
    desktopUserScope.unmount();

    const userTurn = renderTranscript(<MessageMarkdown authored content={content} />);
    expect(screen.queryByRole("button", { name: /in Files/ })).toBeNull();
    expect(userTurn.container).toHaveTextContent("x [blocked]");
  });

  it("Should open path links without navigating and keep other links as before (UT-045)", async () => {
    const content =
      "[x](internal/a.go:42) [y](/tmp/a.md) [z](https://a.dev) [w](notapath) [v](file://evil.host/x)";
    const browser = render(<MessageMarkdown content={content} />);
    const browserAnchor = browser.container.querySelector("a[href='https://a.dev/']")?.outerHTML;
    const browserBlocked = blockedSpans(browser.container);
    browser.unmount();

    const openInEditor = stubShell();
    const { container } = renderTranscript(<MessageMarkdown openPaths content={content} />);
    expect(screen.getByRole("button", { name: "Open a.go in Files" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open a.md in Files" })).toBeInTheDocument();
    expect(container.querySelector("a[href='https://a.dev/']")?.outerHTML).toBe(browserAnchor);
    expect(blockedSpans(container)).toEqual(browserBlocked.slice(-2));
    expect(container.querySelector("a[href^='/tmp']")).toBeNull();

    fireEvent.click(screen.getByText("x"));
    fireEvent.click(screen.getByText("y"));
    expect(openInEditor).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("x"), { metaKey: true });
    await waitFor(() =>
      expect(openInEditor).toHaveBeenCalledWith({
        path: "/repo/internal/a.go",
        line: 42,
        roots: ["/repo"],
      })
    );
    fireEvent.click(screen.getByText("y"), { metaKey: true });
    await waitFor(() =>
      expect(openInEditor).toHaveBeenLastCalledWith({ path: "/tmp/a.md", roots: ["/repo"] })
    );
  });

  it("Should map bridge results to toasts with an Open Settings recovery (UT-046)", async () => {
    const error = vi.spyOn(toast, "error");
    const cases: [OpenInEditorResult, string | null, boolean][] = [
      [{ status: "error", code: "not_found", message: "x" }, "File not found: /repo/a.go", false],
      [{ status: "error", code: "not_a_file", message: "x" }, "Not a file: /repo/a.go", false],
      [
        {
          status: "error",
          code: "editor_failed",
          message: "Couldn't open the editor. Check the editor command in Settings.",
        },
        "Couldn't open the editor. Check the editor command in Settings.",
        true,
      ],
      [{ status: "opened" }, null, false],
      [{ status: "cancelled" }, null, false],
    ];
    for (const [result, message, withAction] of cases) {
      error.mockClear();
      stubShell("darwin", result);
      const view = renderTranscript(<MessageMarkdown openPaths content="See `a.go:1`." />);
      fireEvent.click(screen.getByText("a.go:1"), { metaKey: true });
      await waitFor(() => expect(window.compozyShell?.files?.openInEditor).toHaveBeenCalled());
      await waitFor(() => (message === null ? undefined : expect(error).toHaveBeenCalled()));
      if (message === null) {
        expect(error).not.toHaveBeenCalled();
      } else if (withAction) {
        expect(error).toHaveBeenCalledWith(message, {
          action: { label: "Open Settings", onClick: expect.any(Function) },
        });
        const options = error.mock.calls[0]?.[1] as { action: { onClick: () => void } };
        options.action.onClick();
        expect(navigate).toHaveBeenCalledWith({ to: "/settings/general" });
      } else {
        expect(error).toHaveBeenCalledWith(message);
      }
      view.unmount();
    }
  });

  it("Should reread the detail when the cached one lacks working_dir (UT-048)", async () => {
    const fetchMock = vi.fn(async () =>
      Response.json({ session: { id: SID, workspace_id: WS, working_dir: "/repo/web" } })
    );
    vi.stubGlobal("fetch", fetchMock);
    const openInEditor = stubShell();
    const stale = renderTranscript(
      <MessageMarkdown openPaths content="See `src/app.tsx`." />,
      seededClient({ name: "renamed" })
    );
    fireEvent.click(screen.getByText("src/app.tsx"), { metaKey: true });
    await waitFor(() =>
      expect(openInEditor).toHaveBeenCalledWith({ path: "/repo/web/src/app.tsx", roots: ["/repo"] })
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
    stale.unmount();

    fetchMock.mockClear();
    renderTranscript(<MessageMarkdown openPaths content="See `src/app.tsx`." />);
    fireEvent.click(screen.getByText("src/app.tsx"), { metaKey: true });
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(2));
    expect(openInEditor).toHaveBeenLastCalledWith({ path: "/repo/src/app.tsx", roots: ["/repo"] });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("Should reread the detail when every cached entry with working_dir is invalidated (UT-048)", async () => {
    const fetchMock = vi.fn(async () =>
      Response.json({ session: { id: SID, workspace_id: WS, working_dir: "/repo/wt" } })
    );
    vi.stubGlobal("fetch", fetchMock);
    const openInEditor = stubShell();
    const client = seededClient({ working_dir: "/repo/old" });
    client.setQueryData(sessionKeys.byId(SID, "default"), {
      id: SID,
      workspace_id: WS,
      working_dir: "/repo/old",
    });
    await client.invalidateQueries({ queryKey: sessionKeys.byIdRoot(SID), refetchType: "none" });
    await client.invalidateQueries({
      queryKey: sessionKeys.detail(WS, SID),
      exact: true,
      refetchType: "none",
    });
    renderTranscript(<MessageMarkdown openPaths content="See `src/app.tsx`." />, client);
    fireEvent.click(screen.getByText("src/app.tsx"), { metaKey: true });
    await waitFor(() =>
      expect(openInEditor).toHaveBeenCalledWith({ path: "/repo/wt/src/app.tsx", roots: ["/repo"] })
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("Should leave inline code inside a link undecorated and the link's own clicks intact", async () => {
    const openInEditor = stubShell();
    const { container } = renderTranscript(
      <MessageMarkdown
        openPaths
        content="[`src/app.ts`](https://github.com/o/r/blob/main/src/app.ts)"
      />
    );
    const anchor = container.querySelector("a[href='https://github.com/o/r/blob/main/src/app.ts']");
    expect(anchor).not.toBeNull();
    expect(anchor?.querySelector("button, [data-file-path]")).toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
    // fireEvent returns false only when a handler called preventDefault.
    expect(fireEvent.click(screen.getByText("src/app.ts"), { metaKey: true })).toBe(true);
    expect(openInEditor).not.toHaveBeenCalled();

    const local = renderTranscript(<MessageMarkdown openPaths content="[`a.ts:3`](a.ts:3)" />);
    expect(within(local.container).getAllByRole("button", { name: /in Files/ })).toHaveLength(1);
    expect(local.container.querySelectorAll("[data-file-path]")).toHaveLength(1);
    fireEvent.click(within(local.container).getByText("a.ts:3"), { metaKey: true });
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(1));
    expect(openInEditor).toHaveBeenCalledWith({ path: "/repo/a.ts", line: 3, roots: ["/repo"] });
  });

  it("Should linkify Windows drive paths only on a Windows desktop", () => {
    const content = "See `C:\\Users\\a\\b.txt` and `C:/Users/a/b.txt`.";
    for (const platform of ["darwin", "linux"]) {
      stubShell(platform);
      const view = renderTranscript(<MessageMarkdown openPaths content={content} />);
      expect(within(view.container).queryByRole("button")).toBeNull();
      view.unmount();
    }
    stubShell("win32");
    const win = renderTranscript(<MessageMarkdown openPaths content={content} />);
    expect(
      within(win.container).getAllByRole("button", { name: "Open b.txt in Files" })
    ).toHaveLength(2);
  });

  it("Should leave non-path inline code unchanged on desktop", () => {
    stubShell();
    renderTranscript(<MessageMarkdown openPaths content="Run `compozy start` now." />);
    expect(within(document.body).queryByRole("button")).toBeNull();
  });
});
