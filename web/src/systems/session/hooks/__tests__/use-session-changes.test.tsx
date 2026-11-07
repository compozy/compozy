import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionChangedFilesRowView } from "@/components/assistant-ui/session-changed-files-row";

import type { SessionChangesSummary } from "../../adapters/session-changes-api";
import { sessionKeys } from "../../lib/query-keys";
import { sessionChangeItems } from "../../lib/session-changes";
import type { SessionTranscriptData } from "../../lib/session-transcript-query";
import { useSessionChanges } from "../use-session-changes";
import { useSessionChangesLiveRefresh } from "../use-session-changes-live-refresh";
import { useSessionChangesView } from "../use-session-changes-view";
import {
  requestSessionReviewChanges,
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
  getSessionChangePatches,
  getSessionChanges,
  listSessionChangeBases,
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
  vi.clearAllMocks();
});

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

describe("changed-file routing", () => {
  // UT-053. Invariant: a turn's changed-file row opens Changes focused on a listed file, and
  // falls back to Files at the path when the summary doesn't list it.
  it("Should focus a listed file in Changes and open Files for an unlisted one", async () => {
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
    expect(preference()).toMatchObject({ open: true, tab: "changes" });
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
      { wrapper: createWrapper(newQueryClient()) }
    );
    await waitFor(() => expect(result.current.selectedPath).toBe("b.go"));

    fireEvent.click(screen.getByRole("button", { name: "Review changes in missing.go" }));
    await waitFor(() => expect(preference()?.tab).toBe("files"));
    expect(sessionReviewPanelStore.getSnapshot().context.targets[sessionId]).toMatchObject({
      kind: "file",
      path: "missing.go",
    });
  });
});
