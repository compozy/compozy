import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { extensionFixtures } from "../../mocks/fixtures";
import { extensionKeys } from "../../lib/query-keys";
import { marketplaceKeys } from "@/systems/marketplace/lib/query-keys";

const mocks = vi.hoisted(() => ({
  deactivateBundle: vi.fn(),
  disableExtension: vi.fn(),
  enableExtension: vi.fn(),
  removeExtension: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  updateBundleActivation: vi.fn(),
  updateExtension: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));

vi.mock("../../adapters/extensions-api", () => ({
  deactivateBundle: mocks.deactivateBundle,
  disableExtension: mocks.disableExtension,
  enableExtension: mocks.enableExtension,
  removeExtension: mocks.removeExtension,
  updateBundleActivation: mocks.updateBundleActivation,
  updateExtension: mocks.updateExtension,
}));

import {
  useDeactivateBundle,
  useRemoveExtension,
  useToggleExtension,
  useUpdateBundleActivation,
  useUpdateExtension,
} from "../use-extension-actions";

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.enableExtension.mockResolvedValue(extensionFixtures[0]);
  mocks.disableExtension.mockResolvedValue(extensionFixtures[0]);
  mocks.updateExtension.mockResolvedValue(undefined);
  mocks.removeExtension.mockResolvedValue(undefined);
  mocks.updateBundleActivation.mockResolvedValue({
    bundle_name: "ops-starter",
    created_at: "2026-07-08T09:20:00Z",
    extension_name: "slack-notify",
    id: "activation-ops-starter",
    profile_name: "production",
    scope: "workspace",
    spec_drift: false,
    updated_at: "2026-07-14T18:00:00Z",
    workspace_id: "ws_northstar",
  });
  mocks.deactivateBundle.mockResolvedValue(undefined);
});

describe("useToggleExtension", () => {
  it("Should roll an optimistic toggle back and toast the daemon error", async () => {
    const { queryClient, wrapper } = setup();
    const original = extensionFixtures.map(extension => ({ ...extension }));
    queryClient.setQueryData(extensionKeys.list(), original);
    let rejectDisable: ((error: Error) => void) | undefined;
    mocks.disableExtension.mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectDisable = reject;
      })
    );
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    act(() => result.current.mutate({ enabled: false, name: "otel-bridge" }));

    await waitFor(() =>
      expect(queryClient.getQueryData<typeof original>(extensionKeys.list())?.[0]?.enabled).toBe(
        false
      )
    );
    act(() => rejectDisable?.(new Error("daemon refused the toggle")));

    await waitFor(() =>
      expect(queryClient.getQueryData<typeof original>(extensionKeys.list())?.[0]?.enabled).toBe(
        true
      )
    );
    expect(mocks.toastError).toHaveBeenCalledWith("daemon refused the toggle");
  });

  it("Should enable through the daemon and invalidate inventory without cached rollback state", async () => {
    const { queryClient, wrapper } = setup();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ enabled: true, name: "otel-bridge" });
    });

    expect(mocks.enableExtension).toHaveBeenCalledWith("otel-bridge");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: extensionKeys.list() });
  });
});

describe("useUpdateBundleActivation", () => {
  it("Should send explicit network confirmation in the real PATCH mutation", async () => {
    const { wrapper } = setup();
    const activation = {
      bundle_name: "ops-starter",
      created_at: "2026-07-08T09:20:00Z",
      extension_name: "slack-notify",
      id: "activation-ops-starter",
      profile_name: "production",
      scope: "workspace",
      spec_drift: false,
      updated_at: "2026-07-14T18:00:00Z",
      version: 7,
      workspace_id: "ws_northstar",
    };
    mocks.updateBundleActivation.mockResolvedValue(activation);
    const { result } = renderHook(() => useUpdateBundleActivation(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        body: { confirm_network_requirement: true, expected_version: 7 },
        id: activation.id,
      });
    });

    expect(mocks.updateBundleActivation).toHaveBeenCalledWith(activation.id, {
      confirm_network_requirement: true,
      expected_version: 7,
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("ops-starter updated");
  });

  it("Should toast a bundle update error and still invalidate bundle identities", async () => {
    const { queryClient, wrapper } = setup();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    mocks.updateBundleActivation.mockRejectedValue(new Error("bundle update rejected"));
    const { result } = renderHook(() => useUpdateBundleActivation(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          body: { expected_version: 7 },
          id: "activation-ops-starter",
        })
      ).rejects.toThrow("bundle update rejected");
    });

    expect(mocks.toastError).toHaveBeenCalledWith("bundle update rejected");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: extensionKeys.bundles() });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: extensionKeys.bundle("activation-ops-starter"),
    });
  });
});

describe("extension and bundle lifecycle mutations", () => {
  it("Should reconcile Marketplace discovery after an extension update", async () => {
    const { queryClient, wrapper } = setup();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("otel-bridge");
    });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: extensionKeys.all });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: marketplaceKeys.all });
  });

  it("Should toast successful lifecycle actions without leaking mutation context", async () => {
    const { wrapper } = setup();
    const { result } = renderHook(
      () => ({
        deactivate: useDeactivateBundle(),
        remove: useRemoveExtension(),
        update: useUpdateExtension(),
      }),
      { wrapper }
    );

    await act(async () => {
      await result.current.update.mutateAsync("otel-bridge");
      await result.current.remove.mutateAsync("slack-notify");
      await result.current.deactivate.mutateAsync("activation-ops-starter");
    });

    expect(mocks.updateExtension).toHaveBeenCalledWith("otel-bridge", {});
    expect(mocks.removeExtension).toHaveBeenCalledWith("slack-notify");
    expect(mocks.deactivateBundle).toHaveBeenCalledWith("activation-ops-starter");
    expect(mocks.toastSuccess).toHaveBeenCalledWith("otel-bridge updated");
    expect(mocks.toastSuccess).toHaveBeenCalledWith("slack-notify removed");
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Bundle deactivated");
  });

  it("Should surface extension removal and bundle deactivation errors", async () => {
    const { wrapper } = setup();
    mocks.removeExtension.mockRejectedValue(new Error("remove rejected"));
    mocks.deactivateBundle.mockRejectedValue(new Error("deactivate rejected"));
    const { result } = renderHook(
      () => ({ deactivate: useDeactivateBundle(), remove: useRemoveExtension() }),
      { wrapper }
    );

    await act(async () => {
      await expect(result.current.remove.mutateAsync("slack-notify")).rejects.toThrow(
        "remove rejected"
      );
      await expect(result.current.deactivate.mutateAsync("activation-ops-starter")).rejects.toThrow(
        "deactivate rejected"
      );
    });

    expect(mocks.toastError).toHaveBeenCalledWith("remove rejected");
    expect(mocks.toastError).toHaveBeenCalledWith("deactivate rejected");
  });
});
