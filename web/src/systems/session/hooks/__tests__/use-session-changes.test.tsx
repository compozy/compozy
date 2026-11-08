import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import { createAtom } from "@xstate/store";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionChangedFilesRowView } from "@/components/assistant-ui/session-changed-files-row";
import { filesAppPlaceStore } from "@/systems/files/hooks/use-files-app-place";
import { OsShellContext, type OsShellHandle } from "@/systems/os";

import type { SessionChangesSummary } from "../../adapters/session-changes-api";
import { SessionChangesTab } from "../../components/session-changes/session-changes-tab";
import { sessionKeys } from "../../lib/query-keys";
import { sessionChangeItems } from "../../lib/session-changes";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import { useSessionChanges } from "../use-session-changes";
import { useSessionChangesLiveRefresh } from "../use-session-changes-live-refresh";
import { useSessionChangesView } from "../use-session-changes-view";
import { rehydrateStore } from "@xstate/store/persist";

import {
  requestSessionReviewChanges,
  SESSION_REVIEW_PANEL_STORAGE_KEY,
  sessionReviewPanelStore,
  useSessionReviewPanelState,
} from "../use-session-review-panel-state";

vi.mock("../../adapters/session-changes-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-changes-api")>()),
  getSessionChanges: vi.fn(),
  getSessionChangePatches: vi.fn(),
  getSessionChangeContents: vi.fn(),
  listSessionChangeBases: vi.fn(),
}));

import {
  SessionChangesApiError,
  getSessionChangeContents,
  getSessionChangePatches,
  getSessionChanges,
  listSessionChangeBases,
  type SessionChangeTarget,
} from "../../adapters/session-changes-api";

const WORKSPACE_ID = "ws_changes";

function changesSummary(
  paths: readonly string[],
  overrides: Partial<SessionChangesSummary> = {}
): SessionChangesSummary {
  return {
    scope: "branch",
    root: { kind: "worktree", workspace_id: WORKSPACE_ID, name: "compozy", branch: "feat/x" },
    head: { branch: "feat/x", sha: "c2d1da2e4f00" },
    base: { ref: "main", source: "default_branch", merge_base_sha: "8e7fa1af2000" },
    totals: { files: paths.length, additions: paths.length, deletions: 0 },
    files: paths.map(path => ({
      path,
      status: "modified",
      additions: 1,
      deletions: 0,
      fingerprint: `f:${path}`,
    })),
    truncated: false,
    revision: "sha256:r1",
    ...overrides,
  } as SessionChangesSummary;
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function newQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function transcriptWithSettledTools(count: number): SessionTranscriptData {
  const entries = Array.from({ length: count }, (_, index) => ({
    sequence: index + 1,
    start_sequence: index + 1,
    message: {
      id: `tool-${index}`,
      role: "assistant",
      parts: [
        {
          type: "dynamic-tool",
          toolName: "Edit",
          toolCallId: `call-${index}`,
          state: "output-available",
          input: {},
          output: "ok",
        },
      ],
    },
  }));
  return {
    pages: [{ entries, cursor: count }],
    pageParams: [null],
  } as unknown as SessionTranscriptData;
}

beforeEach(() => {
  vi.mocked(listSessionChangeBases).mockResolvedValue({
    default: { ref: "main", kind: "local", is_default: true },
    resolvable: true,
    refs: [{ ref: "main", kind: "local", is_default: true }],
    truncated: false,
  });
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

/** A patch with unchanged lines above its hunk, so the diff offers to expand context. */
function contextPatch(path: string): string {
  return [
    `diff --git a/${path} b/${path}`,
    "index 1111111..2222222 100644",
    `--- a/${path}`,
    `+++ b/${path}`,
    "@@ -10,3 +10,4 @@",
    " a",
    "+b",
    " c",
    " d",
    "",
  ].join("\n");
}

function mockPatches() {
  vi.mocked(getSessionChangePatches).mockImplementation(
    async (_request, targets: readonly SessionChangeTarget[]) =>
      ({
        patches: targets.map(target => ({
          path: target.path,
          fingerprint: `f:${target.path}`,
          patch: contextPatch(target.path),
        })),
      }) as Awaited<ReturnType<typeof getSessionChangePatches>>
  );
}

function ChangesTab({ sessionId }: { sessionId: string }) {
  const review = useSessionReviewPanelState(sessionId);
  return <SessionChangesTab workspaceId={WORKSPACE_ID} sessionId={sessionId} review={review} />;
}

function renderChangesTab(sessionId: string, wrap?: (children: ReactNode) => ReactNode) {
  const Wrapper = createWrapper(newQueryClient());
  const tab = <ChangesTab sessionId={sessionId} />;
  return render(<Wrapper>{wrap ? wrap(tab) : tab}</Wrapper>);
}

function reviewPreference(sessionId: string) {
  return sessionReviewPanelStore.getSnapshot().context.bySession[sessionId];
}

function expandControls(path: string): number {
  const header = document.querySelector(`[data-testid="changes-file-header"][data-path="${path}"]`);
  const host = header?.closest("diffs-container");
  return host?.shadowRoot?.querySelectorAll("[data-expand-button]").length ?? 0;
}

/** Reports the Changes tab as narrower than its split breakpoint; other observers stay idle. */
class NarrowResizeObserver {
  constructor(private readonly callback: ResizeObserverCallback) {}
  observe(target: Element) {
    if (target.getAttribute("data-testid") !== "session-changes-tab") return;
    this.callback(
      [{ contentRect: { width: 600 } } as ResizeObserverEntry],
      this as unknown as ResizeObserver
    );
  }
  unobserve() {}
  disconnect() {}
}

describe("useSessionChangesLiveRefresh", () => {
  // UT-057. Invariant: a burst of settled tool calls debounces into exactly one summary refresh.
  it("Should refresh the summary once for three tool calls completing within one second", async () => {
    vi.useFakeTimers();
    const queryClient = newQueryClient();
    const sessionId = "sess-live-refresh";
    const transcriptKey = sessionKeys.transcript(WORKSPACE_ID, sessionId);
    queryClient.setQueryData(transcriptKey, transcriptWithSettledTools(0));
    const onRefresh = vi.fn();
    renderHook(
      () =>
        useSessionChangesLiveRefresh({
          workspaceId: WORKSPACE_ID,
          sessionId,
          enabled: true,
          onRefresh,
        }),
      { wrapper: createWrapper(queryClient) }
    );

    for (const count of [1, 2, 3]) {
      act(() => {
        queryClient.setQueryData(transcriptKey, transcriptWithSettledTools(count));
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(300);
      });
    }
    expect(onRefresh).not.toHaveBeenCalled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  // US-010.AC-2. Invariant: the session's OS window gaining focus refreshes the summary once;
  // a window that was already focused, or loses focus, does not.
  it("Should refresh the summary when the session window gains focus", () => {
    const onRefresh = vi.fn();
    const { rerender } = renderHook(
      ({ windowFocused }: { windowFocused: boolean }) =>
        useSessionChangesLiveRefresh({
          workspaceId: WORKSPACE_ID,
          sessionId: "sess-window-focus",
          enabled: true,
          windowFocused,
          onRefresh,
        }),
      { wrapper: createWrapper(newQueryClient()), initialProps: { windowFocused: true } }
    );
    expect(onRefresh).not.toHaveBeenCalled();
    rerender({ windowFocused: false });
    expect(onRefresh).not.toHaveBeenCalled();
    rerender({ windowFocused: true });
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  // US-010.AC-2 (operator decision): switching between in-app OS windows counts as focusing
  // the session window, so the Changes tab rereads its summary.
  it("Should reread the summary when the shell focuses the session's window", async () => {
    const sessionId = "sess-shell-focus";
    const windows = {
      "w-session": {
        desktopId: "d1",
        minimized: false,
        stackActive: true,
        route: { pathname: `/agents/codex/sessions/${sessionId}` },
      },
      "w-files": {
        desktopId: "d1",
        minimized: false,
        stackActive: true,
        route: { pathname: "/files/ws_changes" },
      },
    };
    const projection = createAtom({ activeDesktopId: "d1", focusedId: "w-files", windows });
    const shell = { projection } as unknown as OsShellHandle;
    vi.mocked(getSessionChanges).mockResolvedValue(changesSummary(["a.go"]));
    mockPatches();
    renderChangesTab(sessionId, children => (
      <OsShellContext.Provider value={shell}>{children}</OsShellContext.Provider>
    ));
    await screen.findByTestId("changes-tree");
    expect(getSessionChanges).toHaveBeenCalledTimes(1);

    act(() => projection.set({ activeDesktopId: "d1", focusedId: "w-session", windows }));
    await waitFor(() => expect(getSessionChanges).toHaveBeenCalledTimes(2));
  });
});

describe("useSessionChanges", () => {
  // UT-058. Invariant: a stale detail read refetches the summary once and redraws; a second
  // consecutive stale read stops at "Couldn't refresh · Retry" without looping.
  it("Should refetch once on a stale patch and stop at a retry state on the second", async () => {
    const sessionId = "sess-stale-fence";
    vi.mocked(getSessionChanges)
      .mockResolvedValueOnce(changesSummary(["a.go"]))
      .mockResolvedValue(changesSummary(["a.go"], { revision: "sha256:r2" }));
    vi.mocked(getSessionChangePatches).mockRejectedValue(
      new SessionChangesApiError("changes moved", 409, "changes_stale")
    );
    const reloadItems = vi.fn();
    const { result } = renderHook(
      () =>
        useSessionChanges({
          workspaceId: WORKSPACE_ID,
          sessionId,
          revealed: new Set(),
          reloadItems,
        }),
      { wrapper: createWrapper(newQueryClient()) }
    );
    await waitFor(() => expect(result.current.summary?.revision).toBe("sha256:r1"));

    const first = sessionChangeItems(result.current.summary!, {
      revealed: new Set(),
      collapsed: {},
    });
    await expect(result.current.loadPatch(first[0]!)).rejects.toThrow("changes moved");
    await waitFor(() => expect(result.current.summary?.revision).toBe("sha256:r2"));
    expect(getSessionChanges).toHaveBeenCalledTimes(2);
    await waitFor(() => expect(reloadItems).toHaveBeenCalledWith(["a.go"]));
    expect(result.current.refreshFailed).toBe(false);

    const second = sessionChangeItems(result.current.summary!, {
      revealed: new Set(),
      collapsed: {},
    });
    await expect(result.current.loadPatch(second[0]!)).rejects.toThrow("changes moved");
    await waitFor(() => expect(result.current.refreshFailed).toBe(true));
    expect(getSessionChanges).toHaveBeenCalledTimes(2);
  });
});

describe("Review panel preference", () => {
  // Invariant: a preference stored while the panel still had a Files tab (`tab: "files"`) reads
  // back as the Changes-only panel with its other choices intact.
  it("Should read a stored Files tab preference as the Changes panel", async () => {
    const sessionId = "sess-stored-files-tab";
    localStorage.setItem(
      SESSION_REVIEW_PANEL_STORAGE_KEY,
      JSON.stringify({
        context: {
          bySession: { [sessionId]: { open: true, tab: "files", size: "40%", diffMode: "split" } },
        },
        version: 0,
      })
    );
    await rehydrateStore(sessionReviewPanelStore);
    expect(sessionReviewPanelStore.getSnapshot().context.bySession[sessionId]).toEqual({
      open: true,
      size: "40%",
      diffMode: "split",
    });
    const { result } = renderHook(() => useSessionReviewPanelState(sessionId));
    expect(result.current).toMatchObject({ open: true, size: "40%", diffMode: "split" });
    localStorage.removeItem(SESSION_REVIEW_PANEL_STORAGE_KEY);
  });
});

describe("changed-file routing", () => {
  // UT-053. Invariant: a turn's changed-file row opens Changes focused on a listed file, and
  // falls back to the project's Files window at the path when the summary doesn't list it.
  it("Should focus a listed file in Changes and open the Files window for an unlisted one", async () => {
    const sessionId = "sess-changed-file-route";
    vi.mocked(getSessionChanges).mockResolvedValue(changesSummary(["a.go", "b.go"]));
    const onOpenFile = (path: string) => requestSessionReviewChanges(sessionId, { path });
    const files = [
      { path: "b.go", additions: 1, deletions: 0 },
      { path: "missing.go", additions: 2, deletions: 0 },
    ];
    render(
      <SessionChangedFilesRowView
        row={
          {
            kind: "changed-files",
            id: "turn-1",
            files,
            additions: 3,
            deletions: 0,
            expanded: true,
          } as never
        }
        onToggle={() => undefined}
        onOpenFile={onOpenFile}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Review changes in b.go" }));

    const preference = () => sessionReviewPanelStore.getSnapshot().context.bySession[sessionId];
    expect(preference()).toMatchObject({ open: true });
    const userOpen = vi.fn(async () => undefined);
    const shell = { coordinator: { userOpen } } as unknown as OsShellHandle;
    const queryClient = newQueryClient();
    queryClient.setQueryData(sessionKeys.detail(WORKSPACE_ID, sessionId), {
      id: sessionId,
      workspace_id: WORKSPACE_ID,
      working_dir: "/repo",
    });
    const { result } = renderHook(
      () => {
        const review = useSessionReviewPanelState(sessionId);
        return useSessionChangesView({
          workspaceId: WORKSPACE_ID,
          sessionId,
          review,
          rootRef: { current: null },
          diffRef: { current: null },
        });
      },
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={queryClient}>
            <OsShellContext value={shell}>{children}</OsShellContext>
          </QueryClientProvider>
        ),
      }
    );
    await waitFor(() => expect(result.current.selectedPath).toBe("b.go"));

    fireEvent.click(screen.getByRole("button", { name: "Review changes in missing.go" }));
    await waitFor(() =>
      expect(userOpen).toHaveBeenCalledWith(
        expect.objectContaining({ app: "files", instanceKey: WORKSPACE_ID })
      )
    );
    expect(filesAppPlaceStore.getSnapshot().context.targets[WORKSPACE_ID]).toMatchObject({
      kind: "file",
      path: "missing.go",
    });
  });
});

describe("SessionChangesTab", () => {
  // US-009.AC-1/EC-2. Invariant: `]` steps to the next file, `[` back, and on the last file
  // Next is disabled and `]` stays put.
  it("Should step files with the bracket keys and stop at the last file", async () => {
    const sessionId = "sess-bracket-step";
    vi.mocked(getSessionChanges).mockResolvedValue(
      changesSummary(["a.go", "b.go", "c.go"], {
        totals: { files: 3, additions: 20_001, deletions: 0 },
      })
    );
    mockPatches();
    renderChangesTab(sessionId);
    const position = await screen.findByTestId("changes-large-position");
    const tab = screen.getByTestId("session-changes-tab");
    expect(position).toHaveTextContent("1 of 3");

    fireEvent.keyDown(tab, { key: "]" });
    expect(position).toHaveTextContent("2 of 3");
    fireEvent.keyDown(tab, { key: "[" });
    expect(position).toHaveTextContent("1 of 3");
    fireEvent.keyDown(tab, { key: "]" });
    fireEvent.keyDown(tab, { key: "]" });
    expect(position).toHaveTextContent("3 of 3");
    expect(screen.getByTestId("changes-next-file")).toBeDisabled();
    fireEvent.keyDown(tab, { key: "]" });
    expect(position).toHaveTextContent("3 of 3");
  });

  // US-007.AC-5. Invariant: Alt+S flips the persisted split/unified mode, except while typing
  // in a text field.
  it("Should toggle the persisted diff mode with Alt+S outside text fields", async () => {
    const sessionId = "sess-alt-s";
    vi.mocked(getSessionChanges).mockResolvedValue(changesSummary(["a.go"]));
    mockPatches();
    renderChangesTab(sessionId);
    const viewer = await screen.findByTestId("changes-diff-list");
    const tab = screen.getByTestId("session-changes-tab");

    fireEvent.keyDown(tab, { altKey: true, code: "KeyS" });
    expect(reviewPreference(sessionId)?.diffMode).toBe("split");
    expect(viewer).toHaveAttribute("data-mode", "split");

    fireEvent.keyDown(screen.getByLabelText("Filter changed files"), {
      altKey: true,
      code: "KeyS",
    });
    expect(reviewPreference(sessionId)?.diffMode).toBe("split");

    fireEvent.keyDown(tab, { altKey: true, code: "KeyS" });
    expect(reviewPreference(sessionId)?.diffMode).toBe("unified");
  });

  // US-007.AC-5. Invariant: a narrow session window is always unified; Alt+S leaves the
  // persisted mode alone there.
  it("Should ignore Alt+S in a narrow session window", async () => {
    vi.stubGlobal("ResizeObserver", NarrowResizeObserver);
    const sessionId = "sess-alt-s-narrow";
    vi.mocked(getSessionChanges).mockResolvedValue(changesSummary(["a.go"]));
    mockPatches();
    renderChangesTab(sessionId);
    const viewer = await screen.findByTestId("changes-diff-list");

    fireEvent.keyDown(screen.getByTestId("session-changes-tab"), { altKey: true, code: "KeyS" });
    expect(reviewPreference(sessionId)?.diffMode).toBeUndefined();
    expect(viewer).toHaveAttribute("data-mode", "unified");
  });

  // US-007.EC-5. Invariant: when the daemon refuses a file's full contents as too large, the
  // expand control goes away and the header says "File too large to expand".
  it("Should drop the expand control and say why when a file is too large to expand", async () => {
    const sessionId = "sess-too-large-expand";
    vi.mocked(getSessionChanges).mockResolvedValue(changesSummary(["big.go"]));
    mockPatches();
    vi.mocked(getSessionChangeContents).mockRejectedValue(
      new SessionChangesApiError("file is too large to expand", 422, "too_large_to_expand")
    );
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    renderChangesTab(sessionId);
    await waitFor(() => expect(expandControls("big.go")).toBeGreaterThan(0));

    const header = screen.getByTestId("changes-file-header");
    const expand = header
      .closest("diffs-container")!
      .shadowRoot!.querySelector<HTMLElement>("[data-expand-button]")!;
    fireEvent.click(expand);

    await waitFor(() => expect(getSessionChangeContents).toHaveBeenCalledTimes(1));
    expect(await screen.findByText("File too large to expand")).toBeInTheDocument();
    await waitFor(() => expect(expandControls("big.go")).toBe(0));
  });

  // US-008.EC-1. Invariant: a persisted base that no longer exists is dropped back to the
  // default base with a notice, so later reads stop asking for it.
  it("Should fall back to the default base when the persisted base is gone", async () => {
    const sessionId = "sess-missing-base";
    sessionReviewPanelStore.trigger.changesPreferenceChanged({
      sessionId,
      patch: { scope: "branch", base: "origin/release-1" },
    });
    vi.mocked(getSessionChanges).mockImplementation(async request => {
      if (request.base === "origin/release-1") {
        throw new SessionChangesApiError("base ref not found", 409, "base_not_found");
      }
      return changesSummary(["a.go"]);
    });
    mockPatches();
    renderChangesTab(sessionId);

    expect(await screen.findByTestId("changes-base-fallback")).toHaveTextContent(
      "Base origin/release-1 no longer exists. Showing changes against the default base."
    );
    expect(reviewPreference(sessionId)?.base).toBeUndefined();
    expect(vi.mocked(getSessionChanges).mock.lastCall?.[0].base).toBeUndefined();
    expect(await screen.findByTestId("changes-tree")).toHaveAttribute("data-file-count", "1");
  });
});
