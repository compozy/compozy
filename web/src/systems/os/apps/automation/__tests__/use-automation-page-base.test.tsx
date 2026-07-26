// Suite: Automation list route state
// Invariant: Shared Jobs/Triggers controls serialize filters without erasing the selected view.
// Boundary IN: useAutomationPageBase setters and current TanStack Router search.
// Boundary OUT: Route validation and automation list transport.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ navigate: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useChildMatches: () => [],
  useNavigate: () => mocks.navigate,
}));

vi.mock("@/systems/automation", () => ({
  AutomationApiError: class AutomationApiError extends Error {
    status = 500;
  },
}));

vi.mock("@/systems/settings", () => ({
  useSettingsAutomation: () => ({ data: { runtime: { available: true } } }),
}));

vi.mock("@/systems/workspace", () => ({
  toWorkspaceCommandSelectOptions: () => [],
  useActiveWorkspace: () => ({
    activeWorkspace: null,
    activeWorkspaceId: "ws-a",
    workspaces: [],
  }),
}));

import {
  automationListLoopFilter,
  useAutomationCreateSeed,
  useAutomationPageBase,
} from "@/systems/os/apps/automation/use-automation-page-base";

describe("useAutomationPageBase route state", () => {
  beforeEach(() => vi.clearAllMocks());

  it("Should write every list control and clear only filters", () => {
    const current = {
      enabled: true,
      event: "task.created",
      loop: "daily",
      q: "audit",
      scope: "workspace" as const,
      source: "dynamic" as const,
      view: "cards" as const,
    };
    const { result } = renderHook(() => useAutomationPageBase("triggers", current));
    const updatedSearch = () => mocks.navigate.mock.lastCall?.[0].search(current);

    act(() => result.current.setSearchQuery("  review  "));
    expect(updatedSearch()).toEqual({ ...current, q: "review" });
    act(() => result.current.setScopeFilter("global"));
    expect(updatedSearch()).toEqual({ ...current, scope: "global" });
    act(() => result.current.setSourceFilter("package"));
    expect(updatedSearch()).toEqual({ ...current, source: "package" });
    act(() => result.current.setEnabledFilter(false));
    expect(updatedSearch()).toEqual({ ...current, enabled: false });
    act(() => result.current.setEventFilter("  session.end  "));
    expect(updatedSearch()).toEqual({ ...current, event: "session.end" });
    act(() => result.current.setView("rows"));
    expect(updatedSearch()).toEqual({ ...current, view: undefined });

    act(() => result.current.clearFilters());
    expect(updatedSearch()).toEqual({
      ...current,
      enabled: undefined,
      event: undefined,
      q: undefined,
      scope: undefined,
      source: undefined,
    });
    expect(mocks.navigate.mock.lastCall?.[0].to).toBe("/triggers");
  });

  it.each([
    ["jobs", "/jobs"],
    ["triggers", "/triggers"],
  ] as const)("Should consume loop seeds and preserve unrelated %s search", (kind, target) => {
    const openLoopCreate = vi.fn();
    const { rerender } = renderHook(
      ({ loop }: { loop?: string }) =>
        useAutomationCreateSeed(kind, { loop }, "ws-a", openLoopCreate),
      { initialProps: { loop: "daily-review" as string | undefined } }
    );

    expect(openLoopCreate).toHaveBeenCalledWith("daily-review");
    rerender({ loop: undefined });
    rerender({ loop: "release-review" });

    expect(openLoopCreate.mock.calls).toEqual([["daily-review"], ["release-review"]]);
    expect(mocks.navigate).toHaveBeenCalledTimes(2);
    for (const [navigation] of mocks.navigate.mock.calls) {
      const current = {
        create: "loop" as const,
        loop: "daily-review",
        q: "audit",
        scope: "workspace" as const,
      };
      expect(navigation.to).toBe(target);
      expect(navigation.replace).toBe(true);
      expect(navigation.search(current)).toEqual({
        ...current,
        create: undefined,
        loop: undefined,
      });
    }
  });

  it("Should keep one-shot Loop create seeds out of catalog filters", () => {
    expect(automationListLoopFilter({ create: "loop", loop: "daily-review" })).toBeUndefined();
    expect(automationListLoopFilter({ loop: "daily-review" })).toBe("daily-review");
  });
});
