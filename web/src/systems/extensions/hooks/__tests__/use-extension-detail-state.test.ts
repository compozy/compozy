// Suite: Extension detail dialog state
// Invariant: extension detail selects at most one dialog at a time and only consents to an
// unverified update through the explicit gate, using the catalog target ahead of installed
// remote-version metadata when both are present.
// Boundary IN: useExtensionDetailState dialog actions and update routing.
// Boundary OUT: Extension queries, mutations, and rendered dialog primitives.

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { extensionFixtures } from "../../mocks/fixtures";
import type { InstalledExtensionView } from "../../types";

const mocks = vi.hoisted(() => ({
  bundles: { data: [] },
  detail: { data: null as InstalledExtensionView | null, workspaceId: null as string | null },
  logs: { entries: [] },
  navigate: vi.fn(),
  toggle: { mutate: vi.fn() },
  update: { mutate: vi.fn(), mutateAsync: vi.fn() },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("../use-extension-actions", () => ({
  useToggleExtension: () => mocks.toggle,
  useUpdateExtension: () => mocks.update,
}));
vi.mock("../use-extensions", () => ({
  useBundleActivations: () => mocks.bundles,
  useExtensionDetail: () => mocks.detail,
}));
vi.mock("../use-extension-logs", () => ({ useExtensionLogs: () => mocks.logs }));

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
    mocks.update.mutateAsync.mockResolvedValue(undefined);
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
      result.current.requestUpdate();
    });

    expect(mocks.update.mutate).toHaveBeenCalledWith({
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

    act(() => {
      result.current.requestUpdate();
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
});
