// Invariant: one Marketplace surface reflects complete catalog and installed inventory truth.
// Owner: Marketplace components; canonical suite for browse/card/logo/shelf and source install.
// HTTP and notification boundaries are mocked; routing, query hooks, and UI primitives run normally.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createRootRoute,
  createRoute,
  createRouter,
  createMemoryHistory,
  Outlet,
  RouterProvider,
  useSearch,
  useParams,
} from "@tanstack/react-router";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { TopbarSlotProvider, useTopbarSlotValue } from "@compozy/ui";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";
import { extensionFixtures } from "@/systems/extensions/mocks";
import type { ExtensionEntry, InstalledExtensionView } from "@/systems/extensions";
import { marketplaceCatalogFixture } from "../../mocks";
import type { MarketplaceCatalogResponse } from "../../types";
import { MarketplaceApiError } from "../../adapters/marketplace-api-error";
import { validateMarketplaceSearch } from "../../lib/marketplace-search";
import { MarketplacePage } from "../marketplace-page";
import { MarketplaceInstalledPage } from "../marketplace-installed-page";
import { MarketplaceEntryLogo } from "../marketplace-entry-logo";
import { MarketplaceEntryCard } from "../marketplace-entry-card";
import { MarketplaceCatalogTrail } from "../marketplace-entry-trail";
import { MarketplaceInstalledShelf } from "../marketplace-installed-shelf";
import { useExtensionInstallDialog } from "../use-extension-install-dialog";
import { MarketplaceDetailLocation } from "@/systems/os/apps/marketplace/marketplace-detail-location";
import { MarketplaceWindow } from "@/systems/os/apps/marketplace/marketplace-window";
import type { OsWindow } from "@/systems/os/lib/os-types";
import { OsShellContext } from "@/systems/os/contexts/os-shell-context";
import { WindowManagerRuntime } from "@/systems/os/runtime/window-manager-runtime";
import { RoutingCoordinator } from "@/systems/os/lib/routing-coordinator";
import { validateMarketplaceDetailSearch } from "@/systems/os/apps/marketplace/marketplace-detail-search";
import { Route as BrowseRoute } from "@/routes/_app/marketplace.index";

const mocks = vi.hoisted(() => ({
  installExtension: vi.fn(),
  previewExtensionInstall: vi.fn(),
  toast: vi.fn(),
  readCatalog: vi.fn(),
}));
vi.mock("../../adapters/marketplace-actions-api", async original => ({
  ...(await original<typeof import("../../adapters/marketplace-actions-api")>()),
  installMarketplaceExtension: mocks.installExtension,
}));
vi.mock("@/systems/extensions/adapters/extensions-api", async original => ({
  ...(await original<typeof import("@/systems/extensions/adapters/extensions-api")>()),
  previewExtensionInstall: mocks.previewExtensionInstall,
}));
vi.mock("sonner", () => ({ toast: { success: mocks.toast, error: mocks.toast } }));
let catalog: MarketplaceCatalogResponse;
let extensions: ExtensionEntry[] = [];
const server = setupServer(
  ...profileHandlers,
  ...workspaceHandlers,
  ...statusHandlers,
  http.get("*/api/marketplace", ({ request }) => {
    mocks.readCatalog(new URL(request.url).searchParams);
    return HttpResponse.json(catalog);
  }),
  http.get("*/api/marketplace/entries/:entryId", () =>
    HttpResponse.json(
      { error: { code: "marketplace_entry_not_found", message: "Entry not found" } },
      { status: 404 }
    )
  ),
  http.get("*/api/extensions", () => HttpResponse.json({ extensions })),
  http.post("*/api/marketplace/refresh", () =>
    HttpResponse.json({
      sources: [{ source: "compozy-catalog", outcome: "refreshed", entry_count: 4, stale: false }],
    })
  )
);
const clients: QueryClient[] = [];
const managers: WindowManagerRuntime[] = [];
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterAll(() => server.close());
afterEach(() => {
  cleanup();
  managers.splice(0).forEach(manager => manager.destroy());
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
});
beforeEach(() => {
  vi.clearAllMocks();
  catalog = { ...marketplaceCatalogFixture };
  extensions = [];
  mocks.installExtension.mockReset().mockResolvedValue({ extension: extensionFixtures[0] });
  mocks.previewExtensionInstall
    .mockReset()
    .mockImplementation(async (request: { ref: string }) => ({
      inputs: [],
      declared_profiles: [{ create: false, credentials: [], name: "default" }],
      name: request.ref.split("/").pop(),
      placements: [],
    }));
});
function Head() {
  const slot = useTopbarSlotValue();
  return (
    <header>
      <span>{slot?.crumb}</span>
      <output aria-label="Catalog count">{slot?.count}</output>
      {slot?.actions}
      {slot?.toolbar}
    </header>
  );
}
function Shell() {
  return (
    <TopbarSlotProvider>
      <Head />
      <Outlet />
    </TopbarSlotProvider>
  );
}
function Browse() {
  return (
    <MarketplacePage
      search={validateMarketplaceSearch(useSearch({ strict: false, structuralSharing: false }))}
    />
  );
}
function Installed() {
  return (
    <MarketplaceInstalledPage
      search={validateMarketplaceSearch(useSearch({ strict: false, structuralSharing: false }))}
    />
  );
}
function Detail() {
  const { entryId } = useParams({ strict: false });
  return (
    <MarketplaceDetailLocation
      entryId={entryId!}
      search={validateMarketplaceDetailSearch(
        useSearch({ strict: false, structuralSharing: false })
      )}
      liveDataEnabled
    />
  );
}
function setup(node?: ReactNode, path = "/marketplace") {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  const root = createRootRoute({ component: Shell });
  const browse = createRoute({
    getParentRoute: () => root,
    path: "/marketplace",
    validateSearch: BrowseRoute.options.validateSearch,
    component: node === undefined ? Browse : () => node,
  });
  const installed = createRoute({
    getParentRoute: () => root,
    path: "/marketplace/installed",
    validateSearch: validateMarketplaceSearch,
    component: Installed,
  });
  const detail = createRoute({
    getParentRoute: () => root,
    path: "/marketplace/$entryId",
    component: node === undefined ? Detail : () => node,
  });
  const router = createRouter({
    routeTree: root.addChildren([browse, installed, detail]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  return { router, client, ...view };
}
function Installer() {
  const install = useExtensionInstallDialog();
  return (
    <>
      <button
        data-testid="marketplace-extension-install"
        onClick={() => install.open()}
        type="button"
      >
        Open installer
      </button>
      {install.dialogs}
    </>
  );
}
function renderInstaller() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  return render(
    <QueryClientProvider client={client}>
      <Installer />
    </QueryClientProvider>
  );
}
const entry = marketplaceCatalogFixture.items[0]!;
function installedItems(count: number): InstalledExtensionView[] {
  return Array.from({ length: count }, (_, i) => ({
    extension: { ...extensionFixtures[0]!, name: `local-${i}`, marketplace: null },
    listing: null,
    updateAvailable: false,
  }));
}

describe("Marketplace page and cards", () => {
  it("Should open one flat catalog with complete count, search, Refresh, and two Add choices", async () => {
    setup();
    await screen.findByTestId(`marketplace-card-${entry.entry_id}`);
    expect(screen.getByRole("status", { name: "Catalog count" })).toHaveTextContent(
      String(catalog.total)
    );
    expect(screen.getByRole("searchbox", { name: "Search extensions" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Refresh" })).toBeVisible();
    expect(screen.queryByTestId("marketplace-kind-navigation")).not.toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-section-compozy-catalog")).not.toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-installed-shelf")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findAllByRole("menuitem")).toHaveLength(2);
    await userEvent.click(screen.getByTestId("marketplace-add-github"));
    expect(await screen.findByTestId("extension-install-source-github")).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });
  it("Should keep server-matched entries and send the route query instead of re-filtering cards", async () => {
    setup(undefined, "/marketplace?q=semantic-match");
    expect(await screen.findByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
    expect(mocks.readCatalog.mock.calls.at(-1)?.[0].get("q")).toBe("semantic-match");
    const input = screen.getByRole("searchbox", { name: "Search extensions" });
    expect(input).toHaveValue("semantic-match");
    await userEvent.click(input);
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(input).toHaveValue(""));
  });
  it("Should focus the search shortcut without stealing typed text", async () => {
    setup();
    const input = await screen.findByRole("searchbox", { name: "Search extensions" });
    await userEvent.keyboard("/");
    expect(input).toHaveFocus();
    await userEvent.type(input, "foo/");
    expect(input).toHaveValue("foo/");
  });
  it("Should keep sources in daemon order and expose source sections only when there are several", async () => {
    catalog = {
      ...catalog,
      sources: [
        { name: "compozy-catalog", kind: "feed", state: "ok", count: 1 },
        { name: "team-z", kind: "plugin", state: "ok", count: 1 },
        { name: "team-a", kind: "plugin", state: "ok", count: 1 },
      ],
      items: [
        entry,
        { ...entry, entry_id: "z", source: "team-z", source_ref: "git:z" },
        { ...entry, entry_id: "a", source: "team-a", source_ref: "git:a" },
      ],
    };
    setup();
    await screen.findByTestId("marketplace-section-team-a");
    expect(
      screen.getAllByTestId(/^marketplace-section-/).map(section => section.dataset.testid)
    ).toEqual([
      "marketplace-section-compozy-catalog",
      "marketplace-section-team-z",
      "marketplace-section-team-a",
    ]);
  });
  it("Should preserve degraded source rows and identify their failed refresh", async () => {
    catalog = {
      ...catalog,
      sources: [
        { name: "compozy-catalog", kind: "feed", state: "ok", count: 1 },
        {
          name: "team",
          kind: "plugin",
          state: "degraded",
          count: 1,
          last_read_at: "2026-09-12T00:00:00Z",
        },
      ],
      items: [entry, { ...entry, entry_id: "cached-team", source: "team", source_ref: "git:team" }],
    };
    setup();
    const degraded = await screen.findByTestId("marketplace-section-team");
    expect(degraded).toHaveTextContent(/last read.*could not refresh/);
    expect(within(degraded).getByTestId("marketplace-card-cached-team")).toBeVisible();
    expect(screen.getByTestId("marketplace-section-compozy-catalog")).not.toHaveTextContent(
      "could not refresh"
    );
  });
  it("Should distinguish an empty catalog from a query with no matches", async () => {
    catalog = { ...catalog, items: [], total: 0 };
    const view = setup(undefined, "/marketplace?q=missing");
    expect(await screen.findByText("No extensions match this query")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Clear search" }));
    await waitFor(() => expect(view.router.state.location.searchStr).toBe(""));
    expect(await screen.findByText("No extensions yet")).toBeVisible();
  });
  it("Should keep a sideloaded shelf visible while the catalog is empty", async () => {
    catalog = { ...catalog, items: [], total: 0 };
    extensions = [{ ...extensionFixtures[0]!, marketplace: null, origin: null }];
    setup();
    expect(await screen.findByTestId("marketplace-installed-shelf")).toBeVisible();
    expect(await screen.findByText("No extensions yet")).toBeVisible();
  });
  it("Should report cached stale data while keeping its entries visible", async () => {
    catalog = {
      ...catalog,
      stale: true,
      sources: [
        { ...catalog.sources[0]!, state: "degraded", last_read_at: "2026-09-12T12:00:00Z" },
      ],
    };
    setup();
    expect(await screen.findByTestId("marketplace-stale")).toHaveTextContent(
      "Showing the catalog from"
    );
    expect(screen.getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
  });
  // Invariant: repeated failed retries preserve the cached catalog and emit one failure toast per five seconds.
  it("Should keep the stale catalog and throttle failed Retry notifications", async () => {
    catalog = { ...catalog, stale: true };
    let retries = 0;
    server.use(
      http.post("*/api/marketplace/refresh", () => {
        retries++;
        return HttpResponse.json({ error: "Feed timed out" }, { status: 503 });
      })
    );
    setup();
    await screen.findByTestId("marketplace-stale");
    const retry = screen.getByRole("button", { name: "Retry" });
    await userEvent.click(retry);
    await waitFor(() => expect(mocks.toast).toHaveBeenCalledWith("Feed timed out"));
    await waitFor(() => expect(retry).toBeEnabled());
    await userEvent.click(retry);
    await waitFor(() => expect(retries).toBe(2));
    await waitFor(() => expect(retry).toBeEnabled());
    expect(mocks.toast).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
    expect(screen.getByTestId("marketplace-stale")).toBeVisible();
  });
  it("Should show unreachable rather than an empty catalog when no projection could be read", async () => {
    server.use(
      http.get("*/api/marketplace", () =>
        HttpResponse.json({ error: "feed unavailable" }, { status: 400 })
      )
    );
    setup();
    expect(await screen.findByText("The marketplace is unreachable")).toBeVisible();
    expect(screen.getByTestId("marketplace-retry")).toBeVisible();
    expect(screen.queryByText("No extensions yet")).not.toBeInTheDocument();
  });
  it("Should expose a daemon diagnostic on a successful empty envelope as unreachable", async () => {
    catalog = {
      ...catalog,
      total: 0,
      items: [],
      stale: true,
      error_class: "network",
      error: "feed unavailable",
    };
    setup();
    expect(await screen.findByText("The marketplace is unreachable")).toBeVisible();
    expect(screen.queryByText("No extensions yet")).not.toBeInTheDocument();
  });

  it("Should render one mutually exclusive trail state and keep actions outside the detail link", async () => {
    const onInstall = vi.fn(),
      onUpdate = vi.fn();
    setup(
      <>
        <MarketplaceEntryCard
          entry={entry}
          description={entry.description}
          link={{ to: "/marketplace/$entryId", params: { entryId: entry.entry_id } }}
          trail={
            <MarketplaceCatalogTrail entry={entry} onInstall={onInstall} onUpdate={onUpdate} />
          }
        />
        <MarketplaceCatalogTrail
          entry={{ ...entry, entry_id: "already", installed: true }}
          onInstall={onInstall}
          onUpdate={onUpdate}
        />
        <MarketplaceCatalogTrail
          entry={{
            ...entry,
            entry_id: "blocked",
            installable: false,
            install_blocker: "Catalog policy",
          }}
          onInstall={onInstall}
          onUpdate={onUpdate}
        />
        <MarketplaceCatalogTrail
          entry={{ ...entry, entry_id: "updatable", update_available: true }}
          onInstall={onInstall}
          onUpdate={onUpdate}
        />
        <MarketplaceCatalogTrail
          entry={{ ...entry, entry_id: "busy" }}
          pending
          onInstall={onInstall}
          onUpdate={onUpdate}
        />
      </>
    );
    const install = await screen.findByRole("button", { name: `Install ${entry.name}` });
    expect(install.closest("a")).toBeNull();
    await userEvent.click(install);
    expect(onInstall).toHaveBeenCalledWith(entry);
    expect(screen.getByTestId("marketplace-installed-already")).toHaveTextContent("Installed");
    expect(screen.getByTestId("marketplace-blocked-blocked")).toHaveAttribute(
      "title",
      "Catalog policy"
    );
    expect(screen.getByTestId("marketplace-action-busy")).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: `Update ${entry.name}` }));
    expect(onUpdate).toHaveBeenCalledTimes(1);
  });
  it.each([0, 5, 9])(
    "Should render the Installed shelf count and bounded logos for %i installations",
    async count => {
      const view = setup(
        <div data-testid="shelf-host">
          <MarketplaceInstalledShelf items={installedItems(count)} updates={1} />
        </div>
      );
      await screen.findByTestId("shelf-host");
      const shelf = screen.queryByTestId("marketplace-installed-shelf");
      if (count === 0) {
        expect(shelf).toBeNull();
        return;
      }
      expect(shelf).toHaveAttribute("href", "/marketplace/installed");
      expect(shelf).toHaveAccessibleName(`${count} installed, 1 update available. Open Installed`);
      expect(view.container.querySelectorAll('[data-slot="marketplace-entry-logo"]')).toHaveLength(
        Math.min(count, 6)
      );
      if (count === 9) expect(within(shelf!).getByText("+3")).toBeVisible();
    }
  );
  // UT-033: retired paths never redirect; normal not-found provides a deliberate recovery action.
  it.each(["skills", "mcps", "extensions"])(
    "Should keep the retired %s location until Back is selected",
    async segment => {
      const { router } = setup(undefined, `/marketplace/${segment}?q=x`);
      const back = await screen.findByRole("button", { name: "Back to marketplace" });
      expect(router.state.location.pathname).toBe(`/marketplace/${segment}`);
      await userEvent.click(back);
      await screen.findByTestId("marketplace-page");
      expect(router.state.location.pathname).toBe("/marketplace");
      expect(router.state.location.search).toEqual({ q: "x" });
    }
  );
  it("Should render Browse without interpreting the retired tab query", async () => {
    const { router } = setup(undefined, "/marketplace?tab=market&q=x");
    await screen.findByTestId("marketplace-page");
    expect(router.state.location.pathname).toBe("/marketplace");
    expect(screen.getByRole("searchbox", { name: "Search extensions" })).toHaveValue("x");
  });
  it.each(["/marketplace/skills", "/marketplace/mcp/github"])(
    "Should reopen %s without rewriting window state",
    async pathname => {
      const manager = new WindowManagerRuntime(new QueryClient());
      managers.push(manager);
      const initial = manager.projectionAtom.get();
      const window: OsWindow = {
        id: "catalog-window",
        app: "marketplace" as const,
        instanceKey: null,
        route: { pathname, search: { q: "saved" } },
        navStack: [],
        pinned: false,
        desktopId: "saved-desktop",
        placement: "floating" as const,
        rect: { x: 80, y: 90, w: 900, h: 600 },
        layer: 2,
        minimized: false,
        zoomed: false,
        groupId: null,
        nodeId: null,
        stackId: null,
        stackActive: true,
        parentAxis: null,
      };
      manager.projectionAtom.set({ ...initial, windows: { [window.id]: window } });
      const shell = {
        projection: manager.projectionAtom,
        manager,
        coordinator: new RoutingCoordinator(manager, { navigate: () => {}, replace: () => {} }),
      };
      const { router } = setup(
        <OsShellContext value={shell}>
          <MarketplaceWindow windowId={window.id} />
        </OsShellContext>,
        "/marketplace/saved"
      );
      const back = await screen.findByRole("button", { name: "Back to marketplace" });
      expect(manager.projectionAtom.get().windows[window.id]).toEqual(window);
      await userEvent.click(back);
      await waitFor(() => expect(router.state.location.pathname).toBe("/marketplace"));
      expect(router.state.location.search).toEqual({ q: "saved" });
    }
  );
  it("Should fall through a failed feed image to a brand mark", async () => {
    const view = setup(
      <MarketplaceEntryLogo
        entry={{ entry_id: "github", name: "GitHub", icon: "https://example.test/logo.svg" }}
      />
    );
    await waitFor(() => expect(view.container.querySelector("img")).not.toBeNull());
    const image = view.container.querySelector("img")!;
    expect(image).toHaveAttribute("referrerpolicy", "no-referrer");
    fireEvent.error(image);
    expect(view.container.querySelector('[data-rung="brand"]')).not.toBeNull();
    expect(view.container.querySelector("img")).toBeNull();
  });
  // Invariant: logo fallback follows the package identity and retries a replacement URL.
  it("Should recognize a known brand from the installation slug tail", () => {
    const view = render(
      <MarketplaceEntryLogo
        entry={{
          entry_id: "third-party-kit",
          name: "Slack tools",
          install_slug: "owner/slack-tools",
        }}
      />
    );
    expect(view.container.querySelector('[data-rung="brand"]')).not.toBeNull();
    expect(view.container.querySelector('[data-rung="marble"]')).toBeNull();
  });
  it("Should retry a new icon URL after the previous image failed", () => {
    const item = { entry_id: "github", name: "GitHub", icon: "https://example.test/old.svg" };
    const view = render(<MarketplaceEntryLogo entry={item} />);
    fireEvent.error(view.container.querySelector("img")!);
    expect(view.container.querySelector("img")).toBeNull();
    view.rerender(
      <MarketplaceEntryLogo entry={{ ...item, icon: "https://example.test/new.svg" }} />
    );
    expect(view.container.querySelector("img")).toHaveAttribute(
      "src",
      "https://example.test/new.svg"
    );
  });
  it("Should render unknown-entry marbles with separate SVG ids for duplicate seeds", async () => {
    const tile = { entry_id: "unknown-package", name: "Unknown" };
    const view = setup(
      <>
        <MarketplaceEntryLogo entry={tile} size="sm" />
        <MarketplaceEntryLogo entry={tile} />
        <MarketplaceEntryLogo entry={tile} size="lg" />
      </>
    );
    await waitFor(() =>
      expect(view.container.querySelectorAll('[data-rung="marble"] svg')).toHaveLength(3)
    );
    const ids = Array.from(view.container.querySelectorAll("svg [id]")).map(node => node.id);
    expect(new Set(ids).size).toBe(ids.length);
    const paths = Array.from(view.container.querySelectorAll('[data-rung="marble"] svg')).map(svg =>
      Array.from(svg.querySelectorAll("path")).map(path => [
        path.getAttribute("d"),
        path.getAttribute("fill"),
      ])
    );
    expect(paths[1]).toEqual(paths[0]);
    expect(paths[2]).toEqual(paths[0]);
  });
  it("Should block installed row actions during a batch and reconcile each partial outcome", async () => {
    const one = {
      ...entry,
      entry_id: "one",
      name: "One",
      installed_name: "one",
      installed: true,
      update_available: true,
    };
    const two = { ...one, entry_id: "two", name: "Two", installed_name: "two" };
    extensions = [one, two].map(listing => ({
      ...extensionFixtures[0]!,
      name: listing.entry_id,
      marketplace: listing,
      update_available: true,
    }));
    let finish!: () => void;
    const pending = new Promise<void>(resolve => {
      finish = resolve;
    });
    const bodies: unknown[] = [];
    server.use(
      http.post("*/api/extensions/update", async ({ request }) => {
        bodies.push(await request.json());
        await pending;
        extensions = extensions.map(extension =>
          extension.name === "one"
            ? {
                ...extension,
                update_available: false,
                marketplace: { ...one, update_available: false },
              }
            : extension
        );
        return HttpResponse.json({
          updates: [
            { name: "one", status: "updated" },
            {
              name: "two",
              status: "failed",
              error: { code: "source_unavailable", message: "artifact unavailable" },
            },
          ],
        });
      })
    );
    setup(undefined, "/marketplace/installed");
    const batch = await screen.findByRole("button", { name: "Update all" });
    await userEvent.click(batch);
    await waitFor(() => expect(bodies).toEqual([{ names: ["one", "two"] }]));
    try {
      expect(batch).toBeDisabled();
      expect(screen.getByTestId("marketplace-installed-update-one")).toBeDisabled();
      expect(screen.getByTestId("marketplace-installed-update-two")).toBeDisabled();
      expect(screen.getByTestId("marketplace-installed-switch-two")).toHaveAttribute(
        "aria-disabled",
        "true"
      );
    } finally {
      await act(async () => finish());
    }
    await waitFor(() =>
      expect(screen.queryByTestId("marketplace-installed-update-one")).not.toBeInTheDocument()
    );
    await waitFor(() =>
      expect(screen.getByTestId("marketplace-installed-update-two")).toBeEnabled()
    );
    expect(mocks.toast).toHaveBeenCalledWith("two: artifact unavailable");
    expect(mocks.toast).toHaveBeenCalledWith("1 of 2 updated");
  });

  it("Should remove only after the exact installed name is confirmed and refresh the inventory", async () => {
    extensions = [{ ...extensionFixtures[0]!, name: "local-kit", marketplace: null, origin: null }];
    const removed: string[] = [];
    server.use(
      http.delete("*/api/extensions/:name", ({ params }) => {
        removed.push(String(params.name));
        extensions = [];
        return new HttpResponse(null, { status: 204 });
      })
    );
    setup(undefined, "/marketplace/installed");
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "More for local-kit" }));
    await user.click(await screen.findByRole("menuitem", { name: "Remove…" }));
    const confirm = await screen.findByTestId("remove-extension-confirm");
    await user.type(screen.getByLabelText("Type to confirm"), "local");
    expect(confirm).toBeDisabled();
    expect(removed).toEqual([]);
    await user.type(screen.getByLabelText("Type to confirm"), "-kit");
    expect(confirm).toBeEnabled();
    await user.click(confirm);
    await waitFor(() => expect(removed).toEqual(["local-kit"]));
    expect(await screen.findByText("No extensions installed yet")).toBeVisible();
  });

  // Invariant: Installed is a flat name-ordered inventory, including sideloads and scope metadata.
  it("Should order installed names and show contents and workspace only where applicable", async () => {
    const emptyContents = { skills: 0, mcp_servers: 0, agents: 0, loops: 0, hooks: 0, bridges: 0 };
    extensions = [
      {
        ...extensionFixtures[0]!,
        name: "Zulu",
        marketplace: null,
        origin: null,
        contents: emptyContents,
        workspace_id: undefined,
        update_available: false,
      },
      {
        ...extensionFixtures[0]!,
        name: "Alpha",
        marketplace: null,
        origin: null,
        contents: { ...emptyContents, mcp_servers: 1, skills: 2 },
        workspace_id: "ws-a",
        update_available: false,
      },
    ];
    setup(undefined, "/marketplace/installed");
    await screen.findByTestId("marketplace-installed-card-Alpha");
    const rows = screen.getAllByTestId(/^marketplace-installed-card-/);
    expect(rows.map(row => row.dataset.testid)).toEqual([
      "marketplace-installed-card-Alpha",
      "marketplace-installed-card-Zulu",
    ]);
    expect(rows[0]).toHaveTextContent("1 MCP server · 2 skills");
    expect(rows[0]).toHaveTextContent("workspace · ws-a");
    expect(rows[1]).not.toHaveTextContent("MCP server");
    expect(rows[1]).not.toHaveTextContent("workspace ·");
  });
  it("Should distinguish Installed empties from filtered inventory", async () => {
    const view = setup(undefined, "/marketplace/installed");
    expect(await screen.findByText("No extensions installed yet")).toBeVisible();
    extensions = [{ ...extensionFixtures[0]!, marketplace: null, origin: null }];
    await act(async () => {
      await view.router.navigate({ to: "/marketplace/installed", search: { q: "missing" } });
      await view.client.invalidateQueries();
    });
    expect(await screen.findByText("No installed extensions match this query")).toBeVisible();
  });
});

describe("Extension source installation", () => {
  it("Should pin the previewed local artifact through explicit source consent", async () => {
    const user = userEvent.setup();
    mocks.previewExtensionInstall.mockResolvedValueOnce({
      digest_sha256: "a".repeat(64),
      inputs: [],
      declared_profiles: [{ create: true, credentials: [], name: "operations" }],
      name: "gen-a1b2c3",
      network_requirement_digest: "sha256:local-network",
      placements: [],
    });
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "relative/dist");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "absolute path"
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.clear(screen.getByTestId("extension-install-ref"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-allow-unverified"));
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    expect(mocks.installExtension).not.toHaveBeenCalled();
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("extension-trust-confirm"));

    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        allow_unverified: true,
        expected_digest: "a".repeat(64),
        confirm_network_digest: "sha256:local-network",
        ref: "/srv/hello/dist/gen-a1b2c3",
        source: "local_path",
      })
    );
  });

  it("Should reject a GitHub reference with an empty tag before the request", async () => {
    const user = userEvent.setup();
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-source-github"));
    await user.type(screen.getByTestId("extension-install-ref"), "acme/hello@");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "owner/repo"
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();
  });

  it("Should accept only credential-free HTTPS Git repository URLs", async () => {
    const user = userEvent.setup();
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-source-git"));
    const repositoryHelp = screen.getByRole("button", { name: "About repository url" });
    expect(repositoryHelp).toBeInTheDocument();
    await user.hover(repositoryHelp);
    expect(
      await screen.findByText(
        "A public HTTPS repository URL. Add a branch, tag, or commit in Version."
      )
    ).toBeInTheDocument();

    const ref = screen.getByTestId("extension-install-ref");
    await user.type(ref, "ssh://git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "public HTTPS repository URL"
    );

    await user.clear(ref);
    await user.type(ref, "https://@git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Remove credentials"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com/acme/hello.git?");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Put the Git ref in the Version field"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com./acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "valid host and repository path"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com/acme/hello.git@v1.2.3");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    await user.click(screen.getByTestId("extension-install-submit"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        ref: "https://git.example.com/acme/hello.git@v1.2.3",
        source: "git",
      })
    );
  });

  it("Should open consent only for the daemon checksum diagnostic", async () => {
    const user = userEvent.setup();
    mocks.installExtension
      .mockRejectedValueOnce(
        new MarketplaceApiError(
          "Extension checksum is not registry-verified",
          422,
          "extension_checksum_unverified"
        )
      )
      .mockResolvedValueOnce({});
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    expect(mocks.installExtension).toHaveBeenCalledWith({
      ref: "/srv/hello/dist/gen-a1b2c3",
      source: "local_path",
    });

    await user.click(screen.getByTestId("extension-trust-confirm"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenLastCalledWith({
        allow_unverified: true,
        ref: "/srv/hello/dist/gen-a1b2c3",
        source: "local_path",
      })
    );
  });

  it("Should keep a policy-blocked install on the form instead of offering consent", async () => {
    const user = userEvent.setup();
    mocks.installExtension.mockRejectedValueOnce(
      new MarketplaceApiError(
        "Unverified extension install is blocked by policy",
        422,
        "extension_unverified_policy_blocked"
      )
    );
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-error")).toHaveTextContent(
      "blocked by policy"
    );
    expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument();
  });
});
