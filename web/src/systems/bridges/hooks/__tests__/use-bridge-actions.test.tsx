import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useCreateBridge,
  useDeleteBridgeSecretBinding,
  useDisableBridge,
  useEnableBridge,
  usePutBridgeSecretBinding,
  useResolveBridgeTarget,
  useRestartBridge,
  useTestBridgeDelivery,
  useUpdateBridge,
} from "@/systems/bridges/hooks/use-bridge-actions";
import {
  useRegisterBridgeWebhook,
  useSendBridgeTest,
  useVerifyBridge,
} from "@/systems/bridges/hooks/use-bridge-setup-actions";

vi.mock("@/systems/bridges/adapters/bridges-api", () => ({
  createBridge: vi.fn(),
  deleteBridgeSecretBinding: vi.fn(),
  disableBridge: vi.fn(),
  enableBridge: vi.fn(),
  getBridge: vi.fn(),
  listBridgeSecretBindings: vi.fn(),
  listBridgeProviders: vi.fn(),
  listBridgeRoutes: vi.fn(),
  listBridges: vi.fn(),
  putBridgeSecretBinding: vi.fn(),
  resolveBridgeTarget: vi.fn(),
  restartBridge: vi.fn(),
  testBridgeDelivery: vi.fn(),
  updateBridge: vi.fn(),
}));

vi.mock("@/systems/bridges/adapters/bridge-setup-api", () => ({
  getSlackBridgeManifest: vi.fn(),
  registerBridgeWebhook: vi.fn(),
  sendBridgeTest: vi.fn(),
  verifyBridge: vi.fn(),
}));

import {
  createBridge,
  deleteBridgeSecretBinding,
  disableBridge,
  enableBridge,
  putBridgeSecretBinding,
  resolveBridgeTarget,
  restartBridge,
  testBridgeDelivery,
  updateBridge,
} from "@/systems/bridges/adapters/bridges-api";
import {
  registerBridgeWebhook,
  sendBridgeTest,
  verifyBridge,
} from "@/systems/bridges/adapters/bridge-setup-api";
import { bridgeKeys } from "@/systems/bridges/lib/query-keys";

function createWrapperAndClient() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

describe("useCreateBridge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("creates a bridge and invalidates the bridge root query", async () => {
    vi.mocked(createBridge).mockResolvedValue({
      bridge: {
        created_at: "2026-04-13T12:00:00Z",
        display_name: "Support",
        enabled: true,
        extension_name: "ext-telegram",
        id: "brg_support",
        notification_suppress: false,
        platform: "telegram",
        routing_policy: { include_group: true, include_peer: true, include_thread: true },
        scope: "workspace",
        status: "starting",
        updated_at: "2026-04-13T12:00:00Z",
        workspace_id: "ws_test",
      },
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "starting",
      },
    });

    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateBridge(), { wrapper });

    act(() => {
      result.current.mutate({
        dm_policy: "allowlist",
        display_name: "Support",
        enabled: true,
        extension_name: "ext-telegram",
        notification_suppress: false,
        platform: "telegram",
        provider_config: {
          mode: "bot",
        },
        routing_policy: { include_group: true, include_peer: true, include_thread: true },
        scope: "workspace",
        workspace_id: "ws_test",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(createBridge).toHaveBeenCalledWith({
      dm_policy: "allowlist",
      display_name: "Support",
      enabled: true,
      extension_name: "ext-telegram",
      notification_suppress: false,
      platform: "telegram",
      provider_config: {
        mode: "bot",
      },
      routing_policy: { include_group: true, include_peer: true, include_thread: true },
      scope: "workspace",
      workspace_id: "ws_test",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges"],
    });
  });
});

describe("useTestBridgeDelivery", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("tests delivery and invalidates detail plus routes for the selected bridge", async () => {
    vi.mocked(testBridgeDelivery).mockResolvedValue({
      delivery_target: {
        bridge_instance_id: "brg_support",
        mode: "reply",
        peer_id: "peer_123",
      },
      status: "resolved",
    });

    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useTestBridgeDelivery(), { wrapper });

    act(() => {
      result.current.mutate({
        data: {
          target: {
            bridge_instance_id: "brg_support",
            peer_id: "peer_123",
          },
        },
        id: "brg_support",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(testBridgeDelivery).toHaveBeenCalledWith("brg_support", {
      target: {
        bridge_instance_id: "brg_support",
        peer_id: "peer_123",
      },
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "detail", "brg_support"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "routes", "brg_support"],
    });
  });
});

describe("bridge mutations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("updates a bridge and invalidates detail plus routes", async () => {
    vi.mocked(updateBridge).mockResolvedValue({
      bridge: {
        created_at: "2026-04-13T12:00:00Z",
        display_name: "Support Ops",
        enabled: true,
        extension_name: "ext-telegram",
        id: "brg_support",
        notification_suppress: false,
        platform: "telegram",
        routing_policy: { include_group: true, include_peer: true, include_thread: true },
        scope: "workspace",
        status: "ready",
        updated_at: "2026-04-13T12:10:00Z",
        workspace_id: "ws_test",
      },
      health: {
        auth_failures_total: 0,
        bridge_instance_id: "brg_support",
        delivery_backlog: 0,
        delivery_dropped_total: 0,
        delivery_failures_total: 0,
        route_count: 0,
        status: "ready",
      },
    });

    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateBridge(), { wrapper });

    act(() => {
      result.current.mutate({
        data: {
          display_name: "Support Ops",
        },
        id: "brg_support",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(updateBridge).toHaveBeenCalledWith("brg_support", {
      display_name: "Support Ops",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "routes", "brg_support"],
    });
  });

  it("writes and deletes secret bindings while invalidating the secret-binding query", async () => {
    vi.mocked(putBridgeSecretBinding).mockResolvedValue({
      binding_name: "bot_token",
      bridge_instance_id: "brg_support",
      created_at: "2026-04-13T12:00:00Z",
      kind: "bot_token",
      updated_at: "2026-04-13T12:10:00Z",
      secret_ref: "vault:bridges/brg_support/bot_token",
    });
    vi.mocked(deleteBridgeSecretBinding).mockResolvedValue(undefined);

    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result: putResult } = renderHook(() => usePutBridgeSecretBinding(), { wrapper });
    act(() => {
      putResult.current.mutate({
        bindingName: "bot_token",
        data: {
          kind: "bot_token",
          secret_ref: "vault:bridges/brg_support/bot_token",
          secret_value: "telegram-token",
        },
        id: "brg_support",
      });
    });
    await waitFor(() => {
      expect(putResult.current.isSuccess).toBe(true);
    });

    const { result: deleteResult } = renderHook(() => useDeleteBridgeSecretBinding(), {
      wrapper,
    });
    act(() => {
      deleteResult.current.mutate({
        bindingName: "bot_token",
        id: "brg_support",
      });
    });
    await waitFor(() => {
      expect(deleteResult.current.isSuccess).toBe(true);
    });

    expect(putBridgeSecretBinding).toHaveBeenCalledWith("brg_support", "bot_token", {
      kind: "bot_token",
      secret_ref: "vault:bridges/brg_support/bot_token",
      secret_value: "telegram-token",
    });
    expect(deleteBridgeSecretBinding).toHaveBeenCalledWith("brg_support", "bot_token");
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "secret-bindings", "brg_support"],
    });
  });

  it("runs lifecycle mutations and invalidates routes plus secret bindings", async () => {
    vi.mocked(enableBridge).mockResolvedValue(undefined as never);
    vi.mocked(disableBridge).mockResolvedValue(undefined as never);
    vi.mocked(restartBridge).mockResolvedValue(undefined as never);

    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result: enableResult } = renderHook(() => useEnableBridge(), { wrapper });
    act(() => {
      enableResult.current.mutate({ id: "brg_support" });
    });
    await waitFor(() => {
      expect(enableResult.current.isSuccess).toBe(true);
    });

    const { result: disableResult } = renderHook(() => useDisableBridge(), { wrapper });
    act(() => {
      disableResult.current.mutate({ id: "brg_support" });
    });
    await waitFor(() => {
      expect(disableResult.current.isSuccess).toBe(true);
    });

    const { result: restartResult } = renderHook(() => useRestartBridge(), { wrapper });
    act(() => {
      restartResult.current.mutate({ id: "brg_support" });
    });
    await waitFor(() => {
      expect(restartResult.current.isSuccess).toBe(true);
    });

    expect(enableBridge).toHaveBeenCalledWith("brg_support");
    expect(disableBridge).toHaveBeenCalledWith("brg_support");
    expect(restartBridge).toHaveBeenCalledWith("brg_support");
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "routes", "brg_support"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["bridges", "secret-bindings", "brg_support"],
    });
  });
});

describe("bridge setup control mutations", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("keeps verify and registration evidence in mutation state without caching it as bridge data", async () => {
    vi.mocked(verifyBridge).mockResolvedValue({
      bridge_instance_id: "brg_support",
      checks: [{ check: "provider.identity", remediation: "", status: "pass" }],
    });
    vi.mocked(registerBridgeWebhook).mockResolvedValue({
      bridge_instance_id: "brg_support",
      remediation: "",
      status: "pass",
    });
    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result: verifyResult } = renderHook(() => useVerifyBridge(), { wrapper });
    act(() => verifyResult.current.mutate({ id: "brg_support" }));
    await waitFor(() => expect(verifyResult.current.isSuccess).toBe(true));

    const { result: registerResult } = renderHook(() => useRegisterBridgeWebhook(), { wrapper });
    act(() => registerResult.current.mutate({ id: "brg_support" }));
    await waitFor(() => expect(registerResult.current.isSuccess).toBe(true));

    expect(verifyResult.current.data?.checks[0]?.status).toBe("pass");
    expect(registerResult.current.data?.status).toBe("pass");
    expect(queryClient.getQueryCache().findAll({ queryKey: ["bridges"] })).toEqual([]);
    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("sends a real test and invalidates only the affected lists, detail, and routes", async () => {
    vi.mocked(sendBridgeTest).mockResolvedValue({
      bridge_instance_id: "brg_support",
      delivery_id: "delivery_123",
      delivery_target: {
        bridge_instance_id: "brg_support",
        mode: "direct-send",
        peer_id: "peer_123",
      },
      remote_message_id: "remote_123",
      status: "delivered",
    });
    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSendBridgeTest(), { wrapper });
    const data = {
      message: "Operator test",
      target: {
        bridge_instance_id: "brg_support",
        mode: "direct-send" as const,
        peer_id: "peer_123",
      },
    };

    act(() => result.current.mutate({ data, id: "brg_support" }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(sendBridgeTest).toHaveBeenCalledWith("brg_support", data);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["bridges", "list"] });
    expect(invalidateSpy).toHaveBeenCalledWith({
      exact: true,
      queryKey: ["bridges", "detail", "brg_support"],
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      exact: true,
      queryKey: ["bridges", "routes", "brg_support"],
    });
    expect(invalidateSpy).toHaveBeenCalledTimes(3);
  });
});

describe("useResolveBridgeTarget", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("invalidates every cached target list for the resolved bridge", async () => {
    vi.mocked(resolveBridgeTarget).mockResolvedValue({
      result: { ambiguous: false, match: null, step: 1 },
    });
    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useResolveBridgeTarget(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "brg_support", data: { name: "launch" } });
    });

    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: bridgeKeys.targetsForBridge("brg_support"),
    });
  });

  it("exposes resolution errors without invalidating unchanged target lists", async () => {
    const resolutionError = new Error("target resolution failed");
    vi.mocked(resolveBridgeTarget).mockRejectedValue(resolutionError);
    const { queryClient, wrapper } = createWrapperAndClient();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useResolveBridgeTarget(), { wrapper });

    await act(async () => {
      await expect(
        result.current.mutateAsync({ id: "brg_support", data: { name: "launch" } })
      ).rejects.toBe(resolutionError);
    });

    await waitFor(() => {
      expect(result.current.error).toBe(resolutionError);
    });
    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});
