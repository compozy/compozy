import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { workspaceDetailOptions } from "@/systems/workspace";
import type { CompozyShellBridge } from "@/systems/os/lib/desktop-shell-bridge";

import type { FilesRootRef } from "../../adapters/files-api";
import { fileViewerTarget, outsideViewerTarget } from "../../lib/file-viewer-target";
import { FileViewerPane } from "../file-viewer-pane";
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
});
