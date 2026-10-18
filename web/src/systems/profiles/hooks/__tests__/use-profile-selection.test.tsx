// Suite: global active profile selection
// Invariant: the selected profile (a concrete one or All profiles) is one global answer — switching
// workspaces or breadth never moves it, a switch persists only the Global slot, All profiles survives
// a reload while a concrete view defers to the remembered Global choice, and the aggregate creates
// into that remembered choice (issue #792).
// Owning layer: useActiveProfileView / useSwitchProfile / useProfileReadScope.
// Boundary IN: the real profile view and active workspace stores.
// Boundary OUT: profile HTTP adapters (mocked I/O).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { flushStorage } from "@xstate/store/persist";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { disableGlobalScope, enableGlobalScope, setActiveWorkspaceId } from "@/systems/workspace";

import {
  fetchProfileSelection,
  fetchProfiles,
  putProfileSelection,
} from "../../adapters/profiles-api";
import {
  localProfileView,
  profileViewStore,
  rehydrateProfileViewStore,
  resetProfileViews,
  setProfileView,
} from "../../stores/profile-view-store";
import { useProfileReadScope } from "../use-profile-read-scope";
import { useActiveProfileView, useSwitchProfile } from "../use-profile-selection";

vi.mock("../../adapters/profiles-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/profiles-api")>()),
  fetchProfileSelection: vi.fn(),
  fetchProfiles: vi.fn(),
  putProfileSelection: vi.fn(),
}));

const PROFILE_ROWS = [
  { id: "prof-default", name: "default", color: "#8E8EB5", state: "active" },
  { id: "prof-marketing", name: "marketing", color: "#FF7F3A", state: "active" },
  { id: "prof-design", name: "open-design", color: "#3A7FFF", state: "active" },
];

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("global profile selection", () => {
  beforeEach(() => {
    resetProfileViews();
    disableGlobalScope();
    setActiveWorkspaceId("ws-alpha");
    vi.mocked(fetchProfiles).mockResolvedValue(PROFILE_ROWS as never);
    vi.mocked(fetchProfileSelection).mockResolvedValue({ scope: "global", profile: "marketing" });
    vi.mocked(putProfileSelection).mockReset();
    vi.mocked(putProfileSelection).mockImplementation(async params => params as never);
  });

  it.each([
    { label: "a concrete profile", view: { kind: "profile", profile: "open-design" } as const },
    { label: "All profiles", view: { kind: "aggregate" } as const },
  ])("Should keep $label selected across workspace and breadth switches", async ({ view }) => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useProfileReadScope(), { wrapper });
    act(() => setProfileView(view));

    for (const move of [
      () => setActiveWorkspaceId("ws-beta"),
      () => enableGlobalScope(),
      () => disableGlobalScope(),
      () => setActiveWorkspaceId("ws-alpha"),
    ]) {
      act(move);
      expect(result.current.view).toEqual(view);
      expect(result.current.params).toEqual(
        view.kind === "aggregate" ? { all_profiles: true } : { profile: "open-design" }
      );
    }
  });

  it("Should create into the remembered Global profile while All profiles is selected", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useProfileReadScope(), { wrapper });
    act(() => setProfileView({ kind: "aggregate" }));

    await waitFor(() => expect(result.current.destination).toBe("marketing"));
    expect(result.current.aggregate).toBe(true);
    expect(result.current.scopeLabel).toBeNull();
    await waitFor(() => expect(result.current.destinationOwner?.name).toBe("marketing"));
  });

  it("Should persist a concrete switch in the Global slot only and never the aggregate", async () => {
    const { wrapper } = createWrapper();
    setActiveWorkspaceId("ws-beta");
    const { result } = renderHook(() => useSwitchProfile(), { wrapper });

    await act(() => result.current.mutateAsync({ kind: "profile", profile: "open-design" }));
    expect(putProfileSelection).toHaveBeenCalledExactlyOnceWith({
      scope: "global",
      profile: "open-design",
    });

    await act(() => result.current.mutateAsync({ kind: "aggregate" }));
    expect(putProfileSelection).toHaveBeenCalledTimes(1);
    expect(localProfileView()).toEqual({ kind: "aggregate" });
  });

  it("Should keep All profiles across a reload and resolve a concrete view from the daemon", async () => {
    setProfileView({ kind: "aggregate" });
    flushStorage(profileViewStore);
    const persisted = window.localStorage.getItem("compozy:profile-view");
    // Simulate a fresh document: in-memory state is gone, storage survives.
    profileViewStore.trigger.viewCleared();
    window.localStorage.setItem("compozy:profile-view", persisted ?? "");
    await rehydrateProfileViewStore();
    expect(localProfileView()).toEqual({ kind: "aggregate" });

    setProfileView({ kind: "profile", profile: "open-design" });
    flushStorage(profileViewStore);
    await rehydrateProfileViewStore();
    expect(localProfileView()).toBeNull();
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useActiveProfileView(), { wrapper });
    await waitFor(() => expect(result.current).toEqual({ kind: "profile", profile: "marketing" }));
  });
});
