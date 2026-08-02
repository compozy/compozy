import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { extensionFixtures } from "../../mocks/fixtures";
import { extensionKeys } from "../../lib/query-keys";
import { marketplaceKeys } from "@/systems/marketplace/lib/query-keys";

const mocks = vi.hoisted(() => ({
  activeWorkspaceId: null as string | null,
  disableExtension: vi.fn(),
  enableExtension: vi.fn(),
  removeExtension: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  updateExtension: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: mocks.activeWorkspaceId }),
}));

vi.mock("../../adapters/extensions-api", async () => {
  const actual = await vi.importActual<typeof import("../../adapters/extensions-api")>(
    "../../adapters/extensions-api"
  );
  return {
    ExtensionsApiError: actual.ExtensionsApiError,
    disableExtension: mocks.disableExtension,
    enableExtension: mocks.enableExtension,
    removeExtension: mocks.removeExtension,
    updateExtension: mocks.updateExtension,
  };
});

import { ExtensionsApiError } from "../../adapters/extensions-api";
import {
  useRemoveExtension,
  useToggleExtension,
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
  mocks.activeWorkspaceId = null;
  mocks.enableExtension.mockResolvedValue({
    automation_started: [],
    extension: extensionFixtures[0],
  });
  mocks.disableExtension.mockResolvedValue(extensionFixtures[0]);
  mocks.updateExtension.mockResolvedValue(undefined);
  mocks.removeExtension.mockResolvedValue(undefined);
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

  it("Should enable through the daemon and enumerate the automation it started", async () => {
    const { wrapper } = setup();
    mocks.enableExtension.mockResolvedValue({
      automation_started: ["weekly-audit", "dep-sweep"],
      extension: extensionFixtures[0],
    });
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ enabled: true, name: "dep-kit-ops" });
    });

    expect(mocks.enableExtension).toHaveBeenCalledWith("dep-kit-ops", {
      confirmNetworkDigest: undefined,
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("dep-kit-ops enabled · 2 automation started", {
      description: "dep-sweep, weekly-audit",
    });
  });

  it("Should carry the ratified digest on a confirmed enable retry", async () => {
    const { wrapper } = setup();
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        confirmNetworkDigest: "sha256:6f1c0a94d3b27e58",
        enabled: true,
        name: "dep-kit-ops",
      });
    });

    expect(mocks.enableExtension).toHaveBeenCalledWith("dep-kit-ops", {
      confirmNetworkDigest: "sha256:6f1c0a94d3b27e58",
    });
  });

  /**
   * A confirmation refusal is not an error the operator has to dismiss: the dialog owns it, so a
   * toast here would double-signal the same state.
   */
  it("Should stay silent for a network-confirmation refusal instead of toasting it", async () => {
    const { wrapper } = setup();
    mocks.enableExtension.mockRejectedValue(
      new ExtensionsApiError(
        "confirmation required",
        409,
        "daemon",
        "extension_network_confirmation_required",
        "sha256:6f1c0a94d3b27e58"
      )
    );
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({ enabled: true, name: "dep-kit-ops" })
      ).rejects.toThrow("confirmation required");
    });

    expect(mocks.toastError).not.toHaveBeenCalled();
  });

  /**
   * Enabling and disabling change which kit resources are live and whether confirmation is still
   * required, so a stale inventory would render badges the daemon no longer backs.
   */
  it.each([
    ["enable", true],
    ["disable", false],
  ])(
    "Should invalidate the cached kit inventory after a successful %s",
    async (_label, enabled) => {
      const { queryClient, wrapper } = setup();
      const inventoryKey = extensionKeys.inventory("dep-kit-ops");
      queryClient.setQueryData(inventoryKey, [
        { id: "weekly-audit", kind: "automation", live: !enabled, name: "weekly-audit" },
      ]);
      expect(queryClient.getQueryState(inventoryKey)?.isInvalidated).toBe(false);
      const { result } = renderHook(() => useToggleExtension(), { wrapper });

      await act(async () => {
        await result.current.mutateAsync({ enabled, name: "dep-kit-ops" });
      });

      await waitFor(() =>
        expect(queryClient.getQueryState(inventoryKey)?.isInvalidated).toBe(true)
      );
    }
  );

  it("Should apply the optimistic toggle to the active workspace instance row only", async () => {
    mocks.activeWorkspaceId = "ws_northstar";
    const { queryClient, wrapper } = setup();
    queryClient.setQueryData(
      extensionKeys.list("ws_northstar"),
      extensionFixtures.map(extension => ({ ...extension }))
    );
    queryClient.setQueryData(
      extensionKeys.list(),
      extensionFixtures.map(extension => ({ ...extension }))
    );
    const { result } = renderHook(() => useToggleExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ enabled: false, name: "otel-bridge" });
    });

    expect(
      queryClient.getQueryData<typeof extensionFixtures>(extensionKeys.list("ws_northstar"))?.[0]
        ?.enabled
    ).toBe(false);
    expect(
      queryClient.getQueryData<typeof extensionFixtures>(extensionKeys.list())?.[0]?.enabled
    ).toBe(true);
  });
});

describe("useUpdateExtension", () => {
  it("Should carry the ratified digest on a confirmed update retry", async () => {
    const { wrapper } = setup();
    const { result } = renderHook(() => useUpdateExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        allowUnverified: true,
        confirmNetworkDigest: "sha256:6f1c0a94d3b27e58",
        name: "dep-kit-ops",
        version: "1.1.0",
      });
    });

    expect(mocks.updateExtension).toHaveBeenCalledWith("dep-kit-ops", {
      allow_unverified: true,
      confirm_network_digest: "sha256:6f1c0a94d3b27e58",
      version: "1.1.0",
    });
  });

  it("Should invalidate the cached kit inventory after a successful update", async () => {
    const { queryClient, wrapper } = setup();
    const inventoryKey = extensionKeys.inventory("otel-bridge");
    queryClient.setQueryData(inventoryKey, []);
    const { result } = renderHook(() => useUpdateExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ name: "otel-bridge" });
    });

    await waitFor(() => expect(queryClient.getQueryState(inventoryKey)?.isInvalidated).toBe(true));
  });

  it("Should stay silent for a network-confirmation refusal instead of toasting it", async () => {
    const { wrapper } = setup();
    mocks.updateExtension.mockRejectedValue(
      new ExtensionsApiError(
        "confirmation required",
        409,
        "daemon",
        "extension_network_confirmation_required",
        "sha256:6f1c0a94d3b27e58"
      )
    );
    const { result } = renderHook(() => useUpdateExtension(), { wrapper });

    await act(async () => {
      await expect(result.current.mutateAsync({ name: "dep-kit-ops" })).rejects.toThrow(
        "confirmation required"
      );
    });

    expect(mocks.toastError).not.toHaveBeenCalled();
  });
});

describe("extension lifecycle mutations", () => {
  it("Should reconcile Marketplace discovery after an extension update", async () => {
    const { queryClient, wrapper } = setup();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ name: "otel-bridge", version: "0.6.0" });
    });

    expect(mocks.updateExtension).toHaveBeenCalledWith("otel-bridge", {
      allow_unverified: false,
      version: "0.6.0",
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("otel-bridge updated to v0.6.0");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: extensionKeys.all });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: marketplaceKeys.all });
  });

  it("Should scope only a dev unlink to the active workspace instance", async () => {
    mocks.activeWorkspaceId = "ws_northstar";
    const { wrapper } = setup();
    const { result } = renderHook(() => useRemoveExtension(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ dev: true, name: "slack-notify" });
    });

    expect(mocks.removeExtension).toHaveBeenCalledWith("slack-notify", {
      workspaceId: "ws_northstar",
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("slack-notify dev overlay unlinked");

    await act(async () => {
      await result.current.mutateAsync({ dev: false, name: "otel-bridge" });
    });

    expect(mocks.removeExtension).toHaveBeenLastCalledWith("otel-bridge", {});
    expect(mocks.toastSuccess).toHaveBeenCalledWith("otel-bridge removed");
  });

  it("Should toast successful lifecycle actions without leaking mutation context", async () => {
    const { wrapper } = setup();
    const { result } = renderHook(
      () => ({ remove: useRemoveExtension(), update: useUpdateExtension() }),
      { wrapper }
    );

    await act(async () => {
      await result.current.update.mutateAsync({ name: "otel-bridge" });
      await result.current.remove.mutateAsync({ dev: false, name: "slack-notify" });
    });

    expect(mocks.updateExtension).toHaveBeenCalledWith("otel-bridge", { allow_unverified: false });
    expect(mocks.removeExtension).toHaveBeenCalledWith("slack-notify", {});
    expect(mocks.toastSuccess).toHaveBeenCalledWith("otel-bridge updated");
    expect(mocks.toastSuccess).toHaveBeenCalledWith("slack-notify removed");
  });

  it("Should surface extension removal errors", async () => {
    const { wrapper } = setup();
    mocks.removeExtension.mockRejectedValue(new Error("remove rejected"));
    const { result } = renderHook(() => useRemoveExtension(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({ dev: false, name: "slack-notify" })
      ).rejects.toThrow("remove rejected");
    });

    expect(mocks.toastError).toHaveBeenCalledWith("remove rejected");
  });
});
