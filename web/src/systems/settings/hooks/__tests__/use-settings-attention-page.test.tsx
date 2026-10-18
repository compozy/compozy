// Suite: live attention settings writes and reconciliation.
// Invariant: channel writes preserve current server-owned mutes, pending writes lock the page,
// and a removed workspace causes a canonical policy reread.
// Owning layer: Settings Attention page model. Mocks stop at HTTP adapters and browser permission.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, renderHook, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { profileFixtures } from "@/systems/profiles/mocks/fixtures";
import { resetProfileViews, setProfileView } from "@/systems/profiles";
import { statusFixture } from "@/systems/status/mocks";
import {
  enableGlobalScope,
  useWorkspaces,
  workspaceKeys,
  type WorkspacePayload,
} from "@/systems/workspace";

import {
  getSettingsAttention,
  listSettingsApplyRecords,
  SettingsApiError,
  updateSettingsAttention,
} from "../../adapters/settings-api";
import type { SettingsAttentionSection, SettingsMutationResult } from "../../types";
import { settingsAppliedMutationFixture } from "../../mocks/fixtures";
import { resetSettingsRestartStore } from "../../stores/use-settings-restart-store";
import { useSettingsAttentionPage } from "../use-settings-attention-page";

const api = vi.hoisted(() => ({
  fetchProfiles: vi.fn(),
  fetchProfileSelection: vi.fn(),
  fetchStatus: vi.fn(),
  fetchWorkspaces: vi.fn(),
}));
vi.mock("../../adapters/settings-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/settings-api")>()),
  getSettingsAttention: vi.fn(),
  updateSettingsAttention: vi.fn(),
  listSettingsApplyRecords: vi.fn(),
}));
vi.mock("@/systems/profiles/adapters/profiles-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles/adapters/profiles-api")>()),
  fetchProfiles: api.fetchProfiles,
  fetchProfileSelection: api.fetchProfileSelection,
}));
vi.mock("@/systems/status/adapters/daemon-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/status/adapters/daemon-api")>()),
  fetchStatus: api.fetchStatus,
}));
vi.mock("@/systems/workspace/adapters/workspace-api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace/adapters/workspace-api")>()),
  fetchWorkspaces: api.fetchWorkspaces,
}));

type AttentionConfig = SettingsAttentionSection["config"];
const storedConfig: AttentionConfig = {
  toasts: true,
  sound: true,
  system: false,
  muted_workspaces: [],
};
const removedWorkspace: WorkspacePayload = {
  id: "ws_0123456789abcdef",
  name: "Archive desk",
  root_dir: "/studio/archive",
  add_dirs: [],
  created_at: "2026-10-04T12:00:00Z",
  updated_at: "2026-10-04T12:00:00Z",
};
const keptWorkspace: WorkspacePayload = {
  ...removedWorkspace,
  id: "ws_abcdef0123456789",
  name: "Review desk",
  root_dir: "/studio/review",
};
const mutationResult: SettingsMutationResult = {
  ...settingsAppliedMutationFixture,
  section: "attention",
  scope: "profile",
  profile: "marketing",
};
let serverConfigs: Record<string, AttentionConfig>;
let workspaceCatalog: WorkspacePayload[];
const clients: QueryClient[] = [];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
}

async function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  function Wrapper({ children }: { children: ReactNode }) {
    const [router] = useState(() =>
      createRouter({
        routeTree: createRootRoute({ component: () => children }),
        history: createMemoryHistory({ initialEntries: ["/"] }),
      })
    );
    return (
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );
  }
  const view = renderHook(
    () => {
      useWorkspaces();
      return useSettingsAttentionPage();
    },
    { wrapper: Wrapper }
  );
  await waitFor(() => expect(view.result.current?.config).not.toBeNull());
  await waitFor(() => expect(view.result.current?.isLoading).toBe(false));
  return { client, ...view };
}

describe("useSettingsAttentionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetSettingsRestartStore();
    resetProfileViews();
    enableGlobalScope();
    setProfileView({ scope: "global" }, { kind: "profile", profile: "marketing" });
    serverConfigs = {
      marketing: structuredClone(storedConfig),
      consulting: structuredClone(storedConfig),
    };
    workspaceCatalog = [removedWorkspace, keptWorkspace];
    api.fetchProfiles.mockResolvedValue(profileFixtures);
    api.fetchProfileSelection.mockResolvedValue({ scope: "global", profile: "marketing" });
    api.fetchStatus.mockResolvedValue(statusFixture);
    api.fetchWorkspaces.mockImplementation(async () => structuredClone(workspaceCatalog));
    vi.mocked(listSettingsApplyRecords).mockResolvedValue({ entries: [] });
    vi.mocked(getSettingsAttention).mockImplementation(async filter => ({
      section: "attention",
      scope: filter.scope ?? "user",
      profile: filter.profile ?? "default",
      available_scopes: ["user", "profile"],
      config: structuredClone(serverConfigs[filter.profile ?? "default"] ?? storedConfig),
    }));
    vi.mocked(updateSettingsAttention).mockImplementation(async (body, filter) => {
      const profile = filter?.profile ?? "default";
      const mutes = body.config.muted_workspaces;
      if (mutes?.some(id => !workspaceCatalog.some(workspace => workspace.id === id))) {
        throw new SettingsApiError("workspace not found", 404);
      }
      serverConfigs[profile] = {
        ...body.config,
        muted_workspaces: mutes ?? serverConfigs[profile].muted_workspaces,
      };
      return mutationResult;
    });
  });

  afterEach(() => {
    clients.splice(0).forEach(client => client.clear());
    vi.unstubAllGlobals();
  });

  it("Should display and lock the full config currently being written", async () => {
    const pending = deferred<SettingsMutationResult>();
    vi.mocked(updateSettingsAttention).mockReturnValue(pending.promise);
    const { result } = await renderPage();

    act(() => result.current.setToasts(false));
    await waitFor(() => expect(result.current.isSaving).toBe(true));
    expect(result.current.config?.toasts).toBe(false);
    expect(getSettingsAttention).toHaveBeenCalledWith(
      { scope: "profile", profile: "marketing" },
      expect.any(AbortSignal)
    );
    act(() => result.current.setSound(false));
    expect(updateSettingsAttention).toHaveBeenCalledOnce();
    await act(async () => pending.resolve(mutationResult));
  });

  it("Should expose the failed write without replacing daemon config", async () => {
    vi.mocked(updateSettingsAttention).mockRejectedValue(
      new SettingsApiError("write rejected", 500)
    );
    const { result } = await renderPage();

    act(() => result.current.setSound(false));
    await waitFor(() => expect(result.current.saveError).toBe("write rejected"));
    expect(result.current.config).toEqual(storedConfig);
  });

  it("Should not display another profile's pending attention candidate", async () => {
    const pending = deferred<SettingsMutationResult>();
    vi.mocked(updateSettingsAttention).mockReturnValue(pending.promise);
    const { result } = await renderPage();

    act(() => result.current.setToasts(false));
    await waitFor(() => expect(result.current.isSaving).toBe(true));
    act(() => setProfileView({ scope: "global" }, { kind: "profile", profile: "consulting" }));
    await waitFor(() => expect(result.current.config).toEqual(storedConfig));
    expect(result.current.isSaving).toBe(true);
    await act(async () => pending.resolve(mutationResult));
  });

  it("Should write workspace mutes to the active profile", async () => {
    const { result } = await renderPage();

    act(() => result.current.muteWorkspace(removedWorkspace.id));
    await waitFor(() =>
      expect(result.current.config?.muted_workspaces).toEqual([removedWorkspace.id])
    );
    expect(updateSettingsAttention).toHaveBeenCalledWith(
      { config: { ...storedConfig, muted_workspaces: [removedWorkspace.id] } },
      { scope: "profile", profile: "marketing" }
    );
    expect(serverConfigs.consulting.muted_workspaces).toEqual([]);
  });

  it.each(["toasts", "sound", "system"] as const)(
    "Should change %s without replacing mutes from a stale page",
    async channel => {
      serverConfigs.marketing.muted_workspaces = [removedWorkspace.id];
      const { result } = await renderPage();
      expect(result.current.config?.muted_workspaces).toEqual([removedWorkspace.id]);
      serverConfigs.marketing.muted_workspaces = [];
      workspaceCatalog = [keptWorkspace];
      vi.stubGlobal("Notification", {
        permission: "granted",
        requestPermission: vi.fn().mockResolvedValue("granted"),
      });

      act(() => {
        if (channel === "toasts") result.current.setToasts(false);
        if (channel === "sound") result.current.setSound(false);
        if (channel === "system") result.current.setSystem(true);
      });
      const expected = {
        ...storedConfig,
        [channel]: channel === "system",
        muted_workspaces: [],
      };
      await waitFor(() => expect(result.current.config).toEqual(expected));
      expect(result.current.saveError).toBeNull();
      expect(serverConfigs.marketing).toEqual(expected);
      expect(serverConfigs.consulting).toEqual(storedConfig);
    }
  );

  it("Should reread the profile policy when the workspace catalog removes a muted project", async () => {
    serverConfigs.marketing.muted_workspaces = [removedWorkspace.id, keptWorkspace.id];
    const { client, result } = await renderPage();
    await waitFor(() => expect(api.fetchWorkspaces).toHaveBeenCalled());
    const reads = vi.mocked(getSettingsAttention).mock.calls.length;
    serverConfigs.marketing.muted_workspaces = [keptWorkspace.id];
    workspaceCatalog = [keptWorkspace];

    await act(async () => {
      await client.refetchQueries({ queryKey: workspaceKeys.list() });
    });

    await waitFor(() =>
      expect(result.current.config?.muted_workspaces).toEqual([keptWorkspace.id])
    );
    expect(getSettingsAttention).toHaveBeenCalledTimes(reads + 1);
    expect(serverConfigs.consulting).toEqual(storedConfig);
  });
});
