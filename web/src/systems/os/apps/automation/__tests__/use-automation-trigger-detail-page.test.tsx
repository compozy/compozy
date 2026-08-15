// Suite: Automation trigger detail page hook
// Invariant: the route-level Retry action refetches the trigger-run query for the routed trigger.
// Owning layer: useAutomationTriggerDetailPage, which binds the panel action to TanStack Query.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const queryState = vi.hoisted(() => ({
  refetchRuns: vi.fn(),
  trigger: {
    id: "trg_release",
    name: "release",
    agent_name: "reviewer",
    prompt: "Review release",
    event: "session.stopped",
    scope: "workspace" as const,
    workspace_id: "ws_alpha",
    source: "dynamic" as const,
    target_kind: "agent",
    enabled: true,
    retry: { strategy: "none" as const, max_retries: 0, base_delay: "" },
    fire_limit: { max: 4, window: "1h" },
    webhook_secret_present: false,
    created_at: "2026-08-15T10:00:00Z",
    updated_at: "2026-08-15T10:00:00Z",
  },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => vi.fn() }));
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
vi.mock("@/systems/os/hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => true,
}));
vi.mock("@/systems/workspace", () => ({
  toWorkspaceCommandSelectOptions: () => [],
  useActiveWorkspace: () => ({
    activeWorkspaceId: "ws_alpha",
    isLoading: false,
    workspaces: [{ id: "ws_alpha", name: "Alpha" }],
  }),
}));
vi.mock("@/systems/automation", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/automation")>();
  return {
    ...actual,
    useAutomationTrigger: () => ({ data: queryState.trigger, error: null, isLoading: false }),
    useAutomationTriggerEditor: () => ({ editorDialogProps: {}, openEdit: vi.fn() }),
    useAutomationTriggerRuns: () => ({
      data: [],
      error: null,
      isLoading: false,
      refetch: queryState.refetchRuns,
    }),
    useDeleteAutomationTrigger: () => ({ isPending: false, mutateAsync: vi.fn() }),
    useUpdateAutomationTrigger: () => ({ isPending: false, mutateAsync: vi.fn() }),
  };
});

const { useAutomationTriggerDetailPage } = await import("../use-automation-trigger-detail-page");

describe("useAutomationTriggerDetailPage", () => {
  beforeEach(() => {
    queryState.refetchRuns.mockReset();
  });

  it("Should refetch the routed trigger runs when Retry is requested", () => {
    const { result } = renderHook(() => useAutomationTriggerDetailPage("trg_release"));

    act(() => result.current.handleRetryRuns());

    expect(queryState.refetchRuns).toHaveBeenCalledOnce();
  });
});
