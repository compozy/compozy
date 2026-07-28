import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const bridgeHookMocks = vi.hoisted(() => ({
  sendBridgeTest: vi.fn(),
  testBridgeDelivery: vi.fn(),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

vi.mock("@/systems/bridges", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/bridges")>();
  return {
    ...actual,
    useSendBridgeTest: () => ({ mutateAsync: bridgeHookMocks.sendBridgeTest }),
    useTestBridgeDelivery: () => ({ mutateAsync: bridgeHookMocks.testBridgeDelivery }),
  };
});

import type { BridgeSummary, TestBridgeDeliveryResponse } from "@/systems/bridges";

import { useBridgeDeliveryTests } from "../use-bridge-delivery-tests";

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
      });
    });
  });
});
