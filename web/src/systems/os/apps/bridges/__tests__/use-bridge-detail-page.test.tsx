import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  bridgeDetailFixture,
  bridgeProvidersFixture,
  bridgeSecretBindingsFixture,
  bridgesListFixture,
  updateBridgeFixture,
} from "@/systems/bridges/mocks";

const {
  mockPutSecretBinding,
  mockUpdateBridge,
  mockUseBridge,
  mockUseBridgeProviders,
  mockUseBridgeSecretBindings,
} = vi.hoisted(() => ({
  mockPutSecretBinding: vi.fn(),
  mockUpdateBridge: vi.fn(),
  mockUseBridge: vi.fn(),
  mockUseBridgeProviders: vi.fn(),
  mockUseBridgeSecretBindings: vi.fn(),
}));

vi.mock("@/systems/bridges", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/bridges")>();
  return {
    ...actual,
    useBridge: mockUseBridge,
    useBridgeHealthStream: vi.fn(),
    useBridgeProviders: mockUseBridgeProviders,
    useBridgeRoutes: vi.fn(() => ({ data: [], error: null, isLoading: false })),
    useBridgeSecretBindings: mockUseBridgeSecretBindings,
    useBridgeTargets: vi.fn(() => ({ data: undefined, error: null, isLoading: false })),
    useBridges: vi.fn(() => ({ bridgeHealth: {}, bridges: bridgesListFixture.bridges })),
    useDeleteBridgeSecretBinding: vi.fn(() => ({ isPending: false, mutateAsync: vi.fn() })),
    useDisableBridge: vi.fn(() => ({ isPending: false, mutateAsync: vi.fn() })),
    useEnableBridge: vi.fn(() => ({ isPending: false, mutateAsync: vi.fn() })),
    usePutBridgeSecretBinding: vi.fn(() => ({
      isPending: false,
      mutateAsync: mockPutSecretBinding,
    })),
    useResolveBridgeTarget: vi.fn(() => ({ isPending: false, mutateAsync: vi.fn() })),
    useRestartBridge: vi.fn(() => ({ isPending: false, mutateAsync: vi.fn() })),
    useUpdateBridge: vi.fn(() => ({ isPending: false, mutateAsync: mockUpdateBridge })),
  };
});

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspace: { name: "HQ" }, activeWorkspaceId: "ws_hq" }),
}));

vi.mock("@/hooks/routes/use-bridge-delivery-tests", () => ({
  useBridgeDeliveryTests: () => ({
    panelProps: {
      bridgeEnabled: true,
      draft: { message: "", target: {} },
      dryRunResult: null,
      isDryRunPending: false,
      isSendTestPending: false,
      onDraftChange: vi.fn(),
      onDryRun: vi.fn(),
      onSendTest: vi.fn(),
      sendTestResult: null,
    },
    resetDraft: vi.fn(),
  }),
}));

vi.mock("@/hooks/routes/use-bridge-setup-flow", () => ({
  useBridgeSetupFlow: () => ({
    clearEvidence: vi.fn(),
    clearVerification: vi.fn(),
    isRegistering: false,
    isVerifying: false,
    projection: undefined,
    registerWebhook: vi.fn(),
    verify: vi.fn(),
  }),
}));

import { useBridgeDetailPage } from "../use-bridge-detail-page";

describe("useBridgeDetailPage secret rotations", () => {
  beforeEach(() => {
    mockPutSecretBinding.mockReset();
    mockUpdateBridge.mockReset();
    mockPutSecretBinding.mockResolvedValue(bridgeSecretBindingsFixture[0]);
    mockUpdateBridge.mockResolvedValue(updateBridgeFixture);
    mockUseBridge.mockReturnValue({ data: bridgeDetailFixture, error: null, isLoading: false });
    mockUseBridgeProviders.mockReturnValue({
      data: bridgeProvidersFixture,
      error: null,
      isLoading: false,
    });
    mockUseBridgeSecretBindings.mockReturnValue({
      data: bridgeSecretBindingsFixture,
      error: null,
      isLoading: false,
    });
  });

  it("commits a typed secret before conditionally PATCHing the edited bridge", async () => {
    const callOrder: string[] = [];
    mockPutSecretBinding.mockImplementation(async () => {
      callOrder.push("put");
      return bridgeSecretBindingsFixture[0];
    });
    mockUpdateBridge.mockImplementation(async () => {
      callOrder.push("patch");
      return updateBridgeFixture;
    });
    const { result } = renderHook(() => useBridgeDetailPage("brg_launch_room"));

    act(() => {
      result.current.detailPanelProps.onSecretDraftChange?.("bot_token", "next-token");
      result.current.detailPanelProps.onOpenEdit();
    });
    act(() => {
      result.current.editDialogProps.onDraftChange({
        ...result.current.editDialogProps.draft,
        displayName: "Launch room operations",
      });
    });
    await act(async () => {
      await result.current.editDialogProps.onSubmit();
    });

    expect(mockPutSecretBinding).toHaveBeenCalledWith(
      expect.objectContaining({ bindingName: "bot_token", id: "brg_launch_room" })
    );
    expect(mockUpdateBridge).toHaveBeenCalledWith(
      expect.objectContaining({ id: "brg_launch_room" })
    );
    expect(callOrder).toEqual(["put", "patch"]);
  });

  it("closes after a successful secret-only save without PATCHing the bridge", async () => {
    const { result } = renderHook(() => useBridgeDetailPage("brg_launch_room"));

    act(() => {
      result.current.detailPanelProps.onSecretDraftChange?.("bot_token", "next-token");
      result.current.detailPanelProps.onOpenEdit();
    });
    expect(result.current.editDialogProps.open).toBe(true);

    await act(async () => {
      await result.current.editDialogProps.onSubmit();
    });

    expect(mockPutSecretBinding).toHaveBeenCalledWith(
      expect.objectContaining({ bindingName: "bot_token", id: "brg_launch_room" })
    );
    expect(mockUpdateBridge).not.toHaveBeenCalled();
    expect(result.current.editDialogProps.open).toBe(false);
  });

  it("does not PATCH when the secret binding fails", async () => {
    mockPutSecretBinding.mockRejectedValue(new Error("Vault unavailable"));
    const { result } = renderHook(() => useBridgeDetailPage("brg_launch_room"));

    act(() => {
      result.current.detailPanelProps.onSecretDraftChange?.("bot_token", "next-token");
      result.current.detailPanelProps.onOpenEdit();
    });
    await act(async () => {
      await result.current.editDialogProps.onSubmit();
    });

    expect(mockUpdateBridge).not.toHaveBeenCalled();
  });
});
