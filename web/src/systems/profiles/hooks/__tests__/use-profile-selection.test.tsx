// Suite: global active profile selection
// Invariant: the selected profile (a concrete one or All profiles) is one global answer — switching
// workspaces or breadth never moves it, a switch persists only the Global slot, All profiles survives
// a reload while a concrete view defers to the remembered Global choice, and the aggregate creates
// into that remembered choice (issue #792). A window pinned to one profile reads only that profile,
// whatever the shell shows (issue #1183). A switch moves the remembered choice the aggregate acts as
// at once, so X → Y → All never acts as X while the persist and reread are in flight (issue #1194).
// Leaving All profiles keeps it for a reload after a failure only when the daemon refused the write
// (issue #1436).
// Owning layer: useActiveProfileView / useSwitchProfile / useProfileReadScope.
// Boundary IN: the real profile view and active workspace stores.
// Boundary OUT: profile HTTP adapters (mocked I/O).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { flushStorage } from "@xstate/store/persist";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  disableAllProjects,
  enableAllProjects,
  focusUser,
  setActiveWorkspaceId,
} from "@/systems/workspace";

import {
  fetchProfileSelection,
  fetchProfiles,
  ProfileApiError,
  putProfileSelection,
} from "../../adapters/profiles-api";
import {
  localProfileView,
  profileViewStore,
  rehydrateProfileViewStore,
  resetProfileViews,
  setProfileView,
} from "../../stores/profile-view-store";
import { ProfilePin } from "../../components/profile-pin";
import { useProfileReadScope } from "../use-profile-read-scope";
import {
  useActiveProfileView,
  useCurrentProfileName,
  useProfilePerProfileScope,
  useSwitchProfile,
} from "../use-profile-selection";

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
    disableAllProjects();
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
      () => enableAllProjects(),
      () => disableAllProjects(),
      () => focusUser(),
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

  it("Should read a pinned window in its own profile under All profiles [#1183]", async () => {
    const { queryClient } = createWrapper();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        <ProfilePin profile="open-design">{children}</ProfilePin>
      </QueryClientProvider>
    );
    act(() => setProfileView({ kind: "aggregate" }));
    const { result } = renderHook(
      () => ({
        read: useProfileReadScope(),
        perProfile: useProfilePerProfileScope(),
        current: useCurrentProfileName(),
      }),
      { wrapper }
    );

    // The remembered choice settling to "marketing" must never move a pinned window's reads.
    await waitFor(() => expect(fetchProfileSelection).toHaveBeenCalled());
    await waitFor(() => expect(result.current.read.destinationOwner?.name).toBe("open-design"));
    expect(result.current.read.params).toEqual({ profile: "open-design" });
    expect(result.current.read.destination).toBe("open-design");
    expect(result.current.perProfile).toMatchObject({
      key: "open-design",
      names: ["open-design"],
      destination: "open-design",
    });
    expect(result.current.current).toBe("open-design");
    expect(localProfileView()).toEqual({ kind: "aggregate" });
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

  it("Should act as the switched-to profile under All profiles before the persist settles [#1194]", async () => {
    const { queryClient, wrapper } = createWrapper();
    let settlePut: () => void = () => {};
    vi.mocked(putProfileSelection).mockImplementation(
      params =>
        new Promise(resolve => {
          settlePut = () => resolve(params as never);
        })
    );
    const { result } = renderHook(
      () => ({ read: useProfileReadScope(), switchProfile: useSwitchProfile() }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.read.destination).toBe("marketing"));

    // X → Y → All with no pause: All lands before the write has even started.
    act(() => {
      result.current.switchProfile.mutate({ kind: "profile", profile: "open-design" });
      result.current.switchProfile.mutate({ kind: "aggregate" });
    });
    await waitFor(() => expect(result.current.read.aggregate).toBe(true));
    expect(result.current.read.destination).toBe("open-design");

    // The aggregate rereads the choice while the write is pending; the daemon still answers
    // "marketing", and that answer must not bring it back.
    await act(() => queryClient.refetchQueries({ queryKey: ["profiles", "selection", "global"] }));
    expect(result.current.read.destination).toBe("open-design");

    vi.mocked(fetchProfileSelection).mockResolvedValue({ scope: "global", profile: "open-design" });
    act(() => settlePut());
    await waitFor(() =>
      expect(queryClient.getQueryData(["profiles", "selection", "global"])).toEqual({
        scope: "global",
        profile: "open-design",
      })
    );
    expect(result.current.read.destination).toBe("open-design");
  });

  it("Should keep a later switch when an earlier one fails to persist [#1194]", async () => {
    const { wrapper } = createWrapper();
    let failFirst: () => void = () => {};
    vi.mocked(putProfileSelection)
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            failFirst = () => reject(new Error("daemon unavailable"));
          })
      )
      .mockImplementation(async params => params as never);
    const { result } = renderHook(() => useSwitchProfile(), { wrapper });

    act(() => {
      result.current.mutate({ kind: "profile", profile: "open-design" });
      result.current.mutate({ kind: "profile", profile: "default" });
    });
    // Writes run in switch order: the newer one waits for the older one to settle.
    await waitFor(() => expect(putProfileSelection).toHaveBeenCalledTimes(1));
    act(() => failFirst());

    await waitFor(() => expect(putProfileSelection).toHaveBeenCalledTimes(2));
    expect(putProfileSelection).toHaveBeenLastCalledWith({ scope: "global", profile: "default" });
    await waitFor(() => expect(result.current.isPending).toBe(false));
    expect(localProfileView()).toEqual({ kind: "profile", profile: "default" });
  });

  it("Should act as the newest switch under All profiles while an older write is pending [#1194]", async () => {
    const { wrapper } = createWrapper();
    let settleFirst: () => void = () => {};
    vi.mocked(putProfileSelection)
      .mockImplementationOnce(
        params =>
          new Promise(resolve => {
            settleFirst = () => resolve(params as never);
          })
      )
      .mockImplementation(async params => params as never);
    const { result } = renderHook(
      () => ({ read: useProfileReadScope(), switchProfile: useSwitchProfile() }),
      { wrapper }
    );
    await waitFor(() => expect(result.current.read.destination).toBe("marketing"));

    act(() => {
      result.current.switchProfile.mutate({ kind: "profile", profile: "open-design" });
      result.current.switchProfile.mutate({ kind: "profile", profile: "default" });
      result.current.switchProfile.mutate({ kind: "aggregate" });
    });
    await waitFor(() => expect(result.current.read.aggregate).toBe(true));
    // The older write still pending must not stand in for the newer choice.
    expect(result.current.read.destination).toBe("default");

    vi.mocked(fetchProfileSelection).mockResolvedValue({ scope: "global", profile: "default" });
    act(() => settleFirst());
    await waitFor(() => expect(putProfileSelection).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.read.destination).toBe("default"));
  });

  it("Should restore the remembered choice when the switch cannot be persisted [#1194]", async () => {
    const { queryClient, wrapper } = createWrapper();
    vi.mocked(putProfileSelection).mockRejectedValue(new Error("daemon unavailable"));
    const { result } = renderHook(
      () => ({ read: useProfileReadScope(), switchProfile: useSwitchProfile() }),
      { wrapper }
    );
    act(() => setProfileView({ kind: "aggregate" }));
    await waitFor(() => expect(result.current.read.destination).toBe("marketing"));

    await act(async () => {
      await result.current.switchProfile
        .mutateAsync({ kind: "profile", profile: "open-design" })
        .catch(() => undefined);
    });

    expect(localProfileView()).toEqual({ kind: "aggregate" });
    expect(queryClient.getQueryData(["profiles", "selection", "global"])).toEqual({
      scope: "global",
      profile: "marketing",
    });
    // The failed write stops counting as the remembered choice once it settles.
    await waitFor(() => expect(result.current.read.destination).toBe("marketing"));
  });

  it.each([
    {
      label: "refused",
      error: new ProfileApiError("profile is archived", 409, "", ""),
      kept: true,
    },
    { label: "lost its answer", error: new TypeError("Failed to fetch"), kept: false },
    {
      label: "sent a gateway error for",
      error: new ProfileApiError("bad gateway", 502, "", ""),
      kept: false,
    },
  ])(
    "Should keep All profiles for a reload only when the daemon $label the switch [#1436]",
    async ({ error, kept }) => {
      const { wrapper } = createWrapper();
      vi.mocked(putProfileSelection).mockRejectedValue(error);
      const { result } = renderHook(() => useSwitchProfile(), { wrapper });
      act(() => setProfileView({ kind: "aggregate" }));

      await act(async () => {
        await result.current
          .mutateAsync({ kind: "profile", profile: "marketing" })
          .catch(() => undefined);
      });

      expect(localProfileView()).toEqual({ kind: "aggregate" });
      flushStorage(profileViewStore);
      const persisted = JSON.parse(window.localStorage.getItem("compozy:profile-view") ?? "{}");
      expect(persisted.context.view).toEqual(kept ? { kind: "aggregate" } : null);
    }
  );

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
