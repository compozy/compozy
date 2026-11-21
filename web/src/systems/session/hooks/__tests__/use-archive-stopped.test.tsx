// Suite: Archive stopped (#1924, T-09/T-10)
// Invariant: one action sends its scope (one project or every project) to the daemon for the
// acting profile; the toast reports the count, the skip summary, failures by title, and offers
// Undo only when something was archived; a scope's action is refused while its run is in flight.
// Undo unarchives exactly the reported session ids, root first within each thread, and names the
// threads it could not restore.
// Boundary IN: useArchiveStopped, the session archive adapter and TanStack Query.
// Boundary OUT: the network seam (fetch) and the toast.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "@compozy/ui";

import type { SessionArchiveStoppedResponse } from "../../adapters/session-archive-api";
import { primarySessionFixture } from "../../mocks/fixtures";
import { useArchiveStopped } from "../use-archive-stopped";

vi.mock("@compozy/ui", async importOriginal => ({
  ...(await importOriginal<typeof import("@compozy/ui")>()),
  toast: { message: vi.fn(), error: vi.fn(), success: vi.fn() },
}));

const PROFILE = "default";

function report(overrides: Partial<SessionArchiveStoppedResponse> = {}) {
  return {
    dry_run: false,
    archived: [],
    archived_thread_count: 0,
    archived_session_count: 0,
    skip_counts: {},
    skipped: [],
    skipped_truncated: false,
    failures: [],
    failures_truncated: false,
    ...overrides,
  } satisfies SessionArchiveStoppedResponse;
}

const TWO_THREADS = report({
  archived: [
    {
      root_id: "root-a",
      workspace_id: "ws-alpha",
      title: "Alpha",
      session_ids: ["root-a", "child-a1", "child-a2"],
    },
    { root_id: "root-b", workspace_id: "ws-beta", title: "Beta", session_ids: ["root-b"] },
  ],
  archived_thread_count: 2,
  archived_session_count: 4,
  skip_counts: { waiting_on_you: 2, running: 1 },
});

interface Recorded {
  path: string;
  body: unknown;
}

let recorded: Recorded[];
let archiveAnswer: () => Promise<Response>;
let failingUnarchive: Set<string>;

beforeEach(() => {
  recorded = [];
  failingUnarchive = new Set();
  archiveAnswer = async () => Response.json(TWO_THREADS);
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.message).mockClear();
  vi.mocked(toast.error).mockClear();
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async input => {
      const request = input as Request;
      const url = new URL(request.url);
      const text = await request.clone().text();
      recorded.push({ path: `${url.pathname}${url.search}`, body: text ? JSON.parse(text) : null });
      if (url.pathname === "/api/sessions/archive-stopped") return archiveAnswer();
      const id = url.pathname.split("/").at(-2)!;
      if (failingUnarchive.has(id)) {
        return Response.json({ error: "store unavailable" }, { status: 500 });
      }
      return Response.json({ session: { ...primarySessionFixture, id } });
    })
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderArchiveStopped() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(() => useArchiveStopped(PROFILE), { wrapper });
}

function undoAction(call: unknown[] | undefined): () => void {
  const options = call?.[1] as { action?: { label: string; onClick: () => void } } | undefined;
  expect(options?.action?.label).toBe("Undo");
  return options!.action!.onClick;
}

const unarchived = () =>
  recorded
    .filter(entry => entry.path.includes("/unarchive"))
    .map(entry => entry.path.split("/").at(-2)!);

describe("useArchiveStopped", () => {
  it.each([
    {
      scope: { kind: "project", workspaceId: "ws-alpha" } as const,
      body: { workspace_id: "ws-alpha", dry_run: false },
    },
    { scope: { kind: "all" } as const, body: { all_workspaces: true, dry_run: false } },
  ])("Should send the $scope.kind scope for the acting profile", async ({ scope, body }) => {
    const { result } = renderArchiveStopped();

    act(() => result.current.archiveStopped(scope));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(recorded[0]).toEqual({
      path: `/api/sessions/archive-stopped?profile=${PROFILE}`,
      body,
    });
  });

  it("Should toast the count with the skip summary and Undo", async () => {
    const { result } = renderArchiveStopped();

    act(() => result.current.archiveStopped({ kind: "all" }));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    const call = vi.mocked(toast.success).mock.calls[0];
    expect(call?.[0]).toBe("Archived 2 threads");
    expect(call?.[1]).toMatchObject({ description: "3 skipped: 2 waiting on you, 1 running" });
    undoAction(call);
  });

  it("Should say nothing was eligible without Undo", async () => {
    archiveAnswer = async () => Response.json(report({ skip_counts: { pinned: 1 } }));
    const { result } = renderArchiveStopped();

    act(() => result.current.archiveStopped({ kind: "project", workspaceId: "ws-alpha" }));

    await waitFor(() => expect(toast.message).toHaveBeenCalled());
    const [title, options] = vi.mocked(toast.message).mock.calls[0]!;
    expect(title).toBe("No stopped threads to archive");
    expect(options).toEqual({ description: "1 skipped: 1 pinned" });
  });

  it("Should list up to three failed titles and keep Undo for what was archived", async () => {
    const failure = (title: string) => ({
      root_id: `root-${title}`,
      workspace_id: "ws-alpha",
      title,
      code: "archive_failed",
      message: "store error",
    });
    archiveAnswer = async () =>
      Response.json({
        ...TWO_THREADS,
        skip_counts: {},
        failures: ["One", "Two", "Three", "Four", "Five"].map(failure),
      });
    const { result } = renderArchiveStopped();

    act(() => result.current.archiveStopped({ kind: "all" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    const call = vi.mocked(toast.error).mock.calls[0];
    expect(call?.[0]).toBe("Archived 2 threads, 5 failed");
    expect(call?.[1]).toMatchObject({ description: "One, Two, Three and 2 more" });
    undoAction(call);
  });

  it("Should report a failed request", async () => {
    archiveAnswer = async () => Response.json({ error: "denied" }, { status: 403 });
    const { result } = renderArchiveStopped();

    act(() => result.current.archiveStopped({ kind: "all" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1));
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("Should hold a scope's action while its run is in flight", async () => {
    let answer!: (response: Response) => void;
    archiveAnswer = () => new Promise(resolve => (answer = resolve));
    const { result } = renderArchiveStopped();
    const project = { kind: "project", workspaceId: "ws-alpha" } as const;

    act(() => result.current.archiveStopped(project));

    await waitFor(() => expect(result.current.running(project)).toBe(true));
    expect(result.current.running({ kind: "all" })).toBe(false);
    act(() => result.current.archiveStopped(project));
    expect(recorded).toHaveLength(1);

    answer(Response.json(TWO_THREADS));
    await waitFor(() => expect(result.current.running(project)).toBe(false));
  });

  it("Should undo exactly the archived ids, root first within each thread", async () => {
    const { result } = renderArchiveStopped();
    act(() => result.current.archiveStopped({ kind: "all" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));

    act(() => undoAction(vi.mocked(toast.success).mock.calls[0])());

    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(2));
    expect(vi.mocked(toast.success).mock.calls[1]?.[0]).toBe("Restored 2 threads");
    const order = unarchived();
    expect([...order].sort()).toEqual(["child-a1", "child-a2", "root-a", "root-b"]);
    expect(order.indexOf("root-a")).toBeLessThan(order.indexOf("child-a1"));
    expect(order.indexOf("child-a1")).toBeLessThan(order.indexOf("child-a2"));
    expect(recorded.find(entry => entry.path.includes("/root-b/unarchive"))?.path).toContain(
      "/api/workspaces/ws-beta/"
    );
  });

  it("Should name the threads Undo could not restore", async () => {
    failingUnarchive.add("child-a1");
    const { result } = renderArchiveStopped();
    act(() => result.current.archiveStopped({ kind: "all" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledTimes(1));

    act(() => undoAction(vi.mocked(toast.success).mock.calls[0])());

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    const [title, options] = vi.mocked(toast.error).mock.calls[0]!;
    expect(title).toBe("Restored 1 thread, 1 failed");
    expect(options).toEqual({ description: "Alpha" });
  });
});
