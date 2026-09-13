// Invariant: installed extension details preserve management, runtime evidence, and dev-overlay protection.
// Owner: Marketplace installed-detail composition; canonical suite: marketplace-detail-manage.test.tsx.
// HTTP, EventSource, and the profile-dialog navigation command are the mocked I/O boundaries.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createRootRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from "@tanstack/react-router";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";
import { extensionFixtures } from "@/systems/extensions/mocks";
import type { ExtensionEntry, ExtensionLogEventSource } from "@/systems/extensions";
import { marketplaceCatalogFixture } from "../../mocks";
import type { MarketplaceCatalogEntryResponse } from "../../types";
import { MarketplaceDetailExtensionInstalled } from "../marketplace-detail-extension-installed";

const mocks = vi.hoisted(() => ({
  extensionEnabled: true,
  extensionBoundEnvKeys: [] as string[],
  extensionConsecutiveFailures: 0,
  extensionDeclaredProfiles: [] as Array<{
    created_by_extension: boolean;
    exists: boolean;
    name: string;
    needs_setup: boolean;
  }>,
  extensionDev: false,
  extensionFormat: "compozy",
  extensionInventory: [] as Array<{ id: string; kind: string; live: boolean; name: string }>,
  extensionInventoryDiagnostics: [] as Array<{
    category: string;
    code: string;
    data_freshness: string;
    evidence?: Record<string, unknown>;
    id: string;
    message: string;
    severity: string;
    title: string;
  }>,
  extensionNetworkConfirmationRequired: false,
  extensionNetworkDigest: undefined as string | undefined,
  extensionLogs: [] as Array<{
    message: string;
    sequence: number;
    stream_epoch: string;
    timestamp: string;
  }>,
  extensionOriginPath: undefined as string | undefined,
  extensionInstalledFrom: "marketplace_registry",
  extensionRemoteVersion: undefined as string | undefined,
  extensionRestartBackoffMs: 0,
  extensionToggle: vi.fn(),
  extensionPlacements: [] as Array<{
    create_action?: string;
    dormant: boolean;
    kind: string;
    profile?: string;
    resource: string;
  }>,
  openProfileDialog: vi.fn(),
  extensionUpdateAvailable: false,
  extensionWorkspaceId: null as string | null,
  extensionError: null as Error | null,
}));

vi.mock("@/systems/profiles", async original => ({
  ...(await original<typeof import("@/systems/profiles")>()),
  openProfileDialog: mocks.openProfileDialog,
}));
function extensionPayload(): ExtensionEntry {
  return {
    ...extensionFixtures[0]!,
    trust: undefined,
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 0 },
    workspace_id: mocks.extensionWorkspaceId ?? undefined,
    bound_env_keys: mocks.extensionBoundEnvKeys,
    network_confirmation_required: mocks.extensionNetworkConfirmationRequired,
    network_requirement_digest: mocks.extensionNetworkDigest,
    diagnostics: [
      {
        id: "healthy",
        category: "extension",
        code: "extension_healthy",
        data_freshness: "live",
        message: "Runtime handshake passed.",
        severity: "info",
        title: "Healthy",
      },
    ],
    inputs: [],
    declared_profiles: mocks.extensionDeclaredProfiles,
    dormant_placements: mocks.extensionPlacements.filter(item => item.dormant),
    enabled: mocks.extensionEnabled,
    capabilities: ["tool.provider"],
    consecutive_failures: mocks.extensionConsecutiveFailures,
    daemon_running: true,
    dev: mocks.extensionDev,
    digest_matched: false,
    format: mocks.extensionFormat,
    missing_env: ["PAGER_TOKEN"],
    name: "ops-extension",
    health: "healthy",
    health_message: "Runtime handshake is healthy.",
    origin_path: mocks.extensionOriginPath,
    overrides_published: mocks.extensionDev,
    permissions: ["network/send"],
    placements: mocks.extensionPlacements,
    pid: 4242,
    provenance: {
      allow_unverified: false,
      installed_at: "2026-09-12T12:00:00Z",
      installed_by: "operator:test",
      checksum_sha256: "a".repeat(64),
      checksum_verified: true,
      digest_matched: false,
      installed_from: mocks.extensionInstalledFrom,
      registry_tier: "official",
      slug: "compozy/ops-extension",
    },
    remote_version: mocks.extensionRemoteVersion,
    requires_env: ["PAGER_TOKEN", "REGION"],
    restart_backoff_ms: mocks.extensionRestartBackoffMs,
    source: "marketplace",
    state: "running",
    type: "backend",
    update_available: mocks.extensionUpdateAvailable,
    uptime_seconds: 3661,
    version: "0.5.2",
  };
}
const server = setupServer(
  ...profileHandlers,
  ...workspaceHandlers,
  ...statusHandlers,
  http.get("*/api/extensions", () =>
    mocks.extensionError
      ? HttpResponse.json({ error: mocks.extensionError.message }, { status: 400 })
      : HttpResponse.json({ extensions: [extensionPayload()] })
  ),
  http.get("*/api/extensions/:name/inventory", () =>
    HttpResponse.json({
      diagnostics: mocks.extensionInventoryDiagnostics,
      enabled: true,
      extension: "ops-extension",
      format: mocks.extensionFormat,
      items: mocks.extensionInventory,
    })
  ),
  http.get("*/api/extensions/:name/logs", () =>
    HttpResponse.json({ logs: mocks.extensionLogs, stream_epoch: "epoch-marketplace-test" })
  ),
  http.put("*/api/extensions/:name/enablement", async ({ request }) => {
    const body = (await request.json()) as { enabled: boolean; profile: string };
    mocks.extensionToggle(body.enabled);
    mocks.extensionEnabled = body.enabled;
    return HttpResponse.json(body);
  })
);
const clients: QueryClient[] = [];
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterAll(() => server.close());
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
});
function eventSource(): ExtensionLogEventSource {
  const source: ExtensionLogEventSource = {
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    close: vi.fn(),
    onopen: null,
    onerror: null,
  };
  queueMicrotask(() => source.onopen?.(new Event("open")));
  return source;
}
async function renderDetail(data: MarketplaceCatalogEntryResponse) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  const root = createRootRoute({
    component: () => (
      <MarketplaceDetailExtensionInstalled data={data} logEventSourceFactory={eventSource} />
    ),
  });
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ["/"] }),
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  if (mocks.extensionError) await screen.findByTestId("marketplace-extension-manage-error");
  else {
    await screen.findByTestId("extension-environment-state");
    await waitFor(() =>
      expect(screen.getByTestId("extension-logs-status")).toHaveTextContent("Following live")
    );
    if (mocks.extensionWorkspaceId === null)
      await waitFor(() => expect(client.isFetching()).toBe(0));
  }
}
function extensionDetailData(
  overrides: Partial<MarketplaceCatalogEntryResponse["entry"]> = {}
): MarketplaceCatalogEntryResponse {
  return {
    entry: {
      ...marketplaceCatalogFixture.items[0]!,
      description: "Operational automation kit.",
      entry_id: "ops-extension",
      installed: true,
      installed_name: "ops-extension",
      name: "ops-extension",
      update_available: false,
      ...overrides,
    },
  };
}
async function openRailCard(user: ReturnType<typeof userEvent.setup>, name: string) {
  const trigger = screen.getByRole("button", { name: new RegExp(`^${name}`) });
  await user.click(trigger);
  const panel = document.getElementById(trigger.getAttribute("aria-controls") ?? "");
  if (!panel) throw new Error(`The ${name} disclosure must reference its panel`);
  return panel;
}

describe("Marketplace installed-detail management", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.extensionEnabled = true;
    mocks.extensionError = null;
    mocks.extensionConsecutiveFailures = 0;
    mocks.extensionDev = false;
    mocks.extensionLogs = [];
    mocks.extensionOriginPath = undefined;
    mocks.extensionInstalledFrom = "marketplace_registry";
    mocks.extensionRemoteVersion = undefined;
    mocks.extensionRestartBackoffMs = 0;
    mocks.extensionBoundEnvKeys = [];
    mocks.extensionDeclaredProfiles = [];
    mocks.extensionFormat = "compozy";
    mocks.extensionInventory = [];
    mocks.extensionInventoryDiagnostics = [];
    mocks.extensionNetworkConfirmationRequired = false;
    mocks.extensionNetworkDigest = undefined;
    mocks.extensionPlacements = [];
    mocks.extensionUpdateAvailable = false;
    mocks.extensionWorkspaceId = null;
  });

  it("Should preserve extension enablement, environment, diagnostics, and provenance", async () => {
    const user = userEvent.setup();
    await renderDetail(extensionDetailData());

    expect(screen.getByText("PAGER_TOKEN")).toBeInTheDocument();
    expect(screen.getByText("missing")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake passed.")).toBeInTheDocument();
    expect(screen.getByText("tool.provider")).toBeInTheDocument();
    expect(screen.getByText("network/send")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake is healthy.")).toBeInTheDocument();
    expect(screen.getByText("4242")).toBeInTheDocument();
    expect(screen.getByText("1h 1m")).toBeInTheDocument();
    const provenance = await openRailCard(user, "Provenance");
    expect(within(provenance).getByText("official")).toBeInTheDocument();
    expect(screen.getByTestId("extension-checksum-verified-badge")).toBeInTheDocument();
    await user.click(screen.getByTestId("extension-enabled-switch"));
    await waitFor(() => expect(mocks.extensionToggle).toHaveBeenCalledOnce());
    expect(mocks.extensionToggle.mock.calls[0]?.[0]).toBe(false);
    await waitFor(() =>
      expect(screen.getByTestId("extension-enabled-switch")).toHaveAttribute(
        "aria-checked",
        "false"
      )
    );
  });

  it("Should not present global kit inventory for a workspace dev overlay", async () => {
    mocks.extensionWorkspaceId = "workspace-a";
    mocks.extensionDev = true;
    mocks.extensionInventory = [
      { id: "agent:dep-reviewer", kind: "agent", live: true, name: "dep-reviewer" },
    ];

    await renderDetail(extensionDetailData());

    expect(screen.queryByTestId("extension-kit-inventory")).not.toBeInTheDocument();
  });

  // IT-015: the installed detail is where format and degradation have to agree — the badge comes
  // from the instance's own format and the Skipped rows from its recorded diagnostics.
  it("Should show the format badge and recorded skips for an ingested portable package", async () => {
    mocks.extensionFormat = "agent-plugin";
    mocks.extensionInventory = [
      { id: "acme.tools/deploy-check", kind: "skill", live: true, name: "deploy-check" },
    ];
    mocks.extensionInventoryDiagnostics = [
      {
        category: "extension",
        code: "extension_agent_plugin_component_skipped",
        data_freshness: "live",
        evidence: { component: "mcp_server", scope: "mcp:legacy-events" },
        id: "extension/acme.tools/agent-plugin/mcp:legacy-events",
        message: 'mcp server "legacy-events": unsupported transport "sse"',
        severity: "warn",
        title: "Component skipped",
      },
    ];

    await renderDetail(extensionDetailData());

    const badges = screen.getByTestId("extension-trust-badges");
    expect(within(badges).getByTestId("extension-format-badge")).toHaveTextContent("agent plugin");
    const skipped = screen.getByTestId("extension-skipped-components");
    expect(within(skipped).getByText("mcp: legacy-events")).toBeInTheDocument();
    expect(within(skipped).getByText('unsupported transport "sse"')).toBeInTheDocument();
  });

  it("Should carry no format signal or Skipped section for a native extension", async () => {
    mocks.extensionInventory = [
      { id: "agent:dep-reviewer", kind: "agent", live: true, name: "dep-reviewer" },
    ];

    await renderDetail(extensionDetailData());

    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();
    expect(screen.queryByTestId("extension-skipped-components")).not.toBeInTheDocument();
  });

  it("Should mark a declared environment name as bound when the daemon reports a binding", async () => {
    mocks.extensionBoundEnvKeys = ["REGION", "LEGACY_TOKEN"];
    await renderDetail(extensionDetailData());

    const environment = screen.getByTestId("extension-environment-state");
    expect(within(environment).getByText("bound")).toBeInTheDocument();
    // A binding the manifest no longer declares is listed, never presented as in use.
    expect(within(environment).getByText("LEGACY_TOKEN")).toBeInTheDocument();
    expect(within(environment).getByText("bound · not declared")).toBeInTheDocument();
  });

  it("Should surface the network digest and its consent state when participation is declared", async () => {
    mocks.extensionNetworkDigest = "sha256:6f1c0a94d3b27e58";
    mocks.extensionNetworkConfirmationRequired = true;
    await renderDetail(extensionDetailData());

    expect(screen.getByTestId("extension-network-consent")).toHaveTextContent(
      "confirmation required"
    );
    expect(screen.getByText("sha256:6f1c0a94d3b27e58")).toBeInTheDocument();
  });

  it("Should show declared profile setup and dormant placement recovery", async () => {
    const user = userEvent.setup();
    mocks.extensionDeclaredProfiles = [
      { created_by_extension: true, exists: true, name: "operations", needs_setup: true },
      { created_by_extension: false, exists: false, name: "growth", needs_setup: false },
    ];
    mocks.extensionPlacements = [
      {
        create_action: "profile.create",
        dormant: true,
        kind: "skill",
        profile: "growth",
        resource: "weekly-audit",
      },
    ];
    await renderDetail(extensionDetailData());

    const profiles = screen.getByTestId("extension-declared-profiles");
    expect(within(profiles).getByText("operations")).toBeInTheDocument();
    expect(within(profiles).getByText("Needs setup")).toBeInTheDocument();
    expect(within(profiles).getByText("Dormant")).toBeInTheDocument();
    await user.click(within(profiles).getByRole("button", { name: "Create profile" }));
    expect(mocks.openProfileDialog).toHaveBeenCalledWith({ flow: "create", profile: "growth" });
  });

  it("Should report crash-loop counters, integrity evidence, and the log stream state", async () => {
    const user = userEvent.setup();
    mocks.extensionConsecutiveFailures = 3;
    mocks.extensionRestartBackoffMs = 4000;
    mocks.extensionLogs = [
      {
        message: "listening on :7788",
        sequence: 12,
        stream_epoch: "epoch-marketplace-test",
        timestamp: "2026-07-20T10:00:00Z",
      },
    ];
    await renderDetail(extensionDetailData());

    expect(screen.getByTestId("extension-consecutive-failures")).toHaveTextContent("3");
    expect(screen.getByTestId("extension-restart-backoff")).toHaveTextContent("4000 ms");
    await openRailCard(user, "Provenance");
    expect(screen.getByTestId("extension-provenance-digest")).toHaveTextContent(
      "no digest recorded"
    );
    expect(screen.getByTestId("extension-checksum-verified-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("extension-digest-matched-badge")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-logs-lines")).toHaveTextContent("listening on :7788");
    expect(screen.getByTestId("extension-logs-status")).toHaveTextContent("Following live");
  });

  it("Should reserve verified treatment for a curated pinned checksum", async () => {
    const user = userEvent.setup();
    mocks.extensionInstalledFrom = "local_path";

    await renderDetail(extensionDetailData());

    await openRailCard(user, "Provenance");
    expect(screen.queryByTestId("extension-checksum-verified-badge")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-provenance-checksum")).toHaveTextContent("not pinned");
    expect(screen.getByTestId("extension-source-badge")).toHaveTextContent("Local path");
  });

  it("Should keep update off the rail — the OS-head action owns it", async () => {
    mocks.extensionUpdateAvailable = true;
    mocks.extensionRemoteVersion = "v0.5.2";
    await renderDetail(extensionDetailData({ update_available: true, version: "0.6.0" }));

    expect(screen.queryByTestId("extension-update-action")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-enabled-switch")).toBeInTheDocument();
  });

  it("Should isolate published-row actions from a workspace dev overlay", async () => {
    const user = userEvent.setup();
    mocks.extensionDev = true;
    mocks.extensionUpdateAvailable = true;
    mocks.extensionOriginPath = "/Users/dev/src/ops-extension";
    mocks.extensionWorkspaceId = "ws_northstar";
    await renderDetail(extensionDetailData());

    expect(screen.getByTestId("extension-dev-badge")).toHaveTextContent("dev");
    expect(screen.getByTestId("extension-overrides-published-badge")).toHaveTextContent(
      "overrides published"
    );
    expect(screen.getByTestId("extension-origin-path")).toHaveTextContent(
      "/Users/dev/src/ops-extension"
    );
    expect(screen.queryByTestId("extension-update-action")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-enabled-switch")).toHaveAttribute("aria-disabled", "true");

    await user.click(screen.getByRole("button", { name: "Actions for ops-extension" }));

    expect(
      await screen.findByRole("menuitem", { name: "Unlink dev overlay…" })
    ).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Provenance" })).not.toBeInTheDocument();
  });

  it("Should show a recoverable state when installed extension management fails", async () => {
    mocks.extensionError = new Error("Extension detail failed");

    await renderDetail(extensionDetailData());

    expect(screen.getByTestId("marketplace-extension-manage-error")).toHaveTextContent(
      "Extension detail failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });
});
