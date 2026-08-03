// Suite: Extension detail dialog state
// Invariant: extension detail selects at most one dialog at a time, only consents to an
// unverified update through the explicit gate, and resumes a refused enable or update with the
// exact variables the daemon rejected plus the digest it named.
// Boundary IN: useExtensionDetailState dialog actions, update routing, and confirm resumption.
// Boundary OUT: Extension queries, mutations, and rendered dialog primitives.

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ExtensionsApiError } from "../../adapters/extensions-api";
import { extensionFixtures } from "../../mocks/fixtures";
import type { InstalledExtensionView } from "../../types";

const CURRENT_DIGEST = "sha256:6f1c0a94d3b27e58";

function networkConfirmationRefusal() {
  return new ExtensionsApiError("network confirmation required", 409, "daemon", {
    code: "extension_network_confirmation_required",
    currentDigest: CURRENT_DIGEST,
  });
}

const mocks = vi.hoisted(() => ({
  detail: { data: null as InstalledExtensionView | null, workspaceId: null as string | null },
  inventory: { data: [], error: null, isLoading: false, refetch: vi.fn() },
  logs: { entries: [] },
  logsOptions: null as {
    enabled?: boolean;
    name: string;
    workspaceId?: string | null;
  } | null,
  navigate: vi.fn(),
  toggle: { data: undefined as unknown, mutate: vi.fn(), mutateAsync: vi.fn() },
  update: { mutate: vi.fn(), mutateAsync: vi.fn() },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("../use-extension-actions", () => ({
  useToggleExtension: () => mocks.toggle,
  useUpdateExtension: () => mocks.update,
}));
vi.mock("../use-extensions", () => ({
  useExtensionDetail: () => mocks.detail,
  useExtensionKitInventory: () => mocks.inventory,
}));
vi.mock("../use-extension-logs", () => ({
  useExtensionLogs: (options: { enabled?: boolean; name: string; workspaceId?: string | null }) => {
    mocks.logsOptions = options;
    return mocks.logs;
  },
}));

import { useExtensionDetailState } from "../use-extension-detail-state";

function installedView(overrides: Partial<(typeof extensionFixtures)[number]>) {
  return {
    extension: { ...extensionFixtures[0]!, ...overrides },
    listing: null,
    updateAvailable: true,
  } as InstalledExtensionView;
}

describe("useExtensionDetailState", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.detail.data = null;
    mocks.detail.workspaceId = null;
    mocks.logsOptions = null;
    mocks.toggle.data = undefined;
    mocks.toggle.mutateAsync.mockResolvedValue({
      automation_started: [],
      extension: extensionFixtures[0],
    });
    mocks.update.mutateAsync.mockResolvedValue(undefined);
  });

  it("Should scope logs to the resolved extension instance instead of the active workspace", () => {
    mocks.detail.workspaceId = "ws_active";
    mocks.detail.data = installedView({ dev: false, workspace_id: undefined });

    renderHook(() => useExtensionDetailState("otel-bridge"));

    expect(mocks.logsOptions).toMatchObject({
      enabled: true,
      name: "otel-bridge",
      workspaceId: null,
    });

    mocks.detail.data = installedView({ dev: true, workspace_id: "ws_dev" });

    renderHook(() => useExtensionDetailState("otel-bridge"));

    expect(mocks.logsOptions).toMatchObject({
      enabled: true,
      name: "otel-bridge",
      workspaceId: "ws_dev",
    });
  });

  it("Should replace provenance with removal before a dialog is dismissed", () => {
    const { result } = renderHook(() => useExtensionDetailState("otel-bridge"));

    act(() => {
      result.current.requestProvenance();
    });
    expect(result.current.activeDialog).toBe("provenance");

    act(() => {
      result.current.requestRemoval();
    });
    expect(result.current.activeDialog).toBe("remove");

    act(() => {
      result.current.dismissDialog();
    });
    expect(result.current.activeDialog).toBeNull();
  });

  it("Should update a checksum-verified extension without a consent detour", async () => {
    mocks.detail.data = installedView({ remote_version: undefined });
    const { result } = renderHook(() =>
      useExtensionDetailState("otel-bridge", { updateVersion: "0.6.0" })
    );

    await act(async () => {
      await result.current.requestUpdate();
    });

    expect(mocks.update.mutateAsync).toHaveBeenCalledWith({
      allowUnverified: false,
      name: "otel-bridge",
      version: "0.6.0",
    });
    expect(result.current.activeDialog).toBeNull();
  });

  it("Should gate an unverified update behind explicit consent before allowing it", async () => {
    mocks.detail.data = installedView({
      name: "slack-notify",
      provenance: undefined,
      remote_version: "v1.1.0",
      trust: {
        allow_unverified: true,
        checksum_verified: false,
        decision: "allowed_unverified",
        registry_tier: "community",
      },
    });
    const { result } = renderHook(() =>
      useExtensionDetailState("slack-notify", { updateVersion: "1.2.0" })
    );

    await act(async () => {
      await result.current.requestUpdate();
    });

    expect(mocks.update.mutate).not.toHaveBeenCalled();
    expect(mocks.update.mutateAsync).not.toHaveBeenCalled();
    expect(result.current.activeDialog).toBe("update");

    await act(async () => {
      await result.current.submitUpdate();
    });

    expect(mocks.update.mutateAsync).toHaveBeenCalledWith({
      allowUnverified: true,
      name: "slack-notify",
      version: "1.2.0",
    });
    expect(result.current.activeDialog).toBeNull();
  });

  // UT-064: enable and update share one affordance, and each retry ratifies the digest without
  // silently changing what the operator originally asked for.
  it("Should open the shared confirm affordance when enable is refused and resume it with the digest", async () => {
    mocks.detail.data = installedView({ name: "dep-kit-ops" });
    mocks.toggle.mutateAsync.mockRejectedValueOnce(networkConfirmationRefusal());
    const { result } = renderHook(() => useExtensionDetailState("dep-kit-ops"));

    await act(async () => {
      await result.current.requestToggle(true);
    });

    expect(result.current.networkConfirm).toEqual({
      action: "enable",
      digest: CURRENT_DIGEST,
      variables: { enabled: true, name: "dep-kit-ops" },
    });

    await act(async () => {
      await result.current.submitNetworkConfirm();
    });

    expect(mocks.toggle.mutateAsync).toHaveBeenLastCalledWith({
      confirmNetworkDigest: CURRENT_DIGEST,
      enabled: true,
      name: "dep-kit-ops",
    });
    expect(result.current.networkConfirm).toBeNull();
  });

  it("Should resume a refused unverified update with its original variables plus the digest", async () => {
    mocks.detail.data = installedView({
      name: "dep-kit-ops",
      provenance: undefined,
      remote_version: "v1.1.0",
      trust: {
        allow_unverified: true,
        checksum_verified: false,
        decision: "allowed_unverified",
        registry_tier: "community",
      },
    });
    mocks.update.mutateAsync.mockRejectedValueOnce(networkConfirmationRefusal());
    const { result } = renderHook(() =>
      useExtensionDetailState("dep-kit-ops", { updateVersion: "1.2.0" })
    );

    await act(async () => {
      await result.current.requestUpdate();
    });
    await act(async () => {
      await result.current.submitUpdate();
    });

    // The consent decision and the resolved target survive the refusal.
    expect(result.current.networkConfirm).toEqual({
      action: "update",
      digest: CURRENT_DIGEST,
      variables: { allowUnverified: true, name: "dep-kit-ops", version: "1.2.0" },
    });
    expect(result.current.activeDialog).toBeNull();

    await act(async () => {
      await result.current.submitNetworkConfirm();
    });

    expect(mocks.update.mutateAsync).toHaveBeenLastCalledWith({
      allowUnverified: true,
      confirmNetworkDigest: CURRENT_DIGEST,
      name: "dep-kit-ops",
      version: "1.2.0",
    });
    expect(result.current.networkConfirm).toBeNull();
    expect(result.current.activeDialog).toBeNull();
  });

  it("Should clear the pending confirmation when the operator dismisses it", async () => {
    mocks.detail.data = installedView({ name: "dep-kit-ops" });
    mocks.toggle.mutateAsync.mockRejectedValueOnce(networkConfirmationRefusal());
    const { result } = renderHook(() => useExtensionDetailState("dep-kit-ops"));

    await act(async () => {
      await result.current.requestToggle(true);
    });
    expect(result.current.networkConfirm).not.toBeNull();

    act(() => {
      result.current.dismissNetworkConfirm();
    });

    expect(result.current.networkConfirm).toBeNull();
  });

  it("Should not open confirmation for an unrelated lifecycle failure", async () => {
    mocks.detail.data = installedView({ name: "dep-kit-ops" });
    mocks.toggle.mutateAsync.mockRejectedValueOnce(
      new ExtensionsApiError("daemon refused the toggle", 500, "daemon")
    );
    const { result } = renderHook(() => useExtensionDetailState("dep-kit-ops"));

    await act(async () => {
      await result.current.requestToggle(true);
    });

    expect(mocks.toggle.mutateAsync).toHaveBeenCalledWith({
      enabled: true,
      name: "dep-kit-ops",
    });
    expect(result.current.networkConfirm).toBeNull();
  });
});
