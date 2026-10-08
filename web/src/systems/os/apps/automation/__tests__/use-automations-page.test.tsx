// Suite: Automations listing model
// Invariant: two list loads (minus `start`) merge into one sorted, honestly counted listing,
//   and row actions wait for the daemon (never optimistic).
// Boundary IN: route search + automation HTTP adapter responses.
// Boundary OUT: rendering (row/card suites) and editor internals (editor slice).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { automationStoryJobs, automationStoryTriggers } from "@/systems/automation/mocks";
import { toAutomationView, type AutomationJob, type AutomationTrigger } from "@/systems/automation";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  listJobs: vi.fn(),
  listTriggers: vi.fn(),
  updateJob: vi.fn(),
  updateTrigger: vi.fn(),
  triggerJob: vi.fn(),
  runtime: { available: true } as { available: boolean },
  activeWorkspaceId: "ws_launch_hq" as string | null,
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("sonner", () => ({ toast: mocks.toast }));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mocks.navigate,
}));

vi.mock("@/systems/automation/adapters/automation-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/automation/adapters/automation-api")>()),
  listAutomationJobs: mocks.listJobs,
  listAutomationTriggers: mocks.listTriggers,
  updateAutomationJob: mocks.updateJob,
  updateAutomationTrigger: mocks.updateTrigger,
  triggerAutomationJob: mocks.triggerJob,
}));

vi.mock("@/systems/profiles/hooks/use-profile-read-scope", () => ({
  useProfileReadScope: () => ({
    aggregate: false,
    destination: "default",
    key: "default",
    params: {},
    scopeLabel: "default",
    ownerOf: () => undefined,
  }),
}));

vi.mock("@/systems/settings/hooks/use-settings-sections", () => ({
  useSettingsAutomation: () => ({ data: { runtime: mocks.runtime } }),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspace: mocks.activeWorkspaceId ? { id: "ws_launch_hq", name: "checkout-api" } : null,
    activeWorkspaceId: mocks.activeWorkspaceId,
    isLoading: false,
    pending: false,
    workspaces: [{ id: "ws_launch_hq", name: "checkout-api", root_dir: "/tmp" }],
  }),
}));

const { mergeAutomationViews, soonestNextRun, useAutomationsPage } =
  await import("@/systems/os/apps/automation/use-automations-page");
const { AutomationApiError } = await import("@/systems/automation");

function page<T>(key: "jobs" | "triggers", items: T[], more = false) {
  return {
    [key]: items,
    page: {
      has_more: more,
      limit: 50,
      total: items.length + (more ? 1 : 0),
      ...(more ? { next_cursor: `${key}-2` } : {}),
    },
  };
}

function wrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const toView = (entity: AutomationJob | AutomationTrigger) => toAutomationView(entity);

describe("useAutomationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.runtime = { available: true };
    mocks.activeWorkspaceId = "ws_launch_hq";
    mocks.listJobs.mockResolvedValue(page("jobs", automationStoryJobs));
    mocks.listTriggers.mockResolvedValue(page("triggers", automationStoryTriggers));
  });

  it("Should merge both lists by name with totals, on count and soonest next run", async () => {
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.items).toHaveLength(7));

    expect(result.current.items.map(item => item.name)).toEqual([
      "dependency-review",
      "deploy-webhook",
      "morning-digest",
      "nightly-delivery",
      "release-checklist",
      "rerun-delivery",
      "summarize-failures",
    ]);
    expect(result.current.total).toBe(7);
    expect(result.current.counts).toEqual({ all: 7, schedule: 4, event: 3 });
    expect(result.current.enabledCount).toBe(6);
    expect(result.current.nextRunAt).toBe(
      automationStoryJobs.find(job => job.id === "release-checklist")?.next_run
    );
  });

  it("Should count zero events when only schedules exist", async () => {
    mocks.listTriggers.mockResolvedValue(page("triggers", []));
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.items).toHaveLength(4));
    expect(result.current.counts.event).toBe(0);
  });

  it("Should skip the other list for a Start view", async () => {
    const { result, rerender } = renderHook(
      ({ start }: { start: "schedule" | "event" }) => useAutomationsPage({ start }),
      { initialProps: { start: "schedule" }, wrapper: wrapper() }
    );
    await waitFor(() => expect(result.current.items).toHaveLength(4));
    expect(mocks.listTriggers).not.toHaveBeenCalled();

    rerender({ start: "event" });
    await waitFor(() => expect(result.current.items).toHaveLength(3));
    expect(mocks.listJobs).toHaveBeenCalledTimes(1);
  });

  it("Should send facets to both lists and never ask triggers for task targets", async () => {
    const { result } = renderHook(() => useAutomationsPage({ target: "loop", enabled: true }), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(mocks.listJobs.mock.calls[0]?.[0]).toMatchObject({ target: "loop", enabled: true });
    expect(mocks.listTriggers.mock.calls[0]?.[0]).toMatchObject({ target: "loop", enabled: true });

    vi.clearAllMocks();
    const tasks = renderHook(() => useAutomationsPage({ target: "task", start: "event" }), {
      wrapper: wrapper(),
    });
    await act(async () => {});
    expect(mocks.listJobs).not.toHaveBeenCalled();
    expect(mocks.listTriggers).not.toHaveBeenCalled();
    expect(tasks.result.current.items).toEqual([]);
  });

  it("Should load more only from the list that has more", async () => {
    mocks.listJobs.mockResolvedValueOnce(page("jobs", automationStoryJobs, true));
    mocks.listJobs.mockResolvedValueOnce(page("jobs", []));
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.canLoadMore).toBe(true));

    act(() => result.current.loadMore());
    await waitFor(() => expect(mocks.listJobs).toHaveBeenCalledTimes(2));
    expect(mocks.listJobs.mock.calls[1]?.[0]).toMatchObject({ cursor: "jobs-2" });
    expect(mocks.listTriggers).toHaveBeenCalledTimes(1);
  });

  it("Should keep loaded rows under a partial failure and blank that view count", async () => {
    mocks.listTriggers.mockRejectedValue(new AutomationApiError("boom", 500));
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.partialFailure).toBe("event"));
    expect(result.current.items).toHaveLength(4);
    expect(result.current.loadError).toBeNull();
    expect(result.current.counts.event).toBeNull();
  });

  it("Should fail the listing when both lists fail and name unavailability first", async () => {
    mocks.listJobs.mockRejectedValue(new AutomationApiError("jobs down", 500));
    mocks.listTriggers.mockRejectedValue(new AutomationApiError("triggers down", 500));
    const failed = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(failed.result.current.loadError).not.toBeNull());
    expect(failed.result.current.partialFailure).toBeNull();

    mocks.listJobs.mockRejectedValue(new AutomationApiError("unavailable", 503));
    const unavailable = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() =>
      expect(unavailable.result.current.unavailableMessage).toBe(
        "CompozyOS couldn't load your automations right now. Try again in a moment."
      )
    );
    expect(unavailable.result.current.loadError).toBeNull();
  });

  it("Should wait for the daemon before a switch flips and ignore a second flip", async () => {
    let resolvePatch: (value: unknown) => void = () => undefined;
    mocks.updateJob.mockReturnValue(new Promise(resolve => (resolvePatch = resolve)));
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.items).toHaveLength(7));
    const digest = result.current.items.find(item => item.id === "morning-digest")!;

    act(() => result.current.toggleEnabled(digest, false));
    act(() => result.current.toggleEnabled(digest, false));
    expect(result.current.isTogglePending(digest)).toBe(true);
    await waitFor(() => expect(mocks.updateJob).toHaveBeenCalledTimes(1));
    expect(result.current.items.find(item => item.id === "morning-digest")?.enabled).toBe(true);

    await act(async () => resolvePatch({ ...automationStoryJobs[0], enabled: false }));
    await waitFor(() => expect(result.current.isTogglePending(digest)).toBe(false));
    expect(mocks.toast.success).toHaveBeenCalledWith("Turned off morning-digest.");
  });

  it("Should toast and keep the state when turning off fails", async () => {
    mocks.updateJob.mockRejectedValue(new AutomationApiError("boom", 500));
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.items).toHaveLength(7));
    const digest = result.current.items.find(item => item.id === "morning-digest")!;

    await act(async () => result.current.toggleEnabled(digest, false));
    await waitFor(() =>
      expect(mocks.toast.error).toHaveBeenCalledWith("Couldn't turn off morning-digest. Try again.")
    );
    expect(result.current.items.find(item => item.id === "morning-digest")?.enabled).toBe(true);
  });

  it("Should block row actions while automations are unavailable", async () => {
    mocks.runtime = { available: false };
    const { result } = renderHook(() => useAutomationsPage({}), { wrapper: wrapper() });
    await waitFor(() => expect(result.current.items).toHaveLength(7));
    const digest = result.current.items.find(item => item.id === "morning-digest")!;

    act(() => result.current.runNow(digest));
    act(() => result.current.toggleEnabled(digest, false));
    expect(mocks.triggerJob).not.toHaveBeenCalled();
    expect(mocks.updateJob).not.toHaveBeenCalled();
  });

  it("Should open the editor for a create deep link and strip its params", async () => {
    const { result } = renderHook(() => useAutomationsPage({ create: "1", start: "event" }), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.editorDialogProps.editor).not.toBeNull());
    expect(result.current.editorDialogProps.editor?.draft.start).toBe("event");
    const strip = mocks.navigate.mock.calls.find(([call]) => call.replace === true)?.[0];
    expect(strip.search({ create: "1", start: "event", q: "x" })).toEqual({
      create: undefined,
      loop: undefined,
      start: undefined,
      q: "x",
    });
  });

  it("UT-106 opens a Loop seed with Does fixed and never reopens once the params are gone", async () => {
    const { result, rerender } = renderHook(
      ({ search }: { search: Parameters<typeof useAutomationsPage>[0] }) =>
        useAutomationsPage(search),
      {
        initialProps: {
          search: { create: "loop", start: "event", loop: "software-delivery" } as const,
        },
        wrapper: wrapper(),
      }
    );
    await waitFor(() => expect(result.current.editorDialogProps.editor).not.toBeNull());
    expect(result.current.editorDialogProps.editor).toMatchObject({
      lockedLoop: "software-delivery",
      draft: {
        start: "event",
        target_kind: "loop",
        loop_target: expect.objectContaining({ loop_name: "software-delivery" }),
      },
    });
    const strip = mocks.navigate.mock.calls.find(([call]) => call.replace === true)?.[0];
    expect(
      strip.search({
        create: "loop",
        start: "event",
        loop: "software-delivery",
        scope: "workspace",
      })
    ).toEqual({ create: undefined, loop: undefined, start: undefined, scope: "workspace" });

    // The stripped URL arrives, then the operator closes the dialog: it stays closed.
    rerender({ search: {} as never });
    act(() => result.current.editorDialogProps.editor?.onCancel());
    rerender({ search: {} as never });
    expect(result.current.editorDialogProps.editor).toBeNull();
  });

  it("Should open a plain create link in Global, but hold a Loop seed until a project is active", async () => {
    mocks.activeWorkspaceId = null;
    const created = renderHook(() => useAutomationsPage({ create: "1", start: "schedule" }), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(created.result.current.editorDialogProps.editor).not.toBeNull());
    expect(created.result.current.editorDialogProps.editor?.draft).toMatchObject({
      start: "schedule",
      scope: "global",
    });
    created.unmount();
    mocks.navigate.mockClear();

    const seeded = renderHook(
      () => useAutomationsPage({ create: "loop", loop: "software-delivery" }),
      { wrapper: wrapper() }
    );
    await waitFor(() => expect(seeded.result.current.items).toHaveLength(7));
    expect(seeded.result.current.editorDialogProps.editor).toBeNull();
    expect(mocks.navigate.mock.calls.some(([call]) => call.replace === true)).toBe(false);
  });

  it("Should clear search, every facet and the Start view", async () => {
    const { result } = renderHook(
      () => useAutomationsPage({ start: "event", q: "digest", target: "loop", enabled: true }),
      { wrapper: wrapper() }
    );
    act(() => result.current.clearFilters());
    const update = mocks.navigate.mock.lastCall?.[0];
    expect(update.to).toBe("/automations");
    expect(update.search({ start: "event", q: "digest", target: "loop", view: "cards" })).toEqual({
      start: undefined,
      q: undefined,
      enabled: undefined,
      scope: undefined,
      source: undefined,
      target: undefined,
      loop: undefined,
      view: "cards",
    });
  });
});

describe("mergeAutomationViews", () => {
  it("Should keep a job and a trigger with one name apart, schedule first", () => {
    const job = { ...automationStoryJobs[0], id: "digest", name: "digest" };
    const trigger = { ...automationStoryTriggers[0], id: "digest", name: "digest" };
    const merged = mergeAutomationViews([job], [trigger], toView);
    expect(merged.map(view => view.detailPath)).toEqual([
      "/automations/jobs/digest",
      "/automations/triggers/digest",
    ]);
    expect(mergeAutomationViews([], [trigger], toView)[0]?.kind).toBe("trigger");
  });

  it("Should find no next run when no schedule is on", () => {
    const off = automationStoryJobs.map(job => ({ ...job, enabled: false }));
    expect(soonestNextRun(off.map(toView))).toBeNull();
  });
});
