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

import type { CompozyShellBridge, OpenInEditorResult } from "@/systems/os/lib/desktop-shell-bridge";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";

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

// Suite: transcript file paths (spec #1167 S5; UT-040..UT-046, UT-048).
// Invariant: on the desktop app only, this session's assistant markdown turns path inline code
// and path links into Cmd/Ctrl+click and button affordances that call the shell bridge with the
// click-time resolved path and inside roots; every other client renders exactly as before.
// Owning layer: MessageMarkdown and the session open-in-editor hook. Boundary: `window.compozyShell`
// (stubbed) and `fetch` (stubbed for the detail reread).
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

function renderTranscript(ui: ReactNode, client = seededClient({ working_dir: "/repo" })) {
  return render(
    <QueryClientProvider client={client}>
      <UIProvider>
        <SessionRuntimeRenderProvider sessionId={SID} workspaceId={WS}>
          {ui}
        </SessionRuntimeRenderProvider>
      </UIProvider>
    </QueryClientProvider>
  );
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
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("Should open an inline path on Cmd+click with the resolved path and roots (UT-040)", async () => {
    const openInEditor = stubShell("darwin");
    renderTranscript(<MessageMarkdown openPaths content="Changed `internal/a.go:42` here." />);
    expect(screen.getByRole("button", { name: "Open a.go in editor" })).toBeInTheDocument();
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
  });

  it("Should keep a plain click on the path inert (UT-042)", () => {
    const openInEditor = stubShell();
    renderTranscript(<MessageMarkdown openPaths content="See `internal/a.go:42`." />);
    fireEvent.click(screen.getByText("internal/a.go:42"));
    expect(openInEditor).not.toHaveBeenCalled();
  });

  it("Should reach the open button by keyboard and open on Enter and Space (UT-043)", async () => {
    const openInEditor = stubShell();
    const user = userEvent.setup();
    renderTranscript(<MessageMarkdown openPaths content="See `internal/a.go:42`." />);
    const button = screen.getByRole("button", { name: "Open a.go in editor" });
    await user.tab();
    expect(button).toHaveFocus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(button).toBeEnabled());
    await user.keyboard(" ");
    await waitFor(() => expect(openInEditor).toHaveBeenCalledTimes(2));
  });

  it("Should render exactly as today without the desktop bridge or outside assistant prose (UT-044)", () => {
    const content = "See `internal/a.go:42` and [x](internal/a.go).";
    const browser = render(<MessageMarkdown content={content} />);
    const browserHtml = browser.container.innerHTML;
    browser.unmount();

    const remote = renderTranscript(<MessageMarkdown openPaths content={content} />);
    expect(remote.container.querySelector("[data-slot='stream-markdown']")?.outerHTML).toBe(
      new DOMParser().parseFromString(browserHtml, "text/html").body.firstElementChild?.outerHTML
    );
    expect(screen.queryByRole("button")).toBeNull();
    expect(remote.container).toHaveTextContent("x [blocked]");
    remote.unmount();

    stubShell();
    const userTurn = renderTranscript(<MessageMarkdown authored content={content} />);
    expect(screen.queryByRole("button", { name: /in editor/ })).toBeNull();
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
    expect(screen.getByRole("button", { name: "Open a.go in editor" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open a.md in editor" })).toBeInTheDocument();
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
      fireEvent.click(screen.getByRole("button", { name: "Open a.go in editor" }));
      await waitFor(() => expect(window.compozyShell?.files?.openInEditor).toHaveBeenCalled());
      await waitFor(() => expect(screen.getByRole("button", { name: /in editor/ })).toBeEnabled());
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
    expect(within(local.container).getAllByRole("button", { name: /in editor/ })).toHaveLength(1);
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
      within(win.container).getAllByRole("button", { name: "Open b.txt in editor" })
    ).toHaveLength(2);
  });

  it("Should leave non-path inline code unchanged on desktop", () => {
    stubShell();
    renderTranscript(<MessageMarkdown openPaths content="Run `compozy start` now." />);
    expect(within(document.body).queryByRole("button")).toBeNull();
  });
});
