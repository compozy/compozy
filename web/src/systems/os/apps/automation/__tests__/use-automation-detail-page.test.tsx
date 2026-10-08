// Suite: Automation detail page hook
// Invariants: one projection serves jobs and triggers; workspace boundaries and 404s map to
// explicit states; Run now, On/Off and Delete report through toasts without optimistic state;
// a delete that finds nothing is success; Back restores the listing search; the one-shot
// `edit=options` deep link opens the editor once and leaves the URL.
// Owning layer: useAutomationDetailPage, which binds the panel to TanStack Query and routing.
import { act, renderHook } from "@testing-library/react";
import { toast } from "sonner";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  AutomationApiError,
  automationDetailSearchFrom,
  validateAutomationDetailSearch,
} from "@/systems/automation";
import { LoopsApiError } from "@/systems/loops";
import {
  dependencyReviewJob,
  makeDetailRun,
  morningDigestJob,
  rerunDeliveryTrigger,
} from "@/systems/automation/mocks/detail-fixtures";

const state = vi.hoisted(() => ({
  activeWorkspaceId: "ws_checkout_api" as string | null,
  navigate: vi.fn(),
  job: { data: undefined as unknown, error: null as unknown, isLoading: false },
  trigger: { data: undefined as unknown, error: null as unknown, isLoading: false },
  runs: [] as unknown[],
  loopError: null as unknown,
  update: vi.fn(),
  remove: vi.fn(),
  runNow: vi.fn(),
  openEdit: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => state.navigate }));
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock("@/systems/os/hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => true,
}));
vi.mock("@/systems/settings", () => ({
  useSettingsAutomation: () => ({
    data: { config: { timezone: "UTC" }, runtime: { available: true } },
  }),
}));
vi.mock("@/systems/workspace", () => ({
  toWorkspaceCommandSelectOptions: () => [],
  useActiveWorkspace: () => ({
    activeWorkspaceId: state.activeWorkspaceId,
    isLoading: false,
    workspaces: [{ id: "ws_checkout_api", name: "checkout-api" }],
  }),
}));
vi.mock("@/systems/loops", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/loops")>()),
  useLoop: () => ({ error: state.loopError }),
}));
vi.mock("@/systems/automation", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/automation")>();
  const mutation = (fn: typeof state.update) => ({ isPending: false, mutateAsync: fn });
  const runsQuery = () => ({ data: state.runs, error: null, isLoading: false, refetch: vi.fn() });
  return {
    ...actual,
    useAutomationJob: () => state.job,
    useAutomationTrigger: () => state.trigger,
    useAutomationJobRuns: runsQuery,
    useAutomationTriggerRuns: runsQuery,
    useAutomationEditor: () => ({ editorDialogProps: {}, openEdit: state.openEdit }),
    useUpdateAutomationJob: () => mutation(state.update),
    useUpdateAutomationTrigger: () => mutation(state.update),
    useDeleteAutomationJob: () => mutation(state.remove),
    useDeleteAutomationTrigger: () => mutation(state.remove),
    useTriggerAutomationJob: () => mutation(state.runNow),
  };
});

const { useAutomationDetailPage } = await import("../use-automation-detail-page");

describe("useAutomationDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.activeWorkspaceId = "ws_checkout_api";
    state.job = { data: morningDigestJob, error: null, isLoading: false };
    state.trigger = { data: rerunDeliveryTrigger, error: null, isLoading: false };
    state.runs = [];
    state.loopError = null;
  });

  it("Should project a job into one ready view with its sentence and next run", () => {
    const { result } = renderHook(() => useAutomationDetailPage("job", "morning-digest"));

    expect(result.current.panel.status).toBe("ready");
    expect(result.current.panel.view).toMatchObject({
      kind: "job",
      canRunNow: true,
      nextRunAt: "2026-10-08T09:00:00Z",
      workspaceName: "checkout-api",
    });
  });

  it("Should map another project, a 404 and other failures to their own states (UT-080)", () => {
    state.activeWorkspaceId = "ws_other";
    const elsewhere = renderHook(() => useAutomationDetailPage("trigger", "rerun-delivery"));
    expect(elsewhere.result.current.panel.status).toBe("elsewhere");
    expect(elsewhere.result.current.panel.statusMessage).toBe(
      "This automation belongs to another project. Switch to it to open this page."
    );
    expect(elsewhere.result.current.panel.runs).toEqual([]);

    state.activeWorkspaceId = "ws_checkout_api";
    state.job = {
      data: undefined,
      error: new AutomationApiError("not found", 404),
      isLoading: false,
    };
    const missing = renderHook(() => useAutomationDetailPage("job", "ghost"));
    expect(missing.result.current.panel.status).toBe("missing");
    expect(missing.result.current.panel.statusMessage).toBe(
      "This automation is no longer available."
    );

    state.job = { data: undefined, error: new AutomationApiError("boom", 500), isLoading: false };
    const failed = renderHook(() => useAutomationDetailPage("job", "morning-digest"));
    expect(failed.result.current.panel.status).toBe("error");
    expect(failed.result.current.panel.statusMessage).toBe("boom");

    state.job = { data: undefined, error: null, isLoading: true };
    const loading = renderHook(() => useAutomationDetailPage("job", "morning-digest"));
    expect(loading.result.current.panel.status).toBe("loading");
  });

  it("Should return to the listing with its previous search on Back (UT-062)", () => {
    const { result } = renderHook(() =>
      useAutomationDetailPage("job", "morning-digest", {
        start: "schedule",
        q: "digest",
        target: "agent",
        view: "cards",
      })
    );

    act(() => result.current.panel.onBack());

    expect(state.navigate).toHaveBeenCalledWith({
      to: "/automations",
      search: { start: "schedule", q: "digest", target: "agent", view: "cards" },
    });
  });

  it("Should toast the new state after the daemon answers, and the failure otherwise (UT-061, UT-047)", async () => {
    const { result } = renderHook(() => useAutomationDetailPage("job", "morning-digest"));

    state.update.mockResolvedValueOnce(undefined);
    await act(async () => result.current.panel.onToggleEnabled(false));
    expect(state.update).toHaveBeenCalledWith({
      data: { enabled: false },
      id: "morning-digest",
      profile: "default",
    });
    expect(toast.success).toHaveBeenCalledWith("Turned off morning-digest.");

    state.update.mockRejectedValueOnce(new Error("Automation manager is not configured"));
    await act(async () => result.current.panel.onToggleEnabled(false));
    expect(toast.error).toHaveBeenCalledWith("Automation manager is not configured");
  });

  it("Should queue a Run now even while Off and insert the run first (UT-074)", async () => {
    state.job = { data: dependencyReviewJob, error: null, isLoading: false };
    state.runs = [makeDetailRun()];
    state.runNow.mockResolvedValueOnce(makeDetailRun({ id: "run_queued", status: "scheduled" }));
    const { result } = renderHook(() => useAutomationDetailPage("job", "dependency-review"));

    await act(async () => result.current.panel.onRunNow());

    expect(toast.success).toHaveBeenCalledWith("Queued run run_queued.");
    expect(result.current.panel.runs.map(run => run.id)).toEqual(["run_queued", "run_001"]);
  });

  it("Should show the daemon's refusal and add no run (UT-075)", async () => {
    state.runNow.mockRejectedValueOnce(new AutomationApiError("fire limit reached", 429));
    const { result } = renderHook(() => useAutomationDetailPage("job", "morning-digest"));

    await act(async () => result.current.panel.onRunNow());

    expect(toast.error).toHaveBeenCalledWith("fire limit reached");
    expect(result.current.panel.runs).toEqual([]);
  });

  it("Should treat an already-deleted automation as deleted and replace to the listing (UT-079)", async () => {
    state.remove.mockRejectedValueOnce(new AutomationApiError("not found", 404));
    const { result } = renderHook(() =>
      useAutomationDetailPage("trigger", "rerun-delivery", { start: "event" })
    );

    await act(async () => result.current.panel.onDelete());

    expect(toast.success).toHaveBeenCalledWith("Deleted rerun-delivery.");
    expect(state.navigate).toHaveBeenCalledWith({
      to: "/automations",
      search: { start: "event" },
      replace: true,
    });
  });

  it("Should keep a failed delete as an error for the dialog (UT-079)", async () => {
    state.remove.mockRejectedValueOnce(new AutomationApiError("Internal server error", 500));
    const { result } = renderHook(() => useAutomationDetailPage("trigger", "rerun-delivery"));

    await expect(result.current.panel.onDelete()).rejects.toThrow("Internal server error");
    expect(state.navigate).not.toHaveBeenCalled();
  });

  it("Should flag a Loop target that no longer exists (UT-069)", () => {
    state.loopError = new LoopsApiError("Loop not found: software-delivery", 404);
    const { result } = renderHook(() => useAutomationDetailPage("trigger", "rerun-delivery"));

    expect(result.current.panel.loopMissing).toBe(true);
    expect(result.current.panel.loopWorkspaceName).toBe("checkout-api");
  });

  it("Should open the editor once from edit=options and drop the param (UT-071)", () => {
    const search = { start: "schedule" as const, edit: "options" as const };
    const { rerender } = renderHook(() => useAutomationDetailPage("job", "morning-digest", search));
    rerender();

    expect(state.openEdit).toHaveBeenCalledOnce();
    expect(state.openEdit).toHaveBeenCalledWith(morningDigestJob, { section: "options" });
    expect(state.navigate).toHaveBeenCalledWith({
      to: "/automations/jobs/$jobId",
      params: { jobId: "morning-digest" },
      search: { start: "schedule", edit: undefined },
      replace: true,
    });
  });

  it("Should ask for Options through the route when Set up retries is chosen", () => {
    const { result } = renderHook(() =>
      useAutomationDetailPage("trigger", "rerun-delivery", { q: "delivery" })
    );

    act(() => result.current.panel.onSetUpRetries());

    expect(state.navigate).toHaveBeenCalledWith({
      to: "/automations/triggers/$triggerId",
      params: { triggerId: "rerun-delivery" },
      search: { q: "delivery", edit: "options" },
      replace: false,
    });
  });
});

describe("validateAutomationDetailSearch", () => {
  it("Should keep the listing state Back restores, accept the edit deep link and drop the rest", () => {
    expect(
      validateAutomationDetailSearch({
        start: "event",
        q: " digest ",
        view: "cards",
        edit: "options",
        create: "1",
        bogus: "x",
      })
    ).toEqual({ start: "event", q: "digest", view: "cards", edit: "options" });
    expect(validateAutomationDetailSearch({ edit: "everything" })).toEqual({});
  });

  it("Should carry the listing filters into a detail link but never the one-shot create seed (UT-062)", () => {
    expect(
      automationDetailSearchFrom({ start: "schedule", q: "digest", target: "agent", view: "cards" })
    ).toEqual({ start: "schedule", q: "digest", target: "agent", view: "cards" });
    expect(automationDetailSearchFrom({ create: "loop", loop: "software-delivery" })).toEqual({});
    expect(automationDetailSearchFrom({ loop: "software-delivery" })).toEqual({
      loop: "software-delivery",
    });
  });
});
