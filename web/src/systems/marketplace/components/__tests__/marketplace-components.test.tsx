import {
  createExtensionInputDraft,
  prepareExtensionInputs,
  type ExtensionInputDefinitions,
} from "../extension-install-model";
// Invariant: one Marketplace surface reflects complete catalog and installed inventory truth.
// Owner: Marketplace components; canonical suite for browse/card/logo/shelf and source install.
// HTTP and notification boundaries are mocked; routing, query hooks, and UI primitives run normally.
import { onlineManager, Query, QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  type MockInstance,
  vi,
} from "vitest";
import type { ReactNode } from "react";
import { TopbarSlotProvider, useTopbarSlotValue } from "@compozy/ui";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers, statusFixture } from "@/systems/status/mocks";
import { extensionFixtures } from "@/systems/extensions/mocks";
import type { ExtensionEntry, InstalledExtensionView } from "@/systems/extensions";
import {
  marketplaceCatalogDetailFixture,
  marketplaceCatalogFixture,
  marketplaceCategoryCounts,
  marketplaceSectionsFromCatalog,
} from "../../mocks";
import type { MarketplaceCatalogResponse } from "../../types";
import { MarketplaceApiError } from "../../adapters/marketplace-api-error";
import { validateMarketplaceSearch } from "../../lib/marketplace-search";
import {
  mcpAuthBeginFixture,
  mcpExtensionServerFixtures,
  mcpManagementServerFixtures,
  settingsAppliedMutationFixture,
  settingsApplyRecordsFixture,
  settingsMCPServersCollectionFixture,
} from "@/systems/settings/mocks/fixtures";
import { handlers as vaultHandlers } from "@/systems/vault/mocks";
import { settingsKeys } from "@/systems/settings/lib/query-keys";
import { marketplaceKeys } from "../../lib/query-keys";
import { MarketplaceExtensionServerSection } from "../marketplace-detail-extension-server";
import {
  mcpServerRowTestId,
  SettingsMarketplaceSourcesSection,
  type SettingsMCPServerEntry,
} from "@/systems/settings";
import { AddMarketplaceDialog } from "../add-marketplace-dialog";
import { MarketplacePage } from "../marketplace-page";
import { MarketplaceInstalledPage } from "../marketplace-installed-page";
import { MarketplaceCategoryPage } from "../marketplace-category-page";
import { MarketplaceEntryLogo } from "../marketplace-entry-logo";
import { MarketplaceEntryCard } from "../marketplace-entry-card";
import { MarketplaceCatalogTrail } from "../marketplace-entry-trail";
import type { MarketplaceSetupState } from "../../lib/marketplace-setup-state";
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
  readSections: vi.fn(),
  readExtensions: vi.fn(),
  readMCPServers: vi.fn(),
  putMCPServer: vi.fn(),
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
let mcpServers: SettingsMCPServerEntry[] = [];
const server = setupServer(
  ...profileHandlers,
  ...workspaceHandlers,
  ...statusHandlers,
  ...vaultHandlers,
  http.get("*/api/settings/mcp-servers", () => {
    mocks.readMCPServers();
    return HttpResponse.json({ ...settingsMCPServersCollectionFixture, mcp_servers: mcpServers });
  }),
  http.put("*/api/settings/mcp-servers/:name", async ({ params, request }) => {
    const name = String(params.name);
    mocks.putMCPServer(name, new URL(request.url).searchParams, await request.json());
    mcpServers = [
      ...mcpServers,
      // A personal settings-file server like `buildkite`, renamed to the saved one.
      { ...mcpManagementServerFixtures.find(server => server.name === "buildkite")!, name },
    ];
    return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
  }),
  http.get("*/api/marketplace", ({ request }) => {
    mocks.readCatalog(new URL(request.url).searchParams);
    return HttpResponse.json(catalog);
  }),
  // Browse home: the daemon groups the same rows the list returns into category sections.
  http.get("*/api/marketplace/sections", ({ request }) => {
    mocks.readSections(new URL(request.url).searchParams);
    return HttpResponse.json(marketplaceSectionsFromCatalog(catalog));
  }),
  http.get("*/api/marketplace/entries/:entryId", () =>
    HttpResponse.json(
      { error: { code: "marketplace_entry_not_found", message: "Entry not found" } },
      { status: 404 }
    )
  ),
  http.get("*/api/settings/apply", () => HttpResponse.json(settingsApplyRecordsFixture)),
  http.get("*/api/extensions", () => {
    mocks.readExtensions();
    return HttpResponse.json({ extensions });
  }),
  http.post("*/api/marketplace/refresh", () =>
    HttpResponse.json({
      sources: [{ source: "compozy-catalog", outcome: "refreshed", entry_count: 4, stale: false }],
    })
  )
);
const clients: QueryClient[] = [];
const managers: WindowManagerRuntime[] = [];
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterAll(() => server.close());
afterEach(() => {
  cleanup();
  managers.splice(0).forEach(manager => manager.destroy());
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
});
beforeEach(() => {
  vi.clearAllMocks();
  // One row per entry on Browse: Featured is exercised by its own case (UT-020).
  catalog = {
    ...marketplaceCatalogFixture,
    items: marketplaceCatalogFixture.items.map(item => ({ ...item, featured: false })),
  };
  extensions = [];
  mcpServers = [...mcpManagementServerFixtures];
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
function Category() {
  const { category } = useParams({ strict: false });
  return (
    <MarketplaceCategoryPage
      category={category!}
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
  const category = createRoute({
    getParentRoute: () => root,
    path: "/marketplace/categories/$category",
    validateSearch: validateMarketplaceSearch,
    component: Category,
  });
  const router = createRouter({
    routeTree: root.addChildren([browse, installed, detail, category]),
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
  it("Should highlight literal query occurrences without changing the card text or destination", async () => {
    setup(
      <MarketplaceEntryCard
        entry={{ entry_id: "ops-tools", name: "Ops [tools]" }}
        description="Use [TOOLS] alongside other tools."
        query="[tools]"
        link={{ to: "/marketplace/$entryId", params: { entryId: "ops-tools" } }}
        trail={<button type="button">Install</button>}
      />
    );
    const title = await screen.findByRole("link", { name: "View Ops [tools] details" });
    expect(title).toHaveAttribute("href", "/marketplace/ops-tools");
    expect(title).toHaveTextContent("Ops [tools]");
    expect(screen.getAllByRole("mark").map(mark => mark.textContent)).toEqual([
      "[tools]",
      "[TOOLS]",
    ]);
    expect(screen.getByText("Use", { exact: false })).toHaveTextContent(
      "Use [TOOLS] alongside other tools."
    );
  });

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
    expect(mocks.readMCPServers).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findAllByRole("menuitem")).toHaveLength(4);
    await userEvent.click(screen.getByTestId("marketplace-add-github"));
    expect(await screen.findByTestId("extension-install-source-github")).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });
  it("Should switch Browse to MCPs keeping the query, counting only servers, and adding one", async () => {
    const { router } = setup(undefined, "/marketplace?q=git");
    await screen.findByTestId("marketplace-page");
    const plugins = screen.getByTestId("marketplace-kind-plugins");
    expect(plugins).toHaveAttribute("aria-pressed", "true");
    // Search first, then the kind toggle, like every other listing toolbar.
    expect(
      screen
        .getByRole("searchbox", { name: "Search extensions" })
        .compareDocumentPosition(plugins) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    mocks.readCatalog.mockClear();
    mocks.readExtensions.mockClear();

    await userEvent.click(screen.getByTestId("marketplace-kind-mcps"));
    await waitFor(() => expect(router.state.location.search).toEqual({ kind: "mcps", q: "git" }));
    expect(screen.getByRole("searchbox", { name: "Search MCP servers" })).toHaveValue("git");
    const list = await screen.findByTestId("settings-page-mcp-servers-list");
    for (const server of mcpManagementServerFixtures) {
      const row = within(list).queryByTestId(mcpServerRowTestId(server));
      if (server.name.includes("git")) expect(row).toBeVisible();
      else expect(row).not.toBeInTheDocument();
    }
    expect(screen.getByRole("status", { name: "Catalog count" })).toHaveTextContent(
      String(mcpManagementServerFixtures.length)
    );
    expect(screen.queryByRole("button", { name: "Refresh" })).not.toBeInTheDocument();
    expect(mocks.readCatalog).not.toHaveBeenCalled();
    expect(mocks.readExtensions).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Add MCP server" }));
    const editor = await screen.findByTestId("settings-mcp-servers-editor");
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-name-input"),
      "github-team"
    );
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-command-input"),
      "npx"
    );
    await userEvent.click(within(editor).getByTestId("settings-mcp-servers-editor-save"));
    await waitFor(() => expect(mocks.putMCPServer).toHaveBeenCalledTimes(1));
    const [savedName, savedSearch] = mocks.putMCPServer.mock.calls[0]!;
    expect(savedName).toBe("github-team");
    expect(savedSearch.get("scope")).toBe("user");
    expect(
      await within(screen.getByTestId("settings-page-mcp-servers-list")).findByTestId(
        "settings-page-mcp-servers-row-github-team"
      )
    ).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByTestId("settings-mcp-servers-editor")).not.toBeInTheDocument()
    );

    await userEvent.click(screen.getByTestId("marketplace-kind-plugins"));
    await waitFor(() => expect(router.state.location.search).toEqual({ q: "git" }));
    expect(await screen.findByRole("searchbox", { name: "Search extensions" })).toHaveValue("git");
  });
  it("Should hold a new MCP server's save until the server list has loaded", async () => {
    let release = () => {};
    const listed = new Promise<void>(resolve => {
      release = resolve;
    });
    server.use(
      http.get("*/api/settings/mcp-servers", async () => {
        await listed;
        return HttpResponse.json({
          ...settingsMCPServersCollectionFixture,
          mcp_servers: mcpServers,
        });
      })
    );
    setup();
    await screen.findByTestId(`marketplace-card-${entry.entry_id}`);
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await userEvent.click(await screen.findByTestId("marketplace-add-mcp-server"));
    const editor = await screen.findByTestId("settings-mcp-servers-editor");
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-name-input"),
      "buildkite"
    );
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-command-input"),
      "npx"
    );
    expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    release();
    expect(
      await within(editor).findByText('An MCP server named "buildkite" already exists.')
    ).toBeVisible();
    expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    expect(mocks.putMCPServer).not.toHaveBeenCalled();
  });
  it("Should check a new MCP server's name against a list refetched when the editor opens", async () => {
    setup(undefined, "/marketplace?kind=mcps");
    await screen.findByTestId("settings-page-mcp-servers-list");
    // Added outside the web app after the list was cached.
    mcpServers = [
      ...mcpServers,
      {
        ...mcpManagementServerFixtures.find(server => server.name === "buildkite")!,
        name: "outside",
      },
    ];
    await userEvent.click(screen.getByRole("button", { name: "Add MCP server" }));
    const editor = await screen.findByTestId("settings-mcp-servers-editor");
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-name-input"),
      "outside"
    );
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-command-input"),
      "npx"
    );
    expect(
      await within(editor).findByText('An MCP server named "outside" already exists.')
    ).toBeVisible();
    expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
  });
  it("Should keep a new MCP server's save blocked while its list refetch is paused offline", async () => {
    setup(undefined, "/marketplace?kind=mcps");
    await screen.findByTestId("settings-page-mcp-servers-list");
    onlineManager.setOnline(false);
    try {
      await userEvent.click(screen.getByRole("button", { name: "Add MCP server" }));
      const editor = await screen.findByTestId("settings-mcp-servers-editor");
      await userEvent.type(
        within(editor).getByTestId("settings-mcp-servers-editor-name-input"),
        "offline-one"
      );
      await userEvent.type(
        within(editor).getByTestId("settings-mcp-servers-editor-command-input"),
        "npx"
      );
      expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    } finally {
      onlineManager.setOnline(true);
    }
    await waitFor(() =>
      expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeEnabled()
    );
  });
  it("Should explain a failed server list inside the editor and retry it without losing the draft", async () => {
    setup(undefined, "/marketplace?kind=mcps");
    await screen.findByTestId("settings-page-mcp-servers-list");
    let failing = true;
    server.use(
      http.get("*/api/settings/mcp-servers", () =>
        failing
          ? // 403 is the settings read the query does not retry, so the failure settles at once.
            HttpResponse.json(
              { error: { code: "forbidden", message: "profile is read-only" } },
              { status: 403 }
            )
          : HttpResponse.json({ ...settingsMCPServersCollectionFixture, mcp_servers: mcpServers })
      )
    );
    await userEvent.click(screen.getByRole("button", { name: "Add MCP server" }));
    const editor = await screen.findByTestId("settings-mcp-servers-editor");
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-name-input"),
      "fresh-one"
    );
    await userEvent.type(
      within(editor).getByTestId("settings-mcp-servers-editor-command-input"),
      "npx"
    );
    const alert = await within(editor).findByTestId("settings-mcp-servers-editor-servers-error");
    expect(alert).toHaveTextContent("Couldn't check existing server names.");
    expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    failing = false;
    await userEvent.click(within(alert).getByRole("button", { name: "Try again" }));
    await waitFor(() =>
      expect(
        within(editor).queryByTestId("settings-mcp-servers-editor-servers-error")
      ).not.toBeInTheDocument()
    );
    expect(within(editor).getByTestId("settings-mcp-servers-editor-name-input")).toHaveValue(
      "fresh-one"
    );
    expect(within(editor).getByTestId("settings-mcp-servers-editor-save")).toBeEnabled();
  });
  it("Should carry a pending search draft into the selected kind instead of reverting it", async () => {
    const { router } = setup();
    await screen.findByTestId("marketplace-page");
    fireEvent.change(screen.getByRole("searchbox", { name: "Search extensions" }), {
      target: { value: "sentry" },
    });
    await userEvent.click(screen.getByTestId("marketplace-kind-mcps"));
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ kind: "mcps", q: "sentry" })
    );
    await new Promise(resolve => setTimeout(resolve, 300));
    expect(router.state.location.search).toEqual({ kind: "mcps", q: "sentry" });
    expect(screen.getByRole("searchbox", { name: "Search MCP servers" })).toHaveValue("sentry");
  });
  it("Should offer the daemon restart in the MCPs view when a saved change needs one", async () => {
    server.use(
      http.get("*/api/status", () =>
        HttpResponse.json({
          ...statusFixture,
          config: { ...statusFixture.config, restart_required: true },
        })
      )
    );
    setup(undefined, "/marketplace?kind=mcps");
    expect(await screen.findByTestId("settings-page-mcp-restart-notice")).toBeVisible();
    expect(screen.getByRole("button", { name: "Restart CompozyOS" })).toBeVisible();
  });
  it("Should drop the MCPs restart notice and its polling while the window is hidden", async () => {
    server.use(
      http.get("*/api/status", () =>
        HttpResponse.json({
          ...statusFixture,
          config: { ...statusFixture.config, restart_required: true },
        })
      )
    );
    setup(<MarketplacePage liveDataEnabled={false} search={{ kind: "mcps" }} />);
    expect(await screen.findByTestId("marketplace-page")).toHaveAttribute("data-kind", "mcps");
    expect(screen.queryByTestId("settings-page-mcp-restart-notice")).not.toBeInTheDocument();
  });
  it("Should open the MCP server editor over the MCPs view from Add ▾", async () => {
    const { router } = setup();
    await screen.findByTestId(`marketplace-card-${entry.entry_id}`);
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await userEvent.click(await screen.findByTestId("marketplace-add-mcp-server"));
    await waitFor(() => expect(router.state.location.search).toEqual({ kind: "mcps" }));
    expect(await screen.findByTestId("settings-mcp-servers-editor")).toBeVisible();
    expect(screen.getByTestId("marketplace-page")).toHaveAttribute("data-kind", "mcps");
  });
  it("Should hold the shelf line and the grid as placeholders until the inventory and the catalog arrive", async () => {
    let releaseInventory!: () => void;
    let releaseCatalog!: () => void;
    const inventory = new Promise<void>(resolve => (releaseInventory = resolve));
    const catalogRead = new Promise<void>(resolve => (releaseCatalog = resolve));
    extensions = [{ ...extensionFixtures[0]!, name: "local-0", marketplace: null }];
    server.use(
      http.get("*/api/extensions", async () => {
        await inventory;
        return HttpResponse.json({ extensions });
      }),
      http.get("*/api/marketplace/sections", async () => {
        await catalogRead;
        return HttpResponse.json(marketplaceSectionsFromCatalog(catalog));
      })
    );
    setup();

    // Both reads are pending: one busy region per area — three placeholder sections of six rows —
    // with the shelf's line reserved so the sections do not drop when the installed shelf arrives.
    const grids = await screen.findAllByTestId("marketplace-grid-skeleton");
    expect(grids).toHaveLength(3);
    expect(grids[0]!.closest('[aria-busy="true"]')).toHaveTextContent(
      "Loading marketplace entries"
    );
    expect(screen.getByTestId("marketplace-installed-shelf-skeleton")).toHaveAttribute(
      "aria-busy",
      "true"
    );
    expect(screen.queryByTestId("marketplace-installed-shelf")).not.toBeInTheDocument();

    releaseInventory();
    releaseCatalog();
    expect(await screen.findByTestId("marketplace-installed-shelf")).toBeVisible();
    await screen.findByTestId(`marketplace-card-${entry.entry_id}`);
    expect(screen.queryByTestId("marketplace-grid-skeleton")).not.toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-installed-shelf-skeleton")).not.toBeInTheDocument();
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
  // UT-020 [US-001.AC-1, US-004.AC-1]: sections follow the daemon response, never the sources.
  it("Should render Featured then category sections in response order with daemon labels [UT-020]", async () => {
    catalog = {
      ...catalog,
      sources: [
        { name: "compozy-catalog", kind: "feed", state: "ok", count: 2 },
        { name: "team-a", kind: "plugin", state: "ok", count: 1 },
      ],
      items: [
        { ...entry, featured: true, category: "development", category_label: "Development" },
        {
          ...entry,
          entry_id: "notes",
          name: "notes",
          category: "productivity",
          category_label: "Productivity",
          source: "team-a",
          source_ref: "git:a",
        },
      ],
    };
    setup();
    await screen.findByTestId("marketplace-category-productivity");
    const regions = screen.getAllByRole("region");
    expect(regions.map(region => region.getAttribute("aria-label"))).toEqual([
      "Featured",
      "Productivity",
      "Development",
    ]);
    expect(within(regions[0]!).getByRole("heading", { name: "Featured" })).toBeVisible();
    expect(within(regions[0]!).getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
    expect(within(regions[2]!).getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
    // Compact rows drop the word their category heading already says; Featured mixes categories.
    expect(
      within(within(regions[0]!).getByTestId(`marketplace-card-${entry.entry_id}`)).getByText(
        "Development"
      )
    ).toBeVisible();
    expect(
      within(within(regions[2]!).getByTestId(`marketplace-card-${entry.entry_id}`)).queryByText(
        "Development"
      )
    ).toBeNull();
    expect(screen.queryByTestId(/^marketplace-section-/)).toBeNull();
    expect(screen.queryByText("team-a")).toBeNull();
    // The topbar counts entries once each, even when Featured repeats one.
    expect(screen.getByRole("status", { name: "Catalog count" })).toHaveTextContent("2");
  });
  // UT-021 [US-001.AC-2]: six rows, and View all names the category with the daemon's full count.
  it("Should cut a section at six rows and link View all with the full count [UT-021]", async () => {
    const productivity = Array.from({ length: 7 }, (_, index) => ({
      ...entry,
      entry_id: `p-${index}`,
      name: `p-${index}`,
      category: "productivity",
      category_label: "Productivity",
    }));
    catalog = { ...catalog, items: productivity };
    server.use(
      http.get("*/api/marketplace/sections", ({ request }) => {
        mocks.readSections(new URL(request.url).searchParams);
        return HttpResponse.json({
          ...marketplaceSectionsFromCatalog(catalog),
          sections: [
            {
              id: "productivity",
              label: "Productivity",
              total: 58,
              items: productivity.slice(0, 6),
            },
          ],
        });
      })
    );
    const { router } = setup();
    const section = await screen.findByRole("region", { name: "Productivity" });
    expect(within(section).getAllByTestId(/^marketplace-card-/)).toHaveLength(6);
    const viewAll = within(section).getByRole("link", { name: "View all Productivity" });
    expect(viewAll).toHaveTextContent("58");
    expect(viewAll).toHaveAttribute("href", "/marketplace/categories/productivity");
    expect(mocks.readSections.mock.calls.at(-1)?.[0].get("per_section")).toBe("6");
    expect(mocks.readCatalog).not.toHaveBeenCalled();
    expect(router.state.location.pathname).toBe("/marketplace");
  });
  it("Should keep a degraded source's cached rows in their category and name its failed refresh", async () => {
    catalog = {
      ...catalog,
      stale: true,
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
    const stale = await screen.findByTestId("marketplace-stale");
    expect(stale).toHaveTextContent("Could not refresh team. Showing cached entries from");
    expect(stale).not.toHaveTextContent("compozy-catalog");
    const section = screen.getByRole("region", { name: entry.category_label });
    expect(within(section).getByTestId("marketplace-card-cached-team")).toBeVisible();
  });
  // Invariant: an empty catalog offers working source acquisition, independent of search recovery.
  // Owner: Marketplace page integration; canonical suite: marketplace-components.test.tsx.
  it("Should distinguish an empty catalog from a query with no matches", async () => {
    catalog = { ...catalog, items: [], total: 0 };
    const view = setup(undefined, "/marketplace?q=missing");
    expect(await screen.findByText(`No results for "missing"`)).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Clear search" }));
    await waitFor(() => expect(view.router.state.location.searchStr).toBe(""));
    expect(await screen.findByText("No extensions yet")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "Add plugin marketplace…" }));
    expect(await screen.findByTestId("add-marketplace-dialog")).toBeVisible();
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
      error_class: "network",
      sources: [
        { ...catalog.sources[0]!, state: "degraded", last_read_at: "2026-09-12T12:00:00Z" },
      ],
    };
    setup();
    expect(await screen.findByTestId("marketplace-stale")).toHaveTextContent(
      "Could not refresh compozy-catalog. Showing cached entries from"
    );
    expect(screen.getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
  });
  it("Should name only the failed source and clear its warning after Retry recovers", async () => {
    catalog = {
      ...catalog,
      stale: true,
      sources: [
        { ...catalog.sources[0]!, state: "ok" },
        { ...catalog.sources[0]!, name: "claude-plugins-official", state: "degraded", count: 0 },
      ],
    };
    server.use(
      http.post("*/api/marketplace/refresh", () => {
        catalog = {
          ...catalog,
          stale: false,
          sources: catalog.sources.map(source => ({ ...source, state: "ok" })),
        };
        return HttpResponse.json({ sources: [] });
      })
    );
    setup();
    const warning = await screen.findByTestId("marketplace-stale");
    expect(warning).toHaveTextContent("Could not refresh claude-plugins-official.");
    expect(warning).not.toHaveTextContent("compozy-catalog");
    expect(warning).not.toHaveTextContent("cached entries");
    await userEvent.click(within(warning).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByTestId("marketplace-stale")).toBeNull());
    expect(screen.getByTestId(`marketplace-card-${entry.entry_id}`)).toBeVisible();
  });
  // Invariant: repeated failed retries preserve the cached catalog and emit one failure toast per five seconds.
  it("Should keep the stale catalog and throttle failed Retry notifications", async () => {
    catalog = {
      ...catalog,
      stale: true,
      error_class: "network",
      sources: catalog.sources.map(source => ({ ...source, state: "degraded" })),
    };
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
  // Invariant: a failed daemon read is a load failure with Retry, never blamed on the sources.
  it("Should show a load failure, not unreachable, when the daemon read fails", async () => {
    let reads = 0;
    server.use(
      http.get("*/api/marketplace/sections", () => {
        reads++;
        return HttpResponse.json({ error: "Internal Server Error" }, { status: 500 });
      })
    );
    setup();
    // A 5xx is retried twice with backoff before the failure shows.
    expect(
      await screen.findByText("Couldn't load the marketplace", {}, { timeout: 6_000 })
    ).toBeVisible();
    expect(screen.queryByText("The marketplace is unreachable")).not.toBeInTheDocument();
    expect(screen.queryByText("No extensions yet")).not.toBeInTheDocument();
    const before = reads;
    await userEvent.click(screen.getByTestId("marketplace-retry"));
    await waitFor(() => expect(reads).toBeGreaterThan(before));
  });
  // A failed read after an empty refreshing answer shows its failure, not an endless skeleton.
  it("Should show a failed Browse read instead of the refreshing skeleton", async () => {
    let reads = 0;
    server.use(
      http.get("*/api/marketplace/sections", () => {
        reads++;
        return reads === 1
          ? HttpResponse.json({
              ...marketplaceSectionsFromCatalog(catalog),
              sections: [],
              refreshing: true,
            })
          : HttpResponse.json({ error: "bad request" }, { status: 400 });
      })
    );
    setup();
    expect(
      await screen.findByText("Couldn't load the marketplace", {}, { timeout: 3_000 })
    ).toBeVisible();
  });
  it("Should expose a daemon diagnostic on a successful empty envelope as unreachable", async () => {
    catalog = {
      ...catalog,
      total: 0,
      items: [],
      stale: true,
      error_class: "network",
      error: "feed unavailable",
      // No enabled source answered or holds cached rows: the catalog is unreachable as a whole.
      sources: catalog.sources.map(source => ({ ...source, state: "degraded", count: 0 })),
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
          entry={{
            ...entry,
            entry_id: "occupied",
            name_conflict: { source: "team", source_ref: "github:team/plugins", entry_id: "other" },
          }}
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
    const install = await screen.findByRole("button", { name: `Add ${entry.name}` });
    expect(install).toHaveTextContent("Add");
    expect(install.closest("a")).toBeNull();
    await userEvent.click(install);
    expect(onInstall).toHaveBeenCalledWith(entry);
    expect(screen.getByTestId("marketplace-installed-already")).toHaveTextContent("Installed");
    expect(screen.getByTestId("marketplace-blocked-blocked")).toHaveAttribute(
      "title",
      "Catalog policy"
    );
    expect(screen.getByTestId("marketplace-name-conflict-occupied")).toHaveTextContent(
      "Name in use"
    );
    expect(screen.getByTestId("marketplace-name-conflict-occupied")).toHaveAttribute(
      "title",
      expect.stringContaining("team/other")
    );
    expect(screen.queryByTestId("marketplace-action-occupied")).not.toBeInTheDocument();
    expect(screen.getByTestId("marketplace-action-busy")).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: `Update ${entry.name}` }));
    expect(onUpdate).toHaveBeenCalledTimes(1);
  });
  it("Should keep the Installed fallback and installed actions beside catalog status", async () => {
    const onTurnOn = vi.fn();
    const setupOf = (state: MarketplaceSetupState) => ({
      state,
      onTurnOn,
      onRetry: vi.fn(),
    });
    const installed = { ...entry, installed: true };
    setup(
      <>
        {/* Installed in another scope: no inventory match, so the honest Installed word. */}
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "elsewhere" }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={setupOf("installed")}
        />
        {/* Set up without its control falls back to Installed rather than an empty trail. */}
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "no-control" }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={setupOf("setup")}
        />
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "off-update", update_available: true }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={setupOf("turn-on")}
        />
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "off-blocked", installable: false }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={setupOf("turn-on")}
        />
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "broken" }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={{ ...setupOf("server-failed"), serverFailure: "connection refused" }}
        />
        <MarketplaceCatalogTrail
          entry={{ ...installed, entry_id: "ready-update", update_available: true }}
          onInstall={vi.fn()}
          onUpdate={vi.fn()}
          setup={setupOf("ready")}
        />
      </>
    );
    expect(await screen.findByTestId("marketplace-installed-elsewhere")).toHaveTextContent(
      "Installed"
    );
    expect(screen.getByTestId("marketplace-installed-no-control")).toHaveTextContent("Installed");
    expect(screen.getByTestId("marketplace-action-off-update")).toHaveTextContent("Update");
    await userEvent.click(screen.getByTestId("marketplace-turn-on-off-update"));
    expect(screen.getByTestId("marketplace-blocked-off-blocked")).toBeVisible();
    await userEvent.click(screen.getByTestId("marketplace-turn-on-off-blocked"));
    expect(onTurnOn).toHaveBeenCalledTimes(2);
    // A recorded server failure reads as one, with the daemon's cause, never as Ready.
    expect(screen.getByTestId("marketplace-server-failed-broken")).toHaveTextContent("Unavailable");
    expect(screen.getByTestId("marketplace-server-failed-broken")).toHaveAttribute(
      "title",
      "connection refused"
    );
    // Ready needs nothing from the user, so the Update stays the row's one action.
    expect(screen.getByTestId("marketplace-action-ready-update")).toHaveTextContent("Update");
    expect(screen.queryByTestId("marketplace-ready-ready-update")).toBeNull();
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
        entry={{ entry_id: "slack", name: "Slack", icon: "data:image/png;base64,AAAA" }}
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
  // Invariant: a monochrome brand's feed image has a fixed ink that vanishes on one theme, so its
  // theme-following mark (currentColor) wins over the image in both themes.
  it.each(["github", "linear"])(
    "Should draw the theme-following %s mark instead of its fixed-ink feed image",
    entryId => {
      const view = render(
        <MarketplaceEntryLogo
          entry={{ entry_id: entryId, name: entryId, icon: "data:image/svg+xml;base64,AAAA" }}
        />
      );
      expect(view.container.querySelector("img")).toBeNull();
      expect(view.container.querySelector('[data-rung="brand"]')).not.toBeNull();
    }
  );
  it("Should retry a new icon URL after the previous image failed", () => {
    const item = { entry_id: "slack", name: "Slack", icon: "data:image/png;base64,AAAA" };
    const view = render(<MarketplaceEntryLogo entry={item} />);
    fireEvent.error(view.container.querySelector("img")!);
    expect(view.container.querySelector("img")).toBeNull();
    view.rerender(<MarketplaceEntryLogo entry={{ ...item, icon: "data:image/png;base64,BBBB" }} />);
    expect(view.container.querySelector("img")).toHaveAttribute(
      "src",
      "data:image/png;base64,BBBB"
    );
  });
  it.each([
    "https://example.test/logo.svg",
    "http://127.0.0.1/private.png",
    "//example.test/logo.svg",
    "https://raw.githubusercontent.com/compozy/compozy/main/catalog/icons/missing.svg",
    "https://raw.githubusercontent.com/compozy/compozy/main/catalog/icons/github.svg?redirect=elsewhere",
  ])("Should fall back without requesting an unsupported image %s", icon => {
    const view = render(
      <MarketplaceEntryLogo entry={{ entry_id: "github", name: "GitHub", icon }} />
    );
    expect(view.container.querySelector("img")).toBeNull();
    expect(view.container.querySelector('[data-rung="brand"]')).not.toBeNull();
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
  it.each([undefined, "ws_selected"])(
    "Should block batch actions and reconcile partial outcomes across scopes %s",
    async workspaceId => {
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
        workspace_id: listing.entry_id === "two" ? workspaceId : undefined,
        marketplace: listing,
        update_available: true,
      }));
      const finishers: (() => void)[] = [];
      const bodies: unknown[] = [];
      server.use(
        http.post("*/api/extensions/update", async ({ request }) => {
          const body = (await request.json()) as { names: string[] };
          bodies.push(body);
          await new Promise<void>(resolve => {
            finishers.push(resolve);
          });
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
            ].filter(update => body.names.includes(update.name)),
          });
        })
      );
      setup(undefined, "/marketplace/installed");
      const batch = await screen.findByRole("button", { name: "Update all" });
      await userEvent.click(batch);
      await waitFor(() =>
        expect(bodies).toEqual(
          workspaceId
            ? [
                { profile: "default", scope: "global", names: ["one"] },
                {
                  profile: "default",
                  scope: "workspace",
                  workspace_id: workspaceId,
                  names: ["two"],
                },
              ]
            : [{ profile: "default", scope: "global", names: ["one", "two"] }]
        )
      );
      try {
        expect(screen.getByTestId("marketplace-updates-line")).toHaveTextContent("0 of 2 done");
        expect(batch).toBeDisabled();
        expect(screen.getByTestId("marketplace-installed-update-one")).toBeDisabled();
        expect(screen.getByTestId("marketplace-installed-update-two")).toBeDisabled();
        expect(screen.getByTestId("marketplace-installed-switch-two")).toHaveAttribute(
          "aria-disabled",
          "true"
        );
        if (workspaceId) {
          await act(async () => finishers[0]!());
          await waitFor(() =>
            expect(screen.getByTestId("marketplace-updates-line")).toHaveTextContent("1 of 2 done")
          );
          await waitFor(() =>
            expect(screen.getByTestId("marketplace-installed-switch-one")).not.toHaveAttribute(
              "aria-disabled",
              "true"
            )
          );
          expect(screen.getByTestId("marketplace-installed-update-two")).toBeDisabled();
        }
      } finally {
        await act(async () => {
          finishers.forEach(finish => finish());
        });
      }
      await waitFor(() =>
        expect(screen.queryByTestId("marketplace-installed-update-one")).not.toBeInTheDocument()
      );
      await waitFor(() =>
        expect(screen.getByTestId("marketplace-installed-update-two")).toBeEnabled()
      );
      expect(mocks.toast).toHaveBeenCalledWith("two: artifact unavailable");
      expect(mocks.toast).toHaveBeenCalledWith("1 of 2 updated");
    }
  );

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

  // Invariant: navigation retains the selected installation's profile and workspace through the detail request.
  it.each([undefined, "ws-owned"])(
    "Should retain Installed row ownership when opening details in workspace %s",
    async workspaceId => {
      extensions = [
        {
          ...extensionFixtures[0]!,
          name: "owned-kit",
          profile: "engineering",
          workspace_id: workspaceId,
          marketplace: null,
          origin: null,
          update_available: false,
        },
      ];
      const requests: URL[] = [];
      server.use(
        http.get("*/api/marketplace/entries/:entryId", ({ request }) => {
          requests.push(new URL(request.url));
          return HttpResponse.json(
            { error: { code: "marketplace_entry_not_found", message: "Entry not found" } },
            { status: 404 }
          );
        })
      );
      setup(undefined, "/marketplace/installed");
      const row = await screen.findByTestId("marketplace-installed-card-owned-kit");
      await userEvent.click(within(row).getByRole("link"));
      await waitFor(() => expect(requests.length).toBeGreaterThan(0));
      const request = requests.at(-1)!;
      expect(request.searchParams.get("installed_name")).toBe("owned-kit");
      expect(request.searchParams.get("profile")).toBe("engineering");
      expect(request.searchParams.get("workspace_id")).toBe(workspaceId ?? null);
    }
  );

  // Invariant: Installed is a flat name-ordered inventory, including sideloads and scope metadata.
  it("Should show sideload descriptions and exact installation or server scope", async () => {
    const emptyContents = { skills: 0, mcp_servers: 0, agents: 0, loops: 0, hooks: 0 };
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
        description: "Local review utilities",
        marketplace: null,
        origin: null,
        contents: { ...emptyContents, mcp_servers: 1, skills: 2 },
        workspace_id: "ws-a",
        update_available: false,
      },
      {
        ...extensionFixtures[0]!,
        name: "Beta",
        marketplace: null,
        origin: null,
        contents: emptyContents,
        workspace_id: undefined,
        installation_profile: "work",
        update_available: false,
      },
      {
        ...extensionFixtures[0]!,
        name: "Gamma",
        marketplace: null,
        origin: null,
        contents: emptyContents,
        workspace_id: undefined,
        profile: "engineering",
        placements: [
          { kind: "mcp_server", resource: "review", profile: "engineering", dormant: false },
        ],
        update_available: false,
      },
    ];
    const view = setup(undefined, "/marketplace/installed");
    await screen.findByTestId("marketplace-installed-card-Alpha");
    const rows = screen.getAllByTestId(/^marketplace-installed-card-/);
    expect(rows.map(row => row.dataset.testid)).toEqual([
      "marketplace-installed-card-Alpha",
      "marketplace-installed-card-Beta",
      "marketplace-installed-card-Gamma",
      "marketplace-installed-card-Zulu",
    ]);
    expect(rows[0]).toHaveTextContent("1 MCP server · 2 skills");
    expect(rows[0]).toHaveTextContent("This project");
    expect(rows[0]).toHaveTextContent("Local review utilities");
    expect(rows[1]).toHaveTextContent("profile · work");
    expect(rows[2]).toHaveTextContent("profile · engineering");
    expect(rows[3]).not.toHaveTextContent("MCP server");
    expect(rows[3]).not.toHaveTextContent("This project");
    expect(rows[3]).not.toHaveTextContent("profile ·");
    await act(async () => {
      await view.router.navigate({ to: "/marketplace/installed", search: { q: "utilities" } });
    });
    expect(await screen.findByTestId("marketplace-installed-card-Alpha")).toBeVisible();
    expect(screen.queryByTestId("marketplace-installed-card-Zulu")).not.toBeInTheDocument();
  });
  it("Should distinguish Installed empties from filtered inventory", async () => {
    const view = setup(undefined, "/marketplace/installed");
    expect(await screen.findByText("No extensions installed yet")).toBeVisible();
    extensions = [{ ...extensionFixtures[0]!, marketplace: null, origin: null }];
    await act(async () => {
      await view.router.navigate({ to: "/marketplace/installed", search: { q: "missing" } });
      await view.client.invalidateQueries();
    });
    expect(await screen.findByText(`No results for "missing"`)).toBeVisible();
  });
});

describe("Extension source installation", () => {
  it("Should pin the previewed local artifact through explicit source consent", async () => {
    const user = userEvent.setup();
    mocks.previewExtensionInstall.mockResolvedValueOnce({
      digest_sha256: "a".repeat(64),
      inputs: [
        {
          id: "token",
          prompt: "API key",
          type: "secret",
          required: true,
          binding: { type: "env", name: "TOKEN" },
        },
      ],
      declared_profiles: [{ create: true, credentials: [], name: "operations" }],
      name: "gen-a1b2c3",
      gateway_requirement_digest: "sha256:local-network",
      placements: [],
    });
    renderInstaller();

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-ref"));
    await user.paste("relative/dist");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "absolute path"
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.clear(screen.getByTestId("extension-install-ref"));
    await user.click(screen.getByTestId("extension-install-ref"));
    await user.paste("/srv/hello/dist/gen-a1b2c3");
    // Rarely needed options, including the unverified opt-in, start folded.
    expect(screen.queryByTestId("extension-install-allow-unverified")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "More options" }));
    await user.click(screen.getByTestId("extension-install-allow-unverified"));
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    expect(mocks.installExtension).not.toHaveBeenCalled();
    expect(screen.getByTestId("extension-install-submit")).toBeDisabled();
    await user.click(screen.getByLabelText("API key"));
    await user.paste("literal-secret");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(mocks.previewExtensionInstall).toHaveBeenCalledOnce();

    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("extension-trust-confirm"));

    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        enable: false,
        allow_unverified: true,
        expected_digest: "a".repeat(64),
        confirm_gateway_digest: "sha256:local-network",
        inputs: { token: { value: "literal-secret" } },
        ref: "/srv/hello/dist/gen-a1b2c3",
        source: "local_path",
      })
    );
  });

  it("Should clear secret drafts when closing or changing the acquisition", async () => {
    mocks.previewExtensionInstall.mockResolvedValue({
      name: "kit",
      digest_sha256: "a".repeat(64),
      inputs: [
        {
          id: "token",
          prompt: "API key",
          type: "secret",
          required: true,
          binding: { type: "env", name: "TOKEN" },
        },
      ],
      declared_profiles: [],
      placements: [],
    });
    renderInstaller();
    const user = userEvent.setup();
    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-ref"));
    await user.paste("/srv/kit");
    await user.click(screen.getByTestId("extension-install-submit"));
    await user.click(await screen.findByLabelText("API key"));
    await user.paste("discarded-secret");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByTestId("marketplace-extension-install"));
    const ref = screen.getByTestId("extension-install-ref");
    await user.clear(ref);
    await user.click(ref);
    await user.paste("/srv/kit");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByLabelText("API key")).toHaveValue("");
    await user.click(screen.getByLabelText("API key"));
    await user.paste("also-discarded");
    await user.clear(ref);
    await user.click(ref);
    await user.paste("/srv/other-kit");
    expect(screen.queryByLabelText("API key")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByLabelText("API key")).toHaveValue("");
    expect(mocks.installExtension).not.toHaveBeenCalled();
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
    await user.click(ref);
    await user.paste("ssh://git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "public HTTPS repository URL"
    );

    await user.clear(ref);
    await user.click(ref);
    await user.paste("https://@git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Remove credentials"
    );

    await user.clear(ref);
    await user.click(ref);
    await user.paste("https://git.example.com/acme/hello.git?");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Put the Git ref in the Version field"
    );

    await user.clear(ref);
    await user.click(ref);
    await user.paste("https://git.example.com./acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "valid host and repository path"
    );

    await user.clear(ref);
    await user.click(ref);
    await user.paste("https://git.example.com/acme/hello.git@v1.2.3");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-summary")).toBeVisible();
    await user.click(screen.getByTestId("extension-install-submit"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        enable: false,
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
      enable: false,
      ref: "/srv/hello/dist/gen-a1b2c3",
      source: "local_path",
    });

    await user.click(screen.getByTestId("extension-trust-confirm"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenLastCalledWith({
        enable: false,
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

// Invariant: input form values retain daemon wire types, byte limits and optional omission.
// Owner: Marketplace acquisition form; canonical suite: marketplace-components.test.tsx.
describe("Extension input form", () => {
  const definitions: ExtensionInputDefinitions = [
    {
      id: "token",
      prompt: "API key",
      type: "secret",
      required: true,
      binding: { type: "env", name: "TOKEN" },
    },
    {
      id: "region",
      prompt: "Region",
      type: "identifier",
      required: false,
      default: "eu",
      binding: { type: "url_query", name: "region" },
    },
    {
      id: "enabled",
      prompt: "Enabled",
      type: "boolean",
      required: true,
      default: false,
      binding: { type: "env", name: "ENABLED" },
    },
    {
      id: "note",
      prompt: "Note",
      type: "string",
      required: false,
      binding: { type: "env", name: "NOTE" },
    },
  ];
  it("Should prefill defaults and preserve false without inventing a secret", () => {
    const draft = createExtensionInputDraft(definitions);
    expect(draft).toEqual({ region: "eu", enabled: false });
    expect(prepareExtensionInputs(definitions, draft)).toMatchObject({
      valid: false,
      errors: { token: "Required" },
    });
  });
  it("Should preserve secret bytes, normalize identifiers and omit empty optional fields", () => {
    expect(
      prepareExtensionInputs(definitions, {
        token: "  literal-secret  ",
        region: " us-west ",
        enabled: false,
        note: "",
        ignored: "unused",
      })
    ).toEqual({
      valid: true,
      errors: {},
      inputs: {
        token: { value: "  literal-secret  " },
        region: { value: "us-west" },
        enabled: { value: false },
      },
    });
    const optional = definitions.map(input => ({ ...input, required: false }));
    expect(prepareExtensionInputs(optional, {})).toEqual({ valid: true, errors: {}, inputs: {} });
  });
  it("Should enforce the UTF-8 byte limit including multibyte input", () => {
    const note = definitions.filter(input => input.id === "note");
    expect(prepareExtensionInputs(note, { note: "é".repeat(4096) }).valid).toBe(true);
    expect(prepareExtensionInputs(note, { note: "é".repeat(4096) + "a" }).errors).toEqual({
      note: "Too long (max 8 KB)",
    });
    expect(prepareExtensionInputs(note, { note: "a".repeat(8193) }).valid).toBe(false);
  });
  it.each([
    { token: "   ", enabled: false },
    { token: "secret", enabled: "false" },
    { token: "secret", enabled: false, region: "has space" },
    { token: "secret", enabled: false, note: "nul\0value" },
  ])("Should refuse invalid typed input %#", draft => {
    expect(prepareExtensionInputs(definitions, draft).valid).toBe(false);
  });
});

// Invariant UT047/049: declared facts never imply runtime readiness; authorization captures the
// extension owner and refreshes Installed after a real status observation. HTTP is the I/O boundary.
describe("Marketplace extension MCP controls", () => {
  const published = {
    name: "github",
    owner: "extension:github",
    runtime_name: "github.github",
    scope: "global",
    transport: "http",
    launch: "https://api.githubcopilot.com",
    status: "needs_authorization",
    auth: { method: "oauth", registration: "dynamic", scopes: ["repo"] },
  };

  it("Should show manifest facts before installation without observed status or management actions", async () => {
    const user = userEvent.setup();
    setup(
      <MarketplaceExtensionServerSection
        servers={[published]}
        inputs={[{ id: "region", required: true }]}
      />
    );
    const toggle = await screen.findByRole("button", { name: /^Server/ });
    if (toggle.getAttribute("aria-expanded") === "false") await user.click(toggle);
    const section = screen.getByTestId("marketplace-extension-server-github");
    for (const label of ["Sign-in", "Inputs"])
      expect(within(section).getByText(label)).toBeVisible();
    expect(within(section).queryByText("Launch")).not.toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "Technical details" }));
    for (const label of ["Launch", "Scope", "Owner"])
      expect(within(section).getByText(label)).toBeVisible();
    expect(within(section).getByText("extension:github")).toBeVisible();
    expect(within(section).queryByText("Runtime name")).not.toBeInTheDocument();
    expect(within(section).queryByText("Status")).not.toBeInTheDocument();
    expect(
      within(section).queryByRole("button", { name: /Authorize|Edit configuration/ })
    ).not.toBeInTheDocument();
  });

  it("Should omit the Server section when the extension declares no MCP server", async () => {
    const { router } = setup(<MarketplaceExtensionServerSection servers={[]} inputs={[]} />);
    await waitFor(() => expect(router.state.status).toBe("idle"));
    expect(screen.queryByRole("button", { name: /^Server/ })).not.toBeInTheDocument();
  });

  it("Should prioritize missing inputs over authorization on the installed Server card", async () => {
    server.use(
      http.get("*/api/settings/mcp-servers/:name", () =>
        HttpResponse.json({ server: mcpExtensionServerFixtures[1]! })
      )
    );
    setup(
      <MarketplaceExtensionServerSection
        servers={[published]}
        inputs={[{ id: "token", required: true }]}
        missingInputs={["token"]}
        installed
      />
    );
    await screen.findByRole("button", { name: "Edit configuration" });
    expect(screen.getByTestId("marketplace-extension-server-status-github")).toHaveTextContent(
      "Needs configuration"
    );
    expect(screen.queryByRole("button", { name: "Authorize" })).not.toBeInTheDocument();
  });

  it("Should authorize the extension owner and refresh Installed after callback confirmation", async () => {
    const user = userEvent.setup();
    let authorized = false;
    const requests: URL[] = [];
    const definition = mcpExtensionServerFixtures[1]!;
    extensions = [
      {
        ...extensionFixtures[0]!,
        name: "github",
        marketplace: null,
        origin: null,
        missing_inputs: [],
        mcp_servers: [published],
      },
    ];
    server.use(
      http.get("*/api/settings/mcp-servers/:name", ({ request }) => {
        const url = new URL(request.url);
        expect(url.pathname).toBe("/api/settings/mcp-servers/github");
        expect(url.searchParams.get("owner")).toBe("extension:github");
        expect(url.searchParams.get("scope")).toBe("user");
        return HttpResponse.json({
          server: authorized
            ? {
                ...definition,
                auth_status: {
                  ...definition.auth_status!,
                  status: "authenticated",
                  token_present: true,
                },
                runtime_status: {
                  ...definition.runtime_status!,
                  state: "ready",
                  initialized: true,
                  probe: "succeeded",
                },
              }
            : definition,
        });
      }),
      http.post("*/api/settings/mcp-servers/:name/auth/begin", ({ request }) => {
        requests.push(new URL(request.url));
        return HttpResponse.json(mcpAuthBeginFixture);
      })
    );
    const { client } = setup(undefined, "/marketplace/installed");
    const authorize = await screen.findByRole("button", { name: "Authorize github for github" });
    await waitFor(() => expect(authorize).toBeEnabled());
    await user.click(authorize);
    await screen.findByRole("dialog", { name: "Authorize github" });
    await waitFor(() => expect(requests).toHaveLength(1));
    expect(requests[0]!.searchParams.get("owner")).toBe("extension:github");
    expect(requests[0]!.searchParams.get("scope")).toBe("user");
    authorized = true;
    extensions = extensions.map(extension => ({
      ...extension,
      mcp_servers: [{ ...published, status: "running" }],
    }));
    await act(async () => {
      await client.invalidateQueries({ queryKey: settingsKeys.mcpRoot() });
    });
    await waitFor(() =>
      expect(screen.getByTestId("marketplace-installed-server-status-github")).toHaveTextContent(
        "Running"
      )
    );
    expect(
      screen.queryByRole("button", { name: "Authorize github for github" })
    ).not.toBeInTheDocument();
  });
});

// Invariant: Browse groups by daemon category, a category page pages the daemon list, and each row
// shows the one action the installed inventory supports (US-001–US-007, US-013).
// Owner: Marketplace page integration; canonical suite: marketplace-components.test.tsx.
describe("Category browse and row actions", () => {
  const productivity = Array.from({ length: 9 }, (_, index) => ({
    ...entry,
    entry_id: `prod-${index}`,
    name: `prod-${index}`,
    description: index === 8 ? "Notes that sync" : "Tasks and docs",
    category: "productivity",
    category_label: "Productivity",
    featured: false,
  }));
  /** `GET /api/marketplace?category=` the way the daemon pages it, cursor = next offset. */
  function useCategoryServer(
    rows = productivity,
    pageSize = 5,
    {
      staleCursors = [],
      refreshingReads = 0,
      beforeAnswer,
    }: {
      /** Cursors the daemon rejects as stale (409, restart), once per listed occurrence. */
      staleCursors?: string[];
      /** How many first reads answer an empty, still-refreshing page. */
      refreshingReads?: number;
      /** Holds a read (by cursor, "" for page one) until the returned promise settles. */
      beforeAnswer?: (cursor: string) => Promise<void> | undefined;
    } = {}
  ) {
    let reads = 0;
    // Stale answers already given per cursor; callers may list more cursors after setup.
    const rejectedCursors = new Map<string, number>();
    server.use(
      http.get("*/api/marketplace", async ({ request }) => {
        const params = new URL(request.url).searchParams;
        mocks.readCatalog(params);
        reads += 1;
        const cursor = params.get("cursor") ?? "";
        await beforeAnswer?.(cursor);
        const rejected = rejectedCursors.get(cursor) ?? 0;
        if (staleCursors.filter(stale => stale === cursor).length > rejected) {
          rejectedCursors.set(cursor, rejected + 1);
          return HttpResponse.json(
            { code: "marketplace_cursor_stale", error: "cursor is stale", restart: true },
            { status: 409 }
          );
        }
        if (reads <= refreshingReads) {
          return HttpResponse.json({
            ...catalog,
            categories: [],
            items: [],
            refreshing: true,
            total: 0,
          });
        }
        const category = params.get("category") ?? "";
        if (category === "apps") {
          return HttpResponse.json(
            { code: "marketplace_category_unknown", error: 'unknown category "apps"' },
            { status: 400 }
          );
        }
        const q = params.get("q")?.toLowerCase() ?? "";
        const matches = rows.filter(row =>
          `${row.name} ${row.description}`.toLowerCase().includes(q)
        );
        const scoped = matches.filter(row => !category || row.category === category);
        const offset = Number(params.get("cursor") ?? "0");
        const items = scoped.slice(offset, offset + pageSize);
        const next = offset + pageSize < scoped.length ? String(offset + pageSize) : undefined;
        return HttpResponse.json({
          ...catalog,
          categories: marketplaceCategoryCounts(matches),
          items,
          next_cursor: next,
          total: scoped.length,
        });
      })
    );
  }

  // UT-022 [US-002.AC-1, AC-2]: daemon label as title, the next page appended without duplicates.
  it("Should title the category from the response and load the next page at the end [UT-022]", async () => {
    useCategoryServer();
    setup(undefined, "/marketplace/categories/productivity");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(5);
    expect(within(screen.getByRole("banner")).getByText("Productivity")).toBeVisible();
    expect(screen.getByRole("status", { name: "Catalog count" })).toHaveTextContent("9");
    expect(mocks.readCatalog.mock.calls.at(-1)?.[0].get("category")).toBe("productivity");
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
    expect(mocks.readCatalog.mock.calls.at(-1)?.[0].get("cursor")).toBe("5");
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  // UT-022 [US-002.AC-2]: the end of the list coming into view asks for the next page by itself.
  it("Should load the next page when the end of the category comes into view [UT-022]", async () => {
    const observers: IntersectionObserverCallback[] = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(callback: IntersectionObserverCallback) {
          observers.push(callback);
        }
        observe() {}
        disconnect() {}
      }
    );
    try {
      useCategoryServer();
      setup(undefined, "/marketplace/categories/productivity");
      const grid = await screen.findByTestId("marketplace-category-grid-productivity");
      await waitFor(() => expect(observers).toHaveLength(1));
      act(() =>
        observers[0]!(
          [{ isIntersecting: true } as IntersectionObserverEntry],
          {} as IntersectionObserver
        )
      );
      await waitFor(() =>
        expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9)
      );
      // One observer for the sentinel's lifetime, not one per render.
      expect(observers).toHaveLength(1);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  // UT-022 [US-002.EC-2]: a stale cursor restarts from page one silently, keeping the rows shown.
  it("Should restart a stale category cursor from the first page without an error [UT-022]", async () => {
    useCategoryServer(productivity, 5, { staleCursors: ["5"] });
    setup(undefined, "/marketplace/categories/productivity");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    const firstReads = mocks.readCatalog.mock.calls.length;
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() =>
      expect(
        mocks.readCatalog.mock.calls.slice(firstReads).map(([params]) => params.get("cursor"))
      ).toEqual(["5", null])
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "Load more" })).toBeEnabled());
    expect(grid).toBeInTheDocument();
    expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(5);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByTestId("marketplace-category-error")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
  });

  // UT-022 [US-002.EC-2]: Load more waits for a running restart instead of cancelling it.
  it("Should hold Load more until the stale-cursor restart finishes [UT-022]", async () => {
    const observers: IntersectionObserverCallback[] = [];
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(callback: IntersectionObserverCallback) {
          observers.push(callback);
        }
        observe() {}
        disconnect() {}
      }
    );
    const reachEnd = () =>
      act(() =>
        observers.at(-1)!(
          [{ isIntersecting: true } as IntersectionObserverEntry],
          {} as IntersectionObserver
        )
      );
    try {
      let holdRestart = false;
      let release = () => {};
      const restartHeld = new Promise<void>(resolve => {
        release = resolve;
      });
      // A test-local copy, so a row can change while the restart is held.
      const rows = [...productivity];
      useCategoryServer(rows, 5, {
        staleCursors: ["5"],
        beforeAnswer: cursor => (holdRestart && cursor === "" ? restartHeld : undefined),
      });
      setup(undefined, "/marketplace/categories/productivity");
      const grid = await screen.findByTestId("marketplace-category-grid-productivity");
      const firstReads = mocks.readCatalog.mock.calls.length;
      const cursors = () =>
        mocks.readCatalog.mock.calls.slice(firstReads).map(([params]) => params.get("cursor"));
      holdRestart = true;
      await userEvent.click(screen.getByRole("button", { name: "Load more" }));
      await waitFor(() => expect(cursors()).toEqual(["5", null]));
      // The restart is in flight: the control shows it and the end of the list asks nothing.
      await waitFor(() => expect(screen.getByRole("button", { name: "Loading…" })).toBeDisabled());
      reachEnd();
      expect(cursors()).toEqual(["5", null]);
      // The daemon changes a first-page row while the restart is held; the restart shows it.
      rows[0] = { ...rows[0]!, description: "Tasks refreshed by the restart" };
      release();
      await waitFor(() => expect(screen.getByRole("button", { name: "Load more" })).toBeEnabled());
      expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(5);
      expect(
        within(screen.getByTestId("marketplace-card-prod-0")).getByText(
          "Tasks refreshed by the restart"
        )
      ).toBeInTheDocument();
      expect(screen.queryByRole("alert")).toBeNull();
      reachEnd();
      await waitFor(() =>
        expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9)
      );
      expect(cursors()).toEqual(["5", null, "5"]);
    } finally {
      vi.unstubAllGlobals();
    }
  });

  // UT-022 [US-002.AC-2]: Load more waits for a running refetch instead of cancelling it.
  it("Should ask for the next page only after a running category refetch settles [UT-022]", async () => {
    let holdRefetch = false;
    let release = () => {};
    const refetchHeld = new Promise<void>(resolve => {
      release = resolve;
    });
    const rows = [...productivity];
    useCategoryServer(rows, 5, {
      beforeAnswer: cursor => (holdRefetch && cursor === "" ? refetchHeld : undefined),
    });
    const { client } = setup(undefined, "/marketplace/categories/productivity");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    const before = mocks.readCatalog.mock.calls.length;
    const cursors = () =>
      mocks.readCatalog.mock.calls.slice(before).map(([params]) => params.get("cursor"));
    holdRefetch = true;
    void client.refetchQueries({ queryKey: marketplaceKeys.all });
    await waitFor(() => expect(cursors()).toEqual([null]));
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(cursors()).toEqual([null]);
    // The refetch answers with a changed first page; the next page appends to it.
    rows[0] = { ...rows[0]!, description: "Tasks refreshed by the refetch" };
    release();
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
    expect(cursors()).toEqual([null, "5"]);
    expect(
      within(screen.getByTestId("marketplace-card-prod-0")).getByText(
        "Tasks refreshed by the refetch"
      )
    ).toBeInTheDocument();
  });

  // UT-022 [US-002.AC-2]: the button and the sentinel firing together ask for one next page.
  it("Should send one next-page request when the button and the sentinel fire together [UT-022]", async () => {
    const observers: IntersectionObserverCallback[] = [];
    let cancels: MockInstance<Query["cancel"]> | undefined;
    vi.stubGlobal(
      "IntersectionObserver",
      class {
        constructor(callback: IntersectionObserverCallback) {
          observers.push(callback);
        }
        observe() {}
        disconnect() {}
      }
    );
    try {
      useCategoryServer();
      setup(undefined, "/marketplace/categories/productivity");
      const grid = await screen.findByTestId("marketplace-category-grid-productivity");
      await waitFor(() => expect(observers).toHaveLength(1));
      const before = mocks.readCatalog.mock.calls.length;
      // A second request would cancel the first one's read before React re-renders.
      cancels = vi.spyOn(Query.prototype, "cancel");
      act(() => {
        screen.getByRole("button", { name: "Load more" }).click();
        observers[0]!(
          [{ isIntersecting: true } as IntersectionObserverEntry],
          {} as IntersectionObserver
        );
      });
      await waitFor(() =>
        expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9)
      );
      expect(cancels).not.toHaveBeenCalled();
      expect(
        mocks.readCatalog.mock.calls.slice(before).map(([params]) => params.get("cursor"))
      ).toEqual(["5"]);
    } finally {
      cancels?.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  // UT-022 [US-002.EC-2]: a background refetch that meets a stale cursor restarts silently too.
  it("Should restart a stale cursor met by a background category refetch", async () => {
    const staleCursors: string[] = [];
    useCategoryServer(productivity, 5, { staleCursors });
    const { client } = setup(undefined, "/marketplace/categories/productivity");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
    const before = mocks.readCatalog.mock.calls.length;
    staleCursors.push("5");
    await act(() => client.refetchQueries({ queryKey: marketplaceKeys.all }));
    // The refetch meets the stale cursor, then the restart reads both pages again.
    await waitFor(() =>
      expect(
        mocks.readCatalog.mock.calls.slice(before).map(([params]) => params.get("cursor"))
      ).toEqual([null, "5", null, "5"])
    );
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // A restart that meets a stale cursor again stops and offers Retry instead of hiding the error.
  it("Should offer Retry when the stale-cursor restart cannot recover", async () => {
    const staleCursors: string[] = [];
    useCategoryServer(productivity, 5, { staleCursors });
    const { client } = setup(undefined, "/marketplace/categories/productivity");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9));
    const before = mocks.readCatalog.mock.calls.length;
    staleCursors.push("5", "5");
    await act(() => client.refetchQueries({ queryKey: marketplaceKeys.all }));
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("The next entries could not be loaded.");
    expect(mocks.readCatalog.mock.calls.length - before).toBe(4);
    await userEvent.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(9);
  });

  // A failed read after an empty refreshing answer shows its failure, not an endless skeleton.
  it("Should show a failed category read instead of the refreshing skeleton", async () => {
    let reads = 0;
    server.use(
      http.get("*/api/marketplace", () => {
        reads++;
        return reads === 1
          ? HttpResponse.json({ ...catalog, categories: [], items: [], refreshing: true, total: 0 })
          : HttpResponse.json({ error: "bad request" }, { status: 400 });
      })
    );
    setup(undefined, "/marketplace/categories/productivity");
    const failure = await screen.findByTestId("marketplace-category-error", {}, { timeout: 3_000 });
    expect(within(failure).getByText("Couldn't load the marketplace")).toBeVisible();
  });

  // F3 / _uiux S2: a category read while the catalog refreshes is loading, then fills in by itself.
  it("Should keep loading while the catalog refreshes and follow the refresh", async () => {
    useCategoryServer(productivity, 5, { refreshingReads: 1 });
    setup(undefined, "/marketplace/categories/productivity");
    await waitFor(() => expect(mocks.readCatalog).toHaveBeenCalled());
    expect(screen.queryByTestId("marketplace-category-empty")).toBeNull();
    const grid = await screen.findByTestId(
      "marketplace-category-grid-productivity",
      {},
      { timeout: 3_000 }
    );
    expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(5);
  });

  // UT-023 [US-002.AC-3, EC-3]: the search stays inside the category and clearing restores it.
  it("Should scope the category search and restore the full list when cleared [UT-023]", async () => {
    useCategoryServer();
    const { router } = setup(undefined, "/marketplace/categories/productivity?q=sync");
    const grid = await screen.findByTestId("marketplace-category-grid-productivity");
    expect(within(grid).getAllByTestId(/^marketplace-card-/)).toHaveLength(1);
    const read = mocks.readCatalog.mock.calls.at(-1)?.[0];
    expect(read.get("q")).toBe("sync");
    expect(read.get("category")).toBe("productivity");
    const input = screen.getByRole("searchbox", { name: "Search Productivity" });
    await userEvent.click(input);
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(router.state.location.pathname).toBe("/marketplace/categories/productivity");
    await waitFor(() =>
      expect(
        within(screen.getByTestId("marketplace-category-grid-productivity")).getAllByTestId(
          /^marketplace-card-/
        )
      ).toHaveLength(5)
    );
    await userEvent.type(screen.getByRole("searchbox", { name: "Search Productivity" }), "zzz");
    expect(await screen.findByText('No results for "zzz" in Productivity')).toBeVisible();
  });

  // Invariant: a 5xx category read is a load failure with Retry; only a daemon envelope that
  // reports its sources unreachable says the marketplace is unreachable.
  it("Should separate a failed category read from unreachable sources", async () => {
    let failing = true;
    server.use(
      http.get("*/api/marketplace", () =>
        failing
          ? HttpResponse.json({ error: "Internal Server Error" }, { status: 500 })
          : HttpResponse.json({
              ...catalog,
              items: [],
              total: 0,
              error_class: "source_unreachable",
              error: "feed unavailable",
              sources: catalog.sources.map(source => ({ ...source, state: "degraded", count: 0 })),
            })
      )
    );
    setup(undefined, "/marketplace/categories/productivity");
    // A 5xx is retried twice with backoff before the failure shows.
    const failure = await screen.findByTestId("marketplace-category-error", {}, { timeout: 6_000 });
    expect(within(failure).getByText("Couldn't load the marketplace")).toBeVisible();
    expect(screen.queryByText("The marketplace is unreachable")).not.toBeInTheDocument();
    failing = false;
    await userEvent.click(within(failure).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("The marketplace is unreachable")).toBeVisible();
    expect(screen.queryByText("Couldn't load the marketplace")).not.toBeInTheDocument();
  });

  // Invariant: one degraded source beside a healthy one is a partial failure; a category search
  // with no matches is "No results", never "The marketplace is unreachable".
  it("Should show no results, not unreachable, when only one source is degraded", async () => {
    server.use(
      http.get("*/api/marketplace", () =>
        HttpResponse.json({
          ...catalog,
          items: [],
          total: 0,
          stale: true,
          error_class: "rate_limited",
          error: "GitHub rate limit",
          sources: [
            { ...catalog.sources[0]!, state: "ok" },
            { ...catalog.sources[0]!, name: "team-plugins", state: "degraded", count: 0 },
          ],
        })
      )
    );
    setup(undefined, "/marketplace/categories/productivity?q=zzz");
    expect(await screen.findByText('No results for "zzz" in Productivity')).toBeVisible();
    expect(screen.queryByText("The marketplace is unreachable")).not.toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-category-error")).toBeNull();
  });

  // UT-024 [US-002.EC-1]: an unknown id is a not-found state with a way back to Browse.
  it("Should show the not-found state for an unknown category [UT-024]", async () => {
    useCategoryServer();
    const { router } = setup(undefined, "/marketplace/categories/apps");
    expect(await screen.findByText("This category doesn't exist")).toBeVisible();
    const back = screen.getByRole("link", { name: "Back to Marketplace" });
    expect(back).toHaveAttribute("href", "/marketplace");
    await userEvent.click(back);
    await waitFor(() => expect(router.state.location.pathname).toBe("/marketplace"));
  });

  // UT-024: a blank id never reaches the daemon, so it is not-found instead of loading forever.
  it("Should show the not-found state for a whitespace-only category", async () => {
    useCategoryServer();
    setup(undefined, "/marketplace/categories/%09");
    expect(await screen.findByText("This category doesn't exist")).toBeVisible();
    expect(screen.getByRole("link", { name: "Back to Marketplace" })).toBeVisible();
  });

  // UT-026 [US-003.AC-1]: search replaces sections with flat results that name their category.
  it("Should show flat search results with each result's category word [UT-026]", async () => {
    setup(undefined, "/marketplace?q=otel");
    const results = await screen.findByTestId("marketplace-search-results");
    const card = within(results).getByTestId(`marketplace-card-${entry.entry_id}`);
    expect(within(card).getByText(entry.category_label)).toBeVisible();
    expect(screen.queryByRole("region", { name: entry.category_label })).toBeNull();
    expect(mocks.readSections).not.toHaveBeenCalled();
  });

  const signInServer = {
    name: "github",
    owner: "extension:github",
    runtime_name: "github.github",
    scope: "global",
    transport: "http",
    launch: "https://api.githubcopilot.com",
    status: "needs_authorization",
    auth: { method: "oauth", registration: "dynamic", scopes: ["repo"] },
  };
  function installedRow(extension: Partial<ExtensionEntry>) {
    catalog = {
      ...catalog,
      items: [{ ...entry, featured: false, installed: true, installed_name: "github" }],
    };
    extensions = [
      {
        ...extensionFixtures[0]!,
        name: "github",
        enabled: true,
        marketplace: null,
        origin: null,
        missing_inputs: [],
        mcp_servers: [],
        ...extension,
      },
    ];
  }

  // UT-032 [US-006.AC-1]: one server waiting for sign-in starts that sign-in from the row.
  it("Should start the server sign-in from Set up [UT-032]", async () => {
    const begins: URL[] = [];
    let signedIn = false;
    installedRow({ mcp_servers: [signInServer] });
    server.use(
      http.get("*/api/settings/mcp-servers/:name", () =>
        HttpResponse.json({
          server: {
            ...mcpExtensionServerFixtures[1]!,
            auth_status: {
              ...mcpExtensionServerFixtures[1]!.auth_status!,
              status: signedIn ? "authenticated" : "needs_login",
              token_present: signedIn,
            },
          },
        })
      ),
      http.post("*/api/settings/mcp-servers/:name/auth/begin", ({ request }) => {
        begins.push(new URL(request.url));
        return HttpResponse.json(mcpAuthBeginFixture);
      })
    );
    const { client } = setup();
    const setUp = await screen.findByRole("button", { name: `Set up ${entry.name}` });
    // The live server read has answered, so the press starts that server's sign-in in place.
    await waitFor(() => expect(clients.at(-1)!.isFetching()).toBe(0));
    await userEvent.click(setUp);
    await screen.findByRole("dialog", { name: "Authorize github" });
    await waitFor(() => expect(begins).toHaveLength(1));
    expect(begins[0]!.searchParams.get("owner")).toBe("extension:github");
    // Inventory can become Ready before the dialog observes the completed browser callback.
    extensions = extensions.map(extension => ({
      ...extension,
      mcp_servers: [{ ...signInServer, status: "running" }],
    }));
    await act(async () => {
      await client.invalidateQueries();
    });
    expect(await screen.findByTestId(`marketplace-ready-${entry.entry_id}`)).toBeVisible();
    expect(screen.getByRole("dialog", { name: "Authorize github" })).toBeVisible();
    signedIn = true;
    await act(async () => {
      await client.invalidateQueries();
    });
    expect(await screen.findByTestId("settings-page-mcp-authorize-confirmed")).toBeVisible();
    await userEvent.click(screen.getByTestId("settings-page-mcp-authorize-done"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  // Invariant: Set up is usable from the inventory alone; a slow live server read never disables
  // it, and a press before that read answers opens the detail's setup instead of waiting.
  it("Should keep Set up usable while the live server read is pending [UT-032]", async () => {
    installedRow({ mcp_servers: [signInServer] });
    let reads = 0;
    server.use(
      http.get("*/api/settings/mcp-servers/:name", () => {
        reads += 1;
        return new Promise<never>(() => {});
      })
    );
    const { router } = setup();
    const setUp = await screen.findByRole("button", { name: `Set up ${entry.name}` });
    await waitFor(() => expect(reads).toBe(1));
    expect(setUp).toBeEnabled();
    await userEvent.click(setUp);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/marketplace/${entry.entry_id}`)
    );
    expect(router.state.location.search).toMatchObject({ setup: 1, source: entry.source });
  });

  // UT-032 [US-006.AC-2]: a missing input opens the detail asking it to focus setup.
  it("Should open the detail with setup=1 when an input is missing [UT-032]", async () => {
    installedRow({ missing_inputs: ["GITHUB_TOKEN"], mcp_servers: [signInServer] });
    const { router } = setup();
    await userEvent.click(await screen.findByRole("button", { name: `Set up ${entry.name}` }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/marketplace/${entry.entry_id}`)
    );
    expect(router.state.location.search).toMatchObject({ setup: 1, source: entry.source });
  });

  // UT-033 [US-007.AC-1, AC-2, EC-1]: one action per row; unknown inventory never reads Ready.
  it("Should read Ready, Turn on, or a skeleton from the installed inventory [UT-033]", async () => {
    installedRow({});
    let releaseInventory!: () => void;
    const inventory = new Promise<void>(resolve => (releaseInventory = resolve));
    server.use(
      http.get("*/api/extensions", async () => {
        await inventory;
        return HttpResponse.json({ extensions });
      })
    );
    setup();
    expect(
      await screen.findByTestId(`marketplace-setup-unknown-${entry.entry_id}`)
    ).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByText("Ready")).toBeNull();
    releaseInventory();
    expect(await screen.findByTestId(`marketplace-ready-${entry.entry_id}`)).toHaveTextContent(
      "Ready"
    );
    expect(screen.queryByRole("button", { name: `Add ${entry.name}` })).toBeNull();
  });

  it("Should offer Turn on for an installed extension that is off [UT-033]", async () => {
    installedRow({ enabled: false });
    const enabled: unknown[] = [];
    server.use(
      http.put("*/api/extensions/:name/enablement", async ({ params, request }) => {
        enabled.push({ name: params.name, ...((await request.json()) as object) });
        return HttpResponse.json({ name: "github", enabled: true, profile: "default" });
      })
    );
    setup();
    const turnOn = await screen.findByRole("button", { name: `Turn on ${entry.name}` });
    expect(screen.queryByText("Ready")).toBeNull();
    await userEvent.click(turnOn);
    await waitFor(() =>
      expect(enabled).toEqual([expect.objectContaining({ name: "github", enabled: true })])
    );
  });

  // UT-034 [US-013.AC-2]: the detail names the source the plugin comes from.
  it("Should attribute the detail to its source [UT-034]", async () => {
    const detail = marketplaceCatalogDetailFixture(entry.entry_id)!;
    server.use(
      http.get("*/api/marketplace/entries/:entryId", () =>
        HttpResponse.json({
          ...detail,
          entry: { ...detail.entry, source: "claude-plugins-official" },
        })
      )
    );
    setup(undefined, `/marketplace/${entry.entry_id}?source=claude-plugins-official`);
    expect(await screen.findByTestId("marketplace-detail-source")).toHaveTextContent(
      "From Claude plugins"
    );
  });
});

// Invariant: only the exact successfully checked draft can register a source; collisions use daemon metadata.
// Owner: AddMarketplaceDialog over HTTP; existing Marketplace component suite.
describe("Marketplace source registration", () => {
  const source = {
    name: "team",
    source: "github:acme/team",
    kind: "custom",
    enabled: true,
    state: "ok",
    plugins: 1,
    installable: 1,
    stability: "experimental",
    diagnostics: [],
  };
  const preview = {
    name: "team",
    owner: "Acme",
    document_path: "marketplace.json",
    plugins: 1,
    installable: 1,
    diagnostics: [],
  };

  it("Should invalidate a successful preview on edit and register only the newly checked draft", async () => {
    const calls: Array<{ dryRun: boolean; body: unknown }> = [];
    server.use(
      http.post("*/api/marketplace/sources", async ({ request }) => {
        const dryRun = new URL(request.url).searchParams.get("dry_run") === "true";
        const body = await request.json();
        calls.push({ dryRun, body });
        return HttpResponse.json(dryRun ? preview : { source }, { status: dryRun ? 200 : 201 });
      })
    );
    const onAdded = vi.fn();
    setup(<AddMarketplaceDialog open onOpenChange={() => {}} onAdded={onAdded} />);
    const input = await screen.findByTestId("add-marketplace-ref");
    const submit = screen.getByTestId("add-marketplace-submit");
    expect(submit).toBeDisabled();
    await userEvent.type(input, "acme/old");
    await userEvent.tab();
    await waitFor(() => expect(submit).toBeEnabled());
    await userEvent.clear(input);
    await userEvent.type(input, "acme/team");
    expect(submit).toBeDisabled();
    await userEvent.tab();
    await waitFor(() => expect(submit).toBeEnabled());
    await userEvent.click(submit);
    await waitFor(() => expect(onAdded).toHaveBeenCalledWith(source));
    expect(calls.filter(call => !call.dryRun)).toEqual([
      { dryRun: false, body: { ref: "acme/team" } },
    ]);
  });

  it.each([
    {
      code: "marketplace_not_a_marketplace",
      checked: ["marketplace.json", ".claude-plugin/marketplace.json"],
      retained_by: undefined,
      text: ".claude-plugin/marketplace.json",
      field: "add-marketplace-ref",
    },
    {
      code: "marketplace_source_name_retained",
      checked: undefined,
      retained_by: ["protected-extension"],
      text: "protected-extension",
      field: "add-marketplace-name",
    },
    {
      code: "marketplace_source_name_reserved",
      checked: undefined,
      retained_by: undefined,
      text: "reserved",
      field: "add-marketplace-name",
    },
  ])(
    "Should expose actionable $code details and prevent registration",
    async ({ code, checked, retained_by, text, field }) => {
      server.use(
        http.post("*/api/marketplace/sources", () =>
          HttpResponse.json(
            {
              error:
                code === "marketplace_source_name_reserved" ? "Name is reserved" : "Source refused",
              code,
              checked,
              retained_by,
            },
            { status: code === "marketplace_source_name_retained" ? 409 : 422 }
          )
        )
      );
      setup(<AddMarketplaceDialog open onOpenChange={() => {}} />);
      await userEvent.type(await screen.findByTestId("add-marketplace-ref"), "acme/team");
      await userEvent.tab();
      expect(await screen.findByTestId("add-marketplace-failure")).toHaveTextContent(text);
      expect(screen.getByTestId(field)).toHaveAttribute("aria-invalid", "true");
      expect(screen.getByTestId("add-marketplace-submit")).toBeDisabled();
    }
  );

  it("Should check and submit the explicitly selected suggested name after a collision", async () => {
    const added: unknown[] = [];
    server.use(
      http.post("*/api/marketplace/sources", async ({ request }) => {
        const body = (await request.json()) as { ref: string; name?: string };
        if (!body.name)
          return HttpResponse.json(
            {
              error: "Name already registered",
              code: "marketplace_source_exists",
              suggested_name: "team-acme",
            },
            { status: 409 }
          );
        if (new URL(request.url).searchParams.get("dry_run") === "true")
          return HttpResponse.json({ ...preview, name: body.name });
        added.push(body);
        return HttpResponse.json({ source: { ...source, name: body.name } }, { status: 201 });
      })
    );
    setup(<AddMarketplaceDialog open onOpenChange={() => {}} />);
    await userEvent.type(await screen.findByTestId("add-marketplace-ref"), "acme/team");
    await userEvent.tab();
    const suggested = await screen.findByTestId("add-marketplace-use-suggested");
    expect(screen.getByTestId("add-marketplace-submit")).toBeDisabled();
    await userEvent.click(suggested);
    await waitFor(() => expect(screen.getByTestId("add-marketplace-submit")).toBeEnabled());
    expect(screen.getByTestId("add-marketplace-name")).toHaveValue("team-acme");
    await userEvent.click(screen.getByTestId("add-marketplace-submit"));
    await waitFor(() => expect(added).toEqual([{ ref: "acme/team", name: "team-acme" }]));
  });
});

// Invariant: feed/preset ownership constrains controls; mutations reread daemon state and preserve failures.
// Owner: Settings Marketplace sources integration in the canonical Marketplace component suite.
describe("Marketplace source settings", () => {
  it("Should keep the feed always on and toggle a preset without offering removal", async () => {
    let preset = {
      name: "preset",
      source: "github:acme/preset",
      kind: "preset",
      enabled: false,
      state: "off",
      plugins: 0,
      installable: 0,
      stability: "experimental",
      diagnostics: [],
    };
    const feed = {
      ...preset,
      name: "compozy-catalog",
      source: "catalog:compozy",
      kind: "feed",
      enabled: true,
      state: "ok",
    };
    const patches: unknown[] = [];
    server.use(
      http.get("*/api/marketplace/sources", () => HttpResponse.json({ sources: [feed, preset] })),
      http.patch("*/api/marketplace/sources/preset", async ({ request }) => {
        const body = (await request.json()) as { enabled: boolean };
        patches.push(body);
        preset = { ...preset, enabled: body.enabled, state: "ok", plugins: 1, installable: 1 };
        return HttpResponse.json({ source: preset });
      })
    );
    setup(<SettingsMarketplaceSourcesSection onAdd={() => {}} />);
    const feedRow = await screen.findByTestId("settings-page-marketplace-source-compozy-catalog");
    expect(within(feedRow).queryByRole("switch")).not.toBeInTheDocument();
    const row = await screen.findByTestId("settings-page-marketplace-source-preset");
    await userEvent.click(
      within(row).getByTestId("settings-page-marketplace-source-preset-disclosure")
    );
    expect(within(row).queryByRole("button", { name: "Remove" })).not.toBeInTheDocument();
    await userEvent.click(within(row).getByRole("switch"));
    await waitFor(() => expect(within(row).getByRole("switch")).toBeChecked());
    expect(patches).toEqual([{ enabled: true }]);
    expect(
      within(row).queryByTestId("settings-page-marketplace-source-preset-off")
    ).not.toBeInTheDocument();
  });

  it("Should show cached counts and diagnostics, then remove only after explicit confirmation", async () => {
    let sources = [
      {
        name: "team",
        source: "github:acme/team",
        kind: "custom",
        enabled: true,
        state: "degraded",
        plugins: 3,
        installable: 2,
        stability: "experimental",
        last_read_at: "2026-09-13T10:00:00Z",
        error: "Repository unavailable",
        error_class: "unavailable",
        diagnostics: [],
      },
    ];
    let removals = 0;
    server.use(
      http.get("*/api/marketplace/sources", () => HttpResponse.json({ sources })),
      http.delete("*/api/marketplace/sources/team", () => {
        removals++;
        if (removals === 1)
          return HttpResponse.json({ error: "Temporary removal failure" }, { status: 503 });
        sources = [];
        return new HttpResponse(null, { status: 204 });
      })
    );
    setup(<SettingsMarketplaceSourcesSection onAdd={() => {}} />);
    const row = await screen.findByTestId("settings-page-marketplace-source-team");
    expect(
      within(row).getByTestId("settings-page-marketplace-source-team-count")
    ).toHaveTextContent("3 plugins");
    await userEvent.click(
      within(row).getByTestId("settings-page-marketplace-source-team-disclosure")
    );
    expect(
      within(row).getByTestId("settings-page-marketplace-source-team-reason")
    ).toHaveTextContent("Repository unavailable");
    await userEvent.click(within(row).getByRole("button", { name: "Remove" }));
    expect(removals).toBe(0);
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(removals).toBe(0);
    await userEvent.click(within(row).getByRole("button", { name: "Remove" }));
    await userEvent.click(screen.getByTestId("settings-page-marketplace-sources-remove-confirm"));
    expect(await screen.findByRole("alert")).toHaveTextContent("Temporary removal failure");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await userEvent.click(within(row).getByRole("button", { name: "Remove" }));
    expect(
      within(screen.getByTestId("settings-page-marketplace-sources-remove-dialog")).queryByRole(
        "alert"
      )
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId("settings-page-marketplace-sources-remove-confirm"));
    await waitFor(() =>
      expect(screen.queryByTestId("settings-page-marketplace-source-team")).not.toBeInTheDocument()
    );
    expect(removals).toBe(2);
  });
});
