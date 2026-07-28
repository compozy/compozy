import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  bundleActivationFixtures,
  extensionFixtures,
  extensionProvenanceFixtures,
} from "../../mocks/fixtures";
import type { ExtensionEntry } from "../../types";

const mocks = vi.hoisted(() => ({
  getBundleActivation: vi.fn(),
  getExtensionProvenance: vi.fn(),
  listBundleActivations: vi.fn(),
  listExtensions: vi.fn(),
  useMarketplaceKind: vi.fn(),
}));

vi.mock("@/systems/marketplace", () => ({
  useMarketplaceKind: mocks.useMarketplaceKind,
}));

vi.mock("../../adapters/extensions-api", () => ({
  getBundleActivation: mocks.getBundleActivation,
  getExtensionProvenance: mocks.getExtensionProvenance,
  listBundleActivations: mocks.listBundleActivations,
  listExtensions: mocks.listExtensions,
}));

import {
  useBundleActivation,
  useBundleActivations,
  useExtensionDetail,
  useExtensionInventory,
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
  mocks.listExtensions.mockResolvedValue(extensionFixtures);
  mocks.listBundleActivations.mockResolvedValue(bundleActivationFixtures);
  mocks.getBundleActivation.mockResolvedValue(bundleActivationFixtures[0]);
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
    ]);
    expect(result.current.data[0]?.listing?.description).toBe("Export session spans.");
    expect(mocks.useMarketplaceKind).not.toHaveBeenCalled();
  });

  it("Should leave the listing null when the server attaches no projection", async () => {
    const { result } = renderHook(() => useExtensionInventory(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data.every(item => item.updateAvailable === false)).toBe(true);
    expect(result.current.data.every(item => item.listing === null)).toBe(true);
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

  it("Should load bundle inventory and direct activation detail with separate keys", async () => {
    const inventory = renderHook(() => useBundleActivations(), { wrapper });
    await waitFor(() => expect(inventory.result.current.isSuccess).toBe(true));
    expect(inventory.result.current.data).toEqual(bundleActivationFixtures);

    const detail = renderHook(() => useBundleActivation("activation-ops-starter"), { wrapper });
    await waitFor(() => expect(detail.result.current.isSuccess).toBe(true));
    expect(detail.result.current.data).toEqual(bundleActivationFixtures[0]);
    expect(mocks.getBundleActivation).toHaveBeenCalledWith(
      "activation-ops-starter",
      expect.any(AbortSignal)
    );
  });
});
