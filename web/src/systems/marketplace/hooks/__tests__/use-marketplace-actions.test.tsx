// Invariant: mutations replace pre-install reads and reconcile catalog and installed caches.
// Owning layer: acquisition query lifecycle; canonical suite: use-marketplace-actions.test.tsx.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { extensionFixtures } from "@/systems/extensions/mocks/fixtures";
import { marketplaceCatalogFixture } from "../../mocks/fixtures";
import { useMarketplaceCatalog } from "../use-marketplace";
import { updateExtension } from "@/systems/extensions/adapters/extensions-api";
import { installMarketplaceExtension } from "../../adapters/marketplace-actions-api";
import {
  useInstallMarketplaceExtension,
  useUpdateMarketplaceExtension,
} from "../use-marketplace-actions";
vi.mock("../../adapters/marketplace-actions-api", () => ({
  installMarketplaceExtension: vi.fn(),
  refreshMarketplaceCatalog: vi.fn(),
  updateMarketplaceExtensions: vi.fn(),
}));
vi.mock("@/systems/extensions/adapters/extensions-api", () => ({ updateExtension: vi.fn() }));
const clients: QueryClient[] = [];
function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  clients.push(queryClient);
  const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { invalidateQueries, wrapper };
}
beforeEach(() => vi.clearAllMocks());
afterEach(() => {
  for (const client of clients.splice(0)) client.clear();
  vi.unstubAllGlobals();
});
describe("marketplace acquisition cache boundaries", () => {
  it("Should replace a pre-install initial search with authoritative installed state", async () => {
    /** Builds server responses that distinguish stale and authoritative installation state. */
    const listing = (installed: boolean) =>
      Response.json({
        ...marketplaceCatalogFixture,
        items: [{ ...marketplaceCatalogFixture.items[0], installed }],
        total: 1,
      });
    let releaseStaleRead!: (response: Response) => void;
    const staleRead = new Promise<Response>(resolve => {
      releaseStaleRead = resolve;
    });
    const fetch = vi
      .fn()
      .mockReturnValueOnce(staleRead)
      .mockImplementation(async () => listing(true));
    vi.stubGlobal("fetch", fetch);
    vi.mocked(installMarketplaceExtension).mockResolvedValue({ extension: extensionFixtures[0]! });
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(
      () => ({
        listing: useMarketplaceCatalog({ q: "otel" }),
        install: useInstallMarketplaceExtension(),
      }),
      { wrapper }
    );
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(result.current.listing.data).toBeUndefined();
    act(() => result.current.install.mutate({ ref: "compozy/otel-bridge", source: "curated" }));
    await waitFor(() =>
      expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] })
    );
    releaseStaleRead(listing(false));
    await waitFor(() => expect(result.current.install.isSuccess).toBe(true));
    await waitFor(() =>
      expect(result.current.listing.data?.pages[0]?.items[0]?.installed).toBe(true)
    );
  });

  it("Should invalidate marketplace and extension management after extension install", async () => {
    vi.mocked(installMarketplaceExtension).mockResolvedValue({
      extension: {
        contents: { skills: 0, mcp_servers: 0, hooks: 0, loops: 0, agents: 0, bridges: 0 },
        inputs: [],
        mcp_servers: [],
        missing_inputs: [],
        consecutive_failures: 0,
        daemon_running: false,
        digest_matched: true,
        enabled: false,
        format: "compozy",
        name: "review-pack",
        network_confirmation_required: false,
        profile: "default",
        restart_backoff_ms: 0,
        source: "marketplace",
        state: "installed",
        type: "native",
        update_available: false,
        version: "2.0.0",
      },
    });
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceExtension(), { wrapper });

    act(() => result.current.mutate({ ref: "review-pack", source: "curated", version: "2.0.0" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["extensions"] });
  });

  it("Should update the canonical installed extension and reconcile both cache families", async () => {
    vi.mocked(updateExtension).mockResolvedValue(undefined);
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useUpdateMarketplaceExtension(), { wrapper });

    act(() =>
      result.current.mutate({
        body: { allow_unverified: false, version: "2.0.0" },
        name: "manifest-review-pack",
      })
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(updateExtension).toHaveBeenCalledWith("manifest-review-pack", {
      allow_unverified: false,
      version: "2.0.0",
    });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["extensions"] });
  });
});
