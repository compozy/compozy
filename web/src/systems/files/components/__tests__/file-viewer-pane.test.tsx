import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { workspaceDetailOptions } from "@/systems/workspace";
import type { CompozyShellBridge } from "@/systems/os";

import type { FileSearchFile, FilesRootRef } from "../../adapters/files-api";
import {
  fileViewerTarget,
  outsideViewerTarget,
  type FileViewerTarget,
} from "../../lib/file-viewer-target";
import { FileViewerPane } from "../file-viewer-pane";
import { FilesSearchResults } from "../files-search-results";
import { OpenInButton } from "../open-in-button";

vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
}));

// Suite: file viewer states (file-explorer US-003, US-004; UT-055).
// Invariant: every file kind the daemon reports renders an explicit state — never garbage or a
// silent blank — and sensitive content is requested with `reveal=true` only after Show contents.
// Owning layer: FileViewerPane over the files adapter. Boundary: `fetch` (stubbed).
const ROOT: FilesRootRef = { kind: "session", workspaceId: "ws_1", sessionId: "sess-1" };
const ROOT_PAYLOAD = { kind: "workspace", workspace_id: "ws_1", name: "compozy" };

function frame(client: QueryClient, ui: ReactNode) {
  return (
    <QueryClientProvider client={client}>
      <UIProvider>
        <div style={{ height: 400 }}>{ui}</div>
      </UIProvider>
    </QueryClientProvider>
  );
}

function renderPane(ui: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(frame(client, ui));
  return { ...view, rerenderPane: (next: ReactNode) => view.rerender(frame(client, next)) };
}

function stubContent(handler: (url: URL) => Response) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const request = input instanceof Request ? input : new Request(String(input));
    return handler(new URL(request.url));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function content(body: Record<string, unknown>) {
  return Response.json({ root: ROOT_PAYLOAD, truncated: false, ...body });
}

describe("FileViewerPane", () => {
  afterEach(() => {
    delete window.compozyShell;
    vi.unstubAllGlobals();
  });

  it("Should ask for a file when nothing is selected and keep an outside path as written", () => {
    const view = renderPane(<FileViewerPane root={ROOT} target={null} />);
    expect(screen.getByText("Select a file")).toBeInTheDocument();
    view.unmount();
    renderPane(<FileViewerPane root={ROOT} target={outsideViewerTarget("/tmp/report.md")} />);
    expect(screen.getByText("Outside this project")).toBeInTheDocument();
    expect(screen.getByText("/tmp/report.md")).toBeInTheDocument();
  });

  it("Should gate a sensitive file until Show contents and reveal only on request", async () => {
    const fetchMock = stubContent(url =>
      url.searchParams.get("reveal") === "true"
        ? content({ path: ".env", kind: "text", size: 12, text: "TOKEN=secret\n", sensitive: true })
        : content({ path: ".env", kind: "sensitive", size: 12, sensitive: true })
    );
    const view = renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget(".env")} />);
    expect(await screen.findByText("Sensitive file")).toBeInTheDocument();
    const urls = () => fetchMock.mock.calls.map(([input]) => String((input as Request).url));
    expect(urls().some(url => url.includes("reveal"))).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Show contents" }));
    await waitFor(() => expect(screen.queryByText("Sensitive file")).toBeNull());
    await waitFor(() => expect(screen.getByTestId("file-viewer-text")).toBeInTheDocument());
    expect(urls().filter(url => url.includes("reveal=true"))).toHaveLength(1);
    // Reopening the file brings the gate back (US-004.AC-4).
    view.rerenderPane(<FileViewerPane root={ROOT} target={fileViewerTarget(".env")} />);
    expect(await screen.findByText("Sensitive file")).toBeInTheDocument();
  });

  it.each([
    [{ kind: "binary", size: 2_400_000 }, "Binary file", "2.3 MB · No preview"],
    [
      { kind: "image", size: 12 * 1024 * 1024, truncated: true },
      "Image too large to preview",
      "12 MB",
    ],
  ])("Should name a non-text file explicitly (%j)", async (body, title, detail) => {
    stubContent(() => content({ path: "a.bin", ...body }));
    renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("a.bin")} />);
    const state = (await screen.findByText(title)).closest("[data-slot='empty']");
    expect(state).toHaveTextContent(detail);
  });

  it("Should show the truncation banner for text over the read cap", async () => {
    stubContent(() =>
      content({
        path: "big.log",
        kind: "text",
        size: 14.2 * 1024 * 1024,
        text: "x\n",
        truncated: true,
      })
    );
    renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("big.log")} />);
    expect(await screen.findByTestId("file-viewer-truncated")).toHaveTextContent(
      "Showing the first 1 MB of 14.2 MB"
    );
  });

  it("Should report a missing file with the path and a tree refresh", async () => {
    stubContent(() =>
      Response.json({ error: "file not found", code: "file_not_found" }, { status: 404 })
    );
    const onRefreshTree = vi.fn();
    renderPane(
      <FileViewerPane
        root={ROOT}
        target={fileViewerTarget("gone.go")}
        onRefreshTree={onRefreshTree}
      />
    );
    expect(await screen.findByText("File not found")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Refresh tree" }));
    expect(onRefreshTree).toHaveBeenCalledTimes(1);
  });

  // US-003.EC-2/EC-3: a refresh never leaves old text on screen as if it were current.
  it("Should replace the text with File not found when a refresh finds the file deleted", async () => {
    let deleted = false;
    stubContent(() =>
      deleted
        ? Response.json({ error: "file not found", code: "file_not_found" }, { status: 404 })
        : content({ path: "a.ts", kind: "text", size: 14, text: "const old = 1;" })
    );
    renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("a.ts")} />);
    expect(await screen.findByTestId("file-viewer-focus")).toBeInTheDocument();
    deleted = true;
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(await screen.findByText("File not found")).toBeInTheDocument();
    expect(screen.queryByTestId("file-viewer-focus")).toBeNull();
  });

  it("Should keep the text under a stale notice when a refresh fails transiently", async () => {
    let failing = false;
    stubContent(() =>
      failing
        ? Response.json({ error: "busy", code: "internal" }, { status: 500 })
        : content({ path: "a.ts", kind: "text", size: 14, text: "const old = 1;" })
    );
    renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("a.ts")} />);
    expect(await screen.findByTestId("file-viewer-focus")).toBeInTheDocument();
    failing = true;
    fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(await screen.findByTestId("file-viewer-stale")).toHaveTextContent(
      "Couldn't refresh this file. Showing the last loaded version."
    );
    expect(screen.getByTestId("file-viewer-focus")).toBeInTheDocument();
  });

  // UT-055. Invariant: Open in exists only with the desktop target bridge (US-006.AC-6).
  it("Should render no Open in control without the desktop target bridge (UT-055)", () => {
    const control = <OpenInButton>Open in</OpenInButton>;
    const browser = renderPane(control);
    expect(browser.container).not.toHaveTextContent("Open in");
    browser.unmount();

    window.compozyShell = {
      platform: "darwin",
      files: { openInEditor: vi.fn() },
    } as unknown as CompozyShellBridge;
    const legacyDesktop = renderPane(control);
    expect(legacyDesktop.container).not.toHaveTextContent("Open in");
    legacyDesktop.unmount();

    window.compozyShell = {
      platform: "darwin",
      files: { openInEditor: vi.fn(), listOpenTargets: vi.fn() },
    } as unknown as CompozyShellBridge;
    renderPane(control);
    expect(screen.getByText("Open in")).toBeInTheDocument();
  });
  it("Should select an application and remember it on the primary Open in button", async () => {
    localStorage.removeItem("compozy.open-target");
    const openWith = vi.fn(async () => ({ status: "opened" as const }));
    window.compozyShell = {
      platform: "darwin",
      files: {
        openInEditor: vi.fn(),
        openWith,
        listOpenTargets: async () => [
          { id: "configured", label: "Configured editor", kind: "editor", supportsLine: true },
          { id: "cursor", label: "Cursor", kind: "editor", supportsLine: true },
        ],
      },
    } as unknown as CompozyShellBridge;
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(workspaceDetailOptions("ws_1").queryKey, {
      workspace: {
        id: "ws_1",
        root_dir: "/repo",
        name: "Project",
        add_dirs: [],
        created_at: "2026-11-07T00:00:00Z",
        updated_at: "2026-11-07T00:00:00Z",
      },
    });
    render(
      frame(
        client,
        <OpenInButton root={{ kind: "workspace", workspaceId: "ws_1" }} path="a.ts" line={12} />
      )
    );
    await screen.findByRole("button", { name: "Choose application" });
    fireEvent.click(screen.getByRole("button", { name: "Choose application" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: "Cursor" }));
    await waitFor(() =>
      expect(openWith).toHaveBeenCalledWith({
        target: "cursor",
        path: "/repo/a.ts",
        line: 12,
        column: 1,
        roots: ["/repo", "/repo"],
      })
    );
    expect(await screen.findByRole("button", { name: "Open in Cursor" })).toBeInTheDocument();
    expect(localStorage.getItem("compozy.open-target")).toBe("cursor");
    localStorage.removeItem("compozy.open-target");
  });

  describe("Find in file", () => {
    // 17 `foo` matches spread over 20,000 lines; the third sits on line 12,000.
    const FOO_LINES = [
      10,
      4_000,
      12_000,
      ...Array.from({ length: 14 }, (_, i) => 15_000 + i * 300),
    ];
    function longText(): string {
      const foo = new Set(FOO_LINES);
      return Array.from({ length: 20_000 }, (_, index) =>
        foo.has(index + 1) ? `const foo${index + 1} = ${index + 1};` : `const bar${index + 1} = 0;`
      ).join("\n");
    }
    function renderedLine(container: HTMLElement, line: number): HTMLElement | null {
      const host = container.querySelector("diffs-container");
      return host?.shadowRoot?.querySelector<HTMLElement>(`[data-line="${line}"]`) ?? null;
    }
    function stubLongFile() {
      stubContent(() =>
        content({ path: "big.ts", kind: "text", size: 400_000, text: longText(), revision: "r1" })
      );
    }

    // UT-094
    it("Should count matches over the full text and scroll an unrendered match into view", async () => {
      stubLongFile();
      const scrollTo = vi.spyOn(Element.prototype, "scrollTo");
      const view = renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("big.ts")} />);
      const viewer = await screen.findByTestId("file-viewer-focus");
      expect(renderedLine(view.container, 12_000)).toBeNull();

      viewer.focus();
      fireEvent.keyDown(viewer, { key: "f", code: "KeyF", metaKey: true });
      const input = await screen.findByTestId("viewer-find-input");
      expect(input).toHaveFocus();
      fireEvent.change(input, { target: { value: "foo" } });
      expect(screen.getByTestId("viewer-find-count")).toHaveTextContent("1 of 17");
      fireEvent.keyDown(input, { key: "Enter" });
      fireEvent.keyDown(input, { key: "Enter" });
      expect(screen.getByTestId("viewer-find-count")).toHaveTextContent("3 of 17");

      const code = view.container.querySelector("[data-slot='code-viewer']");
      expect(code).toHaveAttribute("data-find-matches", "17");
      expect(code).toHaveAttribute("data-find-active-line", "12000");
      await waitFor(() => expect(renderedLine(view.container, 12_000)).not.toBeNull());
      const scrolledTo = scrollTo.mock.calls.map(
        ([options]) => (options as ScrollToOptions).top ?? 0
      );
      expect(Math.max(...scrolledTo)).toBeGreaterThan(0);

      fireEvent.keyDown(input, { key: "Enter", shiftKey: true });
      expect(screen.getByTestId("viewer-find-count")).toHaveTextContent("2 of 17");
      fireEvent.keyDown(input, { key: "Escape" });
      expect(screen.queryByTestId("viewer-find-bar")).toBeNull();
      expect(viewer).toHaveFocus();
      scrollTo.mockRestore();
    });

    // UT-095
    it("Should leave Cmd+F to the browser while the viewer is not focused", async () => {
      stubLongFile();
      renderPane(<FileViewerPane root={ROOT} target={fileViewerTarget("big.ts")} />);
      const viewer = await screen.findByTestId("file-viewer-focus");
      const outside = fireEvent.keyDown(document.body, { key: "f", code: "KeyF", metaKey: true });
      expect(outside).toBe(true);
      expect(screen.queryByTestId("viewer-find-bar")).toBeNull();

      const inside = fireEvent.keyDown(viewer, { key: "f", code: "KeyF", metaKey: true });
      expect(inside).toBe(false);
      expect(screen.getByTestId("viewer-find-bar")).toBeInTheDocument();
    });

    it("Should open pre-filled from Search at the opened line's match and note truncation", async () => {
      stubContent(() =>
        content({
          path: "big.ts",
          kind: "text",
          size: 2 * 1024 * 1024,
          text: longText(),
          truncated: true,
        })
      );
      renderPane(
        <FileViewerPane
          root={ROOT}
          target={fileViewerTarget("big.ts", 4_000, { query: "FOO", matchCase: false })}
        />
      );
      expect(await screen.findByTestId("viewer-find-input")).toHaveValue("FOO");
      expect(screen.getByTestId("viewer-find-count")).toHaveTextContent("2 of 17");
      expect(screen.getByTestId("viewer-find-truncated")).toHaveTextContent(
        "Searching the first 1 MB"
      );
      fireEvent.click(screen.getByRole("button", { name: "Match case" }));
      expect(screen.getByTestId("viewer-find-count")).toHaveTextContent("No results");
      expect(screen.getByTestId("viewer-find-input")).toHaveAttribute("aria-invalid", "true");
    });

    // US-014.AC-4. Invariant: ↑/↓ in the Search results open each match in turn, and the seeded
    // find bar of the opened file never takes focus from the results list.
    it("Should keep stepping Search results with ↑/↓ while each match opens seeded", async () => {
      const text = ["const foo = 1;", "bar();", "foo();", "baz();"].join("\n");
      stubContent(url =>
        content({ path: url.searchParams.get("path"), kind: "text", size: 40, text })
      );
      const match = (line: number) => ({
        line,
        column: 1,
        text: line === 1 ? "const foo = 1;" : "foo();",
        ranges: [[line === 1 ? 6 : 0, line === 1 ? 9 : 3]],
      });
      const files = [
        { path: "a.ts", matches: [match(1), match(3)] },
        { path: "b.ts", matches: [match(3)] },
      ] as unknown as FileSearchFile[];
      function Harness() {
        const [target, setTarget] = useState<FileViewerTarget | null>(null);
        return (
          <>
            <FilesSearchResults
              files={files}
              dimmed={false}
              onOpenMatch={(file, hit) =>
                setTarget(fileViewerTarget(file.path, hit.line, { query: "foo", matchCase: false }))
              }
            />
            <FileViewerPane root={ROOT} target={target} />
          </>
        );
      }
      renderPane(<Harness />);
      const results = screen.getByTestId("files-search-results");
      results.focus();
      const press = async (key: string, path: string, count: string) => {
        fireEvent.keyDown(document.activeElement ?? document.body, { key });
        await waitFor(() =>
          expect(screen.getByTestId("viewer-find-count")).toHaveTextContent(count)
        );
        expect(screen.getByTestId("file-viewer-header")).toHaveTextContent(path);
        expect(results).toHaveFocus();
      };
      await press("ArrowDown", "a.ts", "1 of 2");
      await press("ArrowDown", "a.ts", "2 of 2");
      await press("ArrowDown", "b.ts", "2 of 2");
      // Both files hold the same text, so b.ts line 3 is its second `foo`.
      // The last match is a boundary: ↓ stays on it, ↑ steps back.
      await press("ArrowDown", "b.ts", "2 of 2");
      await press("ArrowUp", "a.ts", "2 of 2");
      expect(screen.getByTestId("viewer-find-input")).not.toHaveFocus();
    });
  });
});
