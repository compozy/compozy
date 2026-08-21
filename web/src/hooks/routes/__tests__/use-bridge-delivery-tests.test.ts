import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const bridgeHookMocks = vi.hoisted(() => ({
  registerBridgeWebhook: vi.fn(),
  sendBridgeTest: vi.fn(),
  testBridgeDelivery: vi.fn(),
  verifyBridge: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

vi.mock("@/systems/bridges", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/bridges")>();
  return {
    ...actual,
    useRegisterBridgeWebhook: () => ({ mutateAsync: bridgeHookMocks.registerBridgeWebhook }),
    useSendBridgeTest: () => ({ mutateAsync: bridgeHookMocks.sendBridgeTest }),
    useTestBridgeDelivery: () => ({ mutateAsync: bridgeHookMocks.testBridgeDelivery }),
    useVerifyBridge: () => ({ mutateAsync: bridgeHookMocks.verifyBridge }),
  };
});

import type {
  BridgeSummary,
  BridgeWebhookRegistrationResponse,
  SendBridgeTestResponse,
  TestBridgeDeliveryResponse,
} from "@/systems/bridges";
import { bridgeVerifyFixture, sendBridgeTestFixture } from "@/systems/bridges/mocks";

import { useBridgeDeliveryTests } from "../use-bridge-delivery-tests";
import { useBridgeSetupFlow } from "../use-bridge-setup-flow";

const bridge = {
  created_at: "2026-07-25T12:00:00Z",
  delivery_defaults: {},
  id: "bridge-alpha",
  display_name: "Alpha bridge",
  dm_policy: "open",
  enabled: true,
  extension_name: "ext-alpha",
  notification_suppress: false,
  platform: "test",
  provider_config: {},
  routing_policy: { include_group: true, include_peer: true, include_thread: true },
  scope: "workspace",
  status: "ready",
  updated_at: "2026-07-25T12:00:00Z",
  workspace_id: "workspace-alpha",
  profile_id: "00000000000000000000000000",
  profile_name: "default",
} satisfies BridgeSummary;

function createQueryClientWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });

  return function QueryClientWrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe("use bridge delivery tests", () => {
  it("Should dry-run the bridge currently rendered after the route changes", async () => {
    const nextBridge = { ...bridge, display_name: "Beta bridge", id: "bridge-beta" };
    bridgeHookMocks.testBridgeDelivery.mockClear();
    bridgeHookMocks.testBridgeDelivery.mockResolvedValue({
      delivery_target: { bridge_instance_id: nextBridge.id },
      status: "resolved",
    } satisfies TestBridgeDeliveryResponse);

    const { result, rerender } = renderHook(
      ({ currentBridge }) => useBridgeDeliveryTests(currentBridge),
      { initialProps: { currentBridge: bridge }, wrapper: createQueryClientWrapper() }
    );

    act(() => result.current.resetDraft());
    rerender({ currentBridge: nextBridge });
    act(() => result.current.panelProps.onDryRun());

    await waitFor(() => {
      expect(bridgeHookMocks.testBridgeDelivery).toHaveBeenCalledWith({
        data: expect.objectContaining({
          target: expect.objectContaining({ bridge_instance_id: nextBridge.id }),
        }),
        id: nextBridge.id,
        profile: nextBridge.profile_name,
      });
    });
  });

  it("Should send a real test through the rendered bridge owner", async () => {
    const ownedBridge = { ...bridge, profile_name: "marketing" };
    bridgeHookMocks.sendBridgeTest.mockReset();
    bridgeHookMocks.sendBridgeTest.mockResolvedValue({
      ...sendBridgeTestFixture,
      bridge_instance_id: ownedBridge.id,
    } satisfies SendBridgeTestResponse);

    const { result } = renderHook(() => useBridgeDeliveryTests(ownedBridge), {
      wrapper: createQueryClientWrapper(),
    });

    act(() => result.current.resetDraft());
    act(() =>
      result.current.panelProps.onDraftChange({
        ...result.current.panelProps.draft,
        message: "Profile-owned delivery",
      })
    );
    act(() => result.current.panelProps.onSendTest());

    await waitFor(() => {
      expect(bridgeHookMocks.sendBridgeTest).toHaveBeenCalledWith({
        data: expect.objectContaining({ message: "Profile-owned delivery" }),
        id: ownedBridge.id,
        profile: ownedBridge.profile_name,
      });
    });
  });

  it("Should register and verify setup through the rendered bridge owner", async () => {
    const ownedBridge = { ...bridge, profile_name: "marketing" };
    bridgeHookMocks.registerBridgeWebhook.mockReset();
    bridgeHookMocks.verifyBridge.mockReset();
    bridgeHookMocks.registerBridgeWebhook.mockResolvedValue({
      bridge_instance_id: ownedBridge.id,
      remediation: "",
      status: "pass",
    } satisfies BridgeWebhookRegistrationResponse);
    bridgeHookMocks.verifyBridge.mockResolvedValue({
      ...bridgeVerifyFixture,
      bridge_instance_id: ownedBridge.id,
    });

    const { result } = renderHook(
      () =>
        useBridgeSetupFlow({
          bindings: [],
          bridge: ownedBridge,
          health: undefined,
          provider: undefined,
        }),
      { wrapper: createQueryClientWrapper() }
    );

    act(() => result.current.registerWebhook());
    await waitFor(() =>
      expect(bridgeHookMocks.registerBridgeWebhook).toHaveBeenCalledWith({
        id: ownedBridge.id,
        profile: ownedBridge.profile_name,
      })
    );
    act(() => result.current.verify());
    await waitFor(() =>
      expect(bridgeHookMocks.verifyBridge).toHaveBeenCalledWith({
        id: ownedBridge.id,
        profile: ownedBridge.profile_name,
      })
    );
  });
});
