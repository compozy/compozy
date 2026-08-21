// Suite: extension query hooks
// Invariant: extension catalogs preserve query scope and stop periodic reads when disabled.
// Boundary IN: extension query options, liveness gates, and hook projections.
// Boundary OUT: extension mutations and log transport, owned by their dedicated suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  extensionFixtures,
  extensionInventoryFixtures,
  extensionProvenanceFixtures,
} from "../../mocks/fixtures";
import { extensionKeys } from "../../lib/query-keys";
import type { ExtensionEntry } from "../../types";

const mocks = vi.hoisted(() => ({
  activeWorkspaceId: null as string | null,
  getExtensionInventory: vi.fn(),
  getExtensionProvenance: vi.fn(),
  listExtensions: vi.fn(),
  useMarketplaceKind: vi.fn(),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: mocks.activeWorkspaceId }),
}));

vi.mock("@/systems/marketplace", () => ({
  useMarketplaceKind: mocks.useMarketplaceKind,
}));

vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => ({ destination: "default" }),
}));

vi.mock("../../adapters/extensions-api", () => ({
  getExtensionInventory: mocks.getExtensionInventory,
  getExtensionProvenance: mocks.getExtensionProvenance,
  listExtensions: mocks.listExtensions,
}));

import {
  useExtensionDetail,
  useExtensionInventory,
  useExtensionKitInventory,
  useExtensionProvenance,
} from "../use-extensions";

const otelMarketplace: NonNullable<ExtensionEntry["marketplace"]> = {
  description: "Export session spans.",
  entry_id: "otel-bridge",
  installed: true,
  kind: "extension",
  name: "otel-bridge",
  source: "marketplace_registry",
  update_available: true,
};

function wrapper({ children }: { children: ReactNode }) {
  return createElement(
    QueryClientProvider,
    { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    children
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.activeWorkspaceId = null;
  mocks.listExtensions.mockResolvedValue(extensionFixtures);
  mocks.getExtensionInventory.mockResolvedValue({
    enabled: false,
    extension: "dep-kit-ops",
    items: extensionInventoryFixtures["dep-kit-ops"],
  });
  mocks.getExtensionProvenance.mockResolvedValue(extensionProvenanceFixtures["otel-bridge"]);
});

describe("useExtensionInventory", () => {
  it("Should expose the server-projected listing and issue no browse request", async () => {
    mocks.listExtensions.mockResolvedValue(
      extensionFixtures.map(extension =>
        extension.name === "otel-bridge"
          ? { ...extension, marketplace: otelMarketplace }
          : extension
      )
    );
    const { result } = renderHook(() => useExtensionInventory(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data.map(item => [item.extension.name, item.updateAvailable])).toEqual([
      ["otel-bridge", true],
      ["slack-notify", false],
      ["dep-kit-ops", false],
      ["acme.tools", false],
      ["legacy-notes", false],
    ]);
    expect(result.current.data[0]?.listing?.description).toBe("Export session spans.");
    expect(mocks.useMarketplaceKind).not.toHaveBeenCalled();
  });

  it("Should cache each workspace instance under its own key and never reuse another's rows", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const scopedWrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    mocks.activeWorkspaceId = "ws_northstar";
    const scoped = renderHook(() => useExtensionInventory(), { wrapper: scopedWrapper });
    await waitFor(() => expect(scoped.result.current.isSuccess).toBe(true));

    expect(mocks.listExtensions).toHaveBeenCalledWith(
      { profileName: "default", workspaceId: "ws_northstar" },
      expect.any(AbortSignal)
    );
    expect(client.getQueryData(extensionKeys.list("ws_northstar", "default"))).toEqual(
      extensionFixtures
    );
    expect(client.getQueryData(extensionKeys.list(null, "default"))).toBeUndefined();

    mocks.activeWorkspaceId = null;
    const global = renderHook(() => useExtensionInventory(), { wrapper: scopedWrapper });
    await waitFor(() => expect(global.result.current.isSuccess).toBe(true));

    expect(mocks.listExtensions).toHaveBeenLastCalledWith(
      { profileName: "default", workspaceId: null },
      expect.any(AbortSignal)
    );
    expect(client.getQueryData(extensionKeys.list(null, "default"))).toEqual(extensionFixtures);
  });

  it("Should preserve daemon update availability when the server attaches no listing", async () => {
    const { result } = renderHook(() => useExtensionInventory(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data.map(item => [item.extension.name, item.updateAvailable])).toEqual([
      ["otel-bridge", true],
      ["slack-notify", false],
      ["dep-kit-ops", false],
      ["acme.tools", false],
      ["legacy-notes", false],
    ]);
    expect(result.current.data.every(item => item.listing === null)).toBe(true);
  });

  it("Should not fetch inventory while its retained window is inactive", () => {
    const { result } = renderHook(() => useExtensionInventory(false), { wrapper });

    expect(result.current.fetchStatus).toBe("idle");
    expect(mocks.listExtensions).not.toHaveBeenCalled();
  });
});

describe("extension detail queries", () => {
  it("Should select one extension from the joined inventory by installed name", async () => {
    const { result } = renderHook(() => useExtensionDetail("slack-notify"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.extension.name).toBe("slack-notify");
  });

  it("Should return null when no installed extension owns the requested name", async () => {
    const { result } = renderHook(() => useExtensionDetail("missing"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeNull();
  });
});

describe("extension management queries", () => {
  it("Should load provenance only for an enabled non-empty extension identity", async () => {
    const { result } = renderHook(() => useExtensionProvenance("otel-bridge"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(extensionProvenanceFixtures["otel-bridge"]);
    expect(mocks.getExtensionProvenance).toHaveBeenCalledWith(
      "otel-bridge",
      expect.any(AbortSignal)
    );
  });

  it("Should keep provenance disabled when the dialog is closed", () => {
    const { result } = renderHook(() => useExtensionProvenance("otel-bridge", false), { wrapper });

    expect(result.current.fetchStatus).toBe("idle");
    expect(mocks.getExtensionProvenance).not.toHaveBeenCalled();
  });

  it("Should load the kit inventory under its own name-scoped key", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const inventoryWrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);
    const inventory = renderHook(() => useExtensionKitInventory("dep-kit-ops"), {
      wrapper: inventoryWrapper,
    });

    await waitFor(() => expect(inventory.result.current.isSuccess).toBe(true));
    const expectedInventory = {
      enabled: false,
      extension: "dep-kit-ops",
      items: extensionInventoryFixtures["dep-kit-ops"],
    };
    expect(inventory.result.current.data).toEqual(expectedInventory);
    expect(client.getQueryData(extensionKeys.inventory("dep-kit-ops"))).toEqual(expectedInventory);
    expect(client.getQueryData(extensionKeys.inventory("otel-bridge"))).toBeUndefined();
    expect(mocks.getExtensionInventory).toHaveBeenCalledWith(
      "dep-kit-ops",
      expect.any(AbortSignal)
    );
  });

  it("Should keep the kit inventory disabled until the extension resolves", () => {
    const { result } = renderHook(() => useExtensionKitInventory("dep-kit-ops", false), {
      wrapper,
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(mocks.getExtensionInventory).not.toHaveBeenCalled();
  });
});
