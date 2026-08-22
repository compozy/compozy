// Suite: Marketplace installed-detail management
// Invariant: Installed detail pages retain the owning kind's management controls and runtime
// facts across the redesigned body/rail anatomy.
// Boundary IN: Kind-specific query and mutation view-models.
// Boundary OUT: Transport/cache behavior, owned by each system's hook suites.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MarketplaceDetailExtensionInstalled } from "../marketplace-detail-extension-installed";
import { MarketplaceDetailMCPView } from "../marketplace-detail-mcp";
import { MarketplaceMCPDetailTopbarActions } from "../marketplace-detail-mcp-topbar";
import { MarketplaceDetailSkillInstalled } from "../marketplace-detail-skill-installed";
import type { MarketplaceEntryResponse } from "../../types";

import {
  SETTINGS_QUERY_INTERVALS,
  type SettingsMCPServerCollection,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

const mocks = vi.hoisted(() => ({
  acknowledgeStatus: vi.fn(),
  requestAuthorize: vi.fn(),
  disableSkill: vi.fn(),
  disableSkillError: null as Error | null,
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
  extensionNetworkConfirm: null as { action: "update"; digest: string } | null,
  extensionNetworkConfirmationRequired: false,
  extensionNetworkDigest: undefined as string | undefined,
  extensionLogs: [] as Array<{
    message: string;
    sequence: number;
    stream_epoch: string;
    timestamp: string;
  }>,
  extensionNavigate: vi.fn(),
  extensionOriginPath: undefined as string | undefined,
  extensionInstalledFrom: "marketplace_registry",
  extensionRemoteVersion: undefined as string | undefined,
  extensionDetailOptions: undefined as { updateVersion?: string } | undefined,
  extensionRequestUpdate: vi.fn(),
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
  skillContent: "# Bundled skill\n\nFollow the incident checklist.",
  skillEnabled: true,
  skillError: null as Error | null,
  extensionError: null as Error | null,
  extensionLoading: false,
  authorizePhase: "idle" as "idle" | "waiting",
  mcpQueryCalls: [] as Array<{
    filter: { scope?: string; workspace_id?: string };
    options?: { enabled?: boolean; refetchInterval?: number };
  }>,
  globalMCP: {
    data: undefined as SettingsMCPServerCollection | undefined,
    error: null as Error | null,
    isLoading: false,
    refetch: vi.fn(),
  },
  workspaceMCP: {
    data: undefined as SettingsMCPServerCollection | undefined,
    error: null as Error | null,
    isLoading: false,
    refetch: vi.fn(),
  },
}));

vi.mock("@/systems/profiles", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/profiles")>()),
  openProfileDialog: mocks.openProfileDialog,
}));

// The settings barrel participates in cross-system module cycles, so barrel-level
// overrides of these hooks are order-dependent; mocking their owning modules is not.
vi.mock("@/systems/settings/components/mcp-authorize-dialog", () => ({
  MCPAuthorizeDialog: ({ scope }: { scope: string }) => (
    <output aria-label="Detail authorization scope">{scope}</output>
  ),
}));

vi.mock("@/systems/settings/hooks/use-mcp-authorize", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/settings/hooks/use-mcp-authorize")>();
  return {
    ...actual,
    useMCPAuthorize: () => ({
      acknowledgeStatus: mocks.acknowledgeStatus,
      requestAuthorize: mocks.requestAuthorize,
      phase: mocks.authorizePhase,
    }),
  };
});

vi.mock("@/systems/settings/hooks/use-settings-collections", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/settings/hooks/use-settings-collections")>();
  return {
    ...actual,
    useSettingsMCPServers: (
      filter: { scope?: string; workspace_id?: string },
      options?: { enabled?: boolean; refetchInterval?: number }
    ) => {
      mocks.mcpQueryCalls.push({ filter, options });
      return filter.scope === "global" ? mocks.globalMCP : mocks.workspaceMCP;
    },
  };
});

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    params,
    to,
  }: {
    children?: React.ReactNode;
    params?: Record<string, string>;
    to: string;
  }) => {
    const href = params
      ? Object.entries(params).reduce(
          (current, [key, value]) => current.replace(`$${key}`, encodeURIComponent(value)),
          to
        )
      : to;
    return <a href={href}>{children}</a>;
  },
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "workspace-a" }),
}));

vi.mock("@/systems/skill/hooks/use-skill-actions", async () => {
  const { useState } = await vi.importActual<typeof import("react")>("react");
  return {
    useDisableSkill: () => {
      const [error, setError] = useState<Error | null>(null);
      return {
        error,
        isPending: false,
        mutate: (params: { name: string; workspace: string }) => {
          mocks.disableSkill(params);
          setError(mocks.disableSkillError);
        },
      };
    },
    useEnableSkill: () => ({ error: null, isPending: false, mutate: vi.fn() }),
  };
});

vi.mock("@/systems/skill/hooks/use-skills", () => ({
  useSkill: () => ({
    data: mocks.skillError
      ? undefined
      : {
          activation: {
            active: false,
            reasons: [
              {
                gate: "requires_capabilities",
                code: "missing_capability",
                missing: ["browser.operate"],
                message: "gate requires_capabilities unmet: browser.operate",
              },
            ],
          },
          enabled: mocks.skillEnabled,
          metadata: {
            capabilities: ["git.inspect", "git.review"],
            recent_calls: [
              {
                label: "Review pull request",
                status: "success",
                timestamp: "2026-07-18T12:00:00Z",
              },
            ],
          },
          name: "bundled-skill",
          provenance: {
            installed_from_extension: "ops-extension",
            precedence_tier: "bundled",
            registry: "builtin",
            slug: "bundled-skill",
            version: "1.2.0",
          },
          source: "bundled",
        },
    error: mocks.skillError,
    isLoading: false,
    refetch: vi.fn(),
  }),
  useSkillContent: (_name: string, _workspace: string, enabled: boolean) => ({
    data: enabled ? mocks.skillContent : undefined,
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
  useSkillShadows: () => ({
    data: {
      shadows: [
        {
          detected_at: "2026-07-18T12:00:00Z",
          path: "/workspace/.agents/skills/bundled-skill/SKILL.md",
          resolved_to_winner: true,
          tier: "workspace",
        },
      ],
    },
    error: null,
    isLoading: false,
  }),
}));

vi.mock("@/systems/extensions/hooks/use-extension-detail-state", () => ({
  useExtensionDetailState: (_name: string, options?: { updateVersion?: string }) => {
    mocks.extensionDetailOptions = options;
    return extensionDetailStateStub();
  },
}));

vi.mock("@/systems/extensions/components/extension-dialogs", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/extensions/components/extension-dialogs")>()),
  ExtensionProvenanceDialog: () => null,
  RemoveExtensionDialog: () => null,
}));

function extensionDetailStateStub() {
  return {
    logs: {
      entries: mocks.extensionLogs,
      error: null,
      follow: true,
      isLoading: false,
      refetch: vi.fn(),
      setFollow: vi.fn(),
      status: "live" as const,
    },
    update: { error: null, isPending: false },
    requestUpdate: mocks.extensionRequestUpdate,
    submitUpdate: vi.fn(),
    workspaceId: mocks.extensionWorkspaceId,
    dismissNetworkConfirm: vi.fn(),
    inventory: {
      data: {
        diagnostics: mocks.extensionInventoryDiagnostics,
        enabled: true,
        extension: "ops-extension",
        format: mocks.extensionFormat,
        items: mocks.extensionInventory,
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    },
    networkConfirm: mocks.extensionNetworkConfirm,
    requestToggle: mocks.extensionToggle,
    submitNetworkConfirm: vi.fn(),
    detail: {
      data: mocks.extensionError
        ? undefined
        : {
            extension: {
              bound_env_keys: mocks.extensionBoundEnvKeys,
              network_confirmation_required: mocks.extensionNetworkConfirmationRequired,
              network_requirement_digest: mocks.extensionNetworkDigest,
              diagnostics: [
                {
                  id: "healthy",
                  message: "Runtime handshake passed.",
                  severity: "info",
                  title: "Healthy",
                },
              ],
              declared_profiles: mocks.extensionDeclaredProfiles,
              dormant_placements: mocks.extensionPlacements.filter(item => item.dormant),
              enabled: true,
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
            },
          },
      error: mocks.extensionError,
      isLoading: mocks.extensionLoading,
      refetch: vi.fn(),
    },
    navigate: mocks.extensionNavigate,
    activeDialog: null,
    dismissDialog: vi.fn(),
    requestProvenance: vi.fn(),
    requestRemoval: vi.fn(),
    toggle: { isPending: false, mutate: mocks.extensionToggle },
  };
}

function skillDetailData(): MarketplaceEntryResponse {
  return {
    entry: {
      description: "Bundled incident-review skill.",
      entry_id: "bundled-skill",
      installed: true,
      installed_name: "bundled-skill",
      kind: "skill",
      name: "bundled-skill",
      source: "registry",
      update_available: false,
      version: "1.2.0",
    },
    skill: {
      install_slug: "compozy/bundled-skill",
      readme: "Bundled incident-review skill readme.",
      versions: ["1.2.0", "1.1.0"],
    },
  };
}

function extensionDetailData(
  overrides: Partial<MarketplaceEntryResponse["entry"]> = {}
): MarketplaceEntryResponse {
  return {
    entry: {
      description: "Operational automation kit.",
      entry_id: "ops-extension",
      installed: true,
      installed_name: "ops-extension",
      kind: "extension",
      name: "ops-extension",
      source: "registry",
      update_available: false,
      ...overrides,
    },
  };
}

async function openRailCard(user: ReturnType<typeof userEvent.setup>, name: string) {
  await user.click(screen.getByRole("button", { name: new RegExp(`^${name}`) }));
}

describe("Marketplace installed-detail management", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.skillError = null;
    mocks.skillEnabled = true;
    mocks.disableSkillError = null;
    mocks.extensionError = null;
    mocks.extensionLoading = false;
    mocks.extensionConsecutiveFailures = 0;
    mocks.extensionDev = false;
    mocks.extensionLogs = [];
    mocks.extensionOriginPath = undefined;
    mocks.extensionInstalledFrom = "marketplace_registry";
    mocks.extensionRemoteVersion = undefined;
    mocks.extensionDetailOptions = undefined;
    mocks.extensionRestartBackoffMs = 0;
    mocks.extensionBoundEnvKeys = [];
    mocks.extensionDeclaredProfiles = [];
    mocks.extensionFormat = "compozy";
    mocks.extensionInventory = [];
    mocks.extensionInventoryDiagnostics = [];
    mocks.extensionNetworkConfirm = null;
    mocks.extensionNetworkConfirmationRequired = false;
    mocks.extensionNetworkDigest = undefined;
    mocks.extensionPlacements = [];
    mocks.extensionUpdateAvailable = false;
    mocks.extensionWorkspaceId = null;
    mocks.authorizePhase = "idle";
    mocks.mcpQueryCalls.length = 0;
    mocks.globalMCP = { data: undefined, error: null, isLoading: false, refetch: vi.fn() };
    mocks.workspaceMCP = { data: undefined, error: null, isLoading: false, refetch: vi.fn() };
  });

  it("Should preserve skill enablement, content, and shadow resolution", async () => {
    const user = userEvent.setup();
    const view = render(<MarketplaceDetailSkillInstalled data={skillDetailData()} />);

    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByTestId("skill-detail-activation-status")).toHaveTextContent("Inactive");
    expect(screen.getByTestId("skill-detail-activation-reasons")).toHaveTextContent(
      "Missing capability: browser.operate"
    );
    expect(screen.getByTestId("skill-enabled-switch")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("skill-capabilities-list")).toHaveTextContent("git.inspect");
    expect(screen.getByTestId("skill-recent-calls-table")).toHaveTextContent("Review pull request");
    expect(screen.getByTestId("skill-shadow-table")).toHaveTextContent("workspace");
    await openRailCard(user, "Provenance");
    expect(screen.getByTestId("skill-provenance-table")).toHaveTextContent("ops-extension");
    await user.click(screen.getByTestId("view-full-content-btn"));
    expect(screen.getByTestId("content-body")).toHaveTextContent("Follow the incident checklist");
    await user.click(screen.getByTestId("skill-enabled-switch"));
    expect(mocks.disableSkill).toHaveBeenCalledWith({
      name: "bundled-skill",
      workspace: "workspace-a",
    });

    mocks.skillEnabled = false;
    view.rerender(<MarketplaceDetailSkillInstalled data={skillDetailData()} />);
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.getByTestId("skill-enabled-switch")).toHaveAttribute("aria-checked", "false");
  });

  it("Should surface a rejected skill availability update", async () => {
    const user = userEvent.setup();
    mocks.disableSkillError = new Error("Skill policy rejected the update");
    render(<MarketplaceDetailSkillInstalled data={skillDetailData()} />);

    await user.click(screen.getByTestId("skill-enabled-switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent("Skill policy rejected the update");
  });

  it("Should preserve extension enablement, environment, diagnostics, and provenance", async () => {
    const user = userEvent.setup();
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.getByText("PAGER_TOKEN")).toBeInTheDocument();
    expect(screen.getByText("missing")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake passed.")).toBeInTheDocument();
    expect(screen.getByText("tool.provider")).toBeInTheDocument();
    expect(screen.getByText("network/send")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake is healthy.")).toBeInTheDocument();
    expect(screen.getByText("4242")).toBeInTheDocument();
    expect(screen.getByText("1h 1m")).toBeInTheDocument();
    await openRailCard(user, "Provenance");
    expect(screen.getByText("official")).toBeInTheDocument();
    expect(screen.getByText("verified")).toBeInTheDocument();
    await user.click(screen.getByTestId("extension-enabled-switch"));
    expect(mocks.extensionToggle).toHaveBeenCalledOnce();
    expect(mocks.extensionToggle.mock.calls[0]?.[0]).toBe(false);
  });

  it("Should not present global kit inventory for a workspace dev overlay", () => {
    mocks.extensionWorkspaceId = "workspace-a";
    mocks.extensionDev = true;
    mocks.extensionInventory = [
      { id: "agent:dep-reviewer", kind: "agent", live: true, name: "dep-reviewer" },
    ];

    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.queryByTestId("extension-kit-inventory")).not.toBeInTheDocument();
  });

  // IT-015: the installed detail is where format and degradation have to agree — the badge comes
  // from the instance's own format and the Skipped rows from its recorded diagnostics.
  it("Should show the format badge and recorded skips for an ingested portable package", () => {
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

    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    const badges = screen.getByTestId("extension-trust-badges");
    expect(within(badges).getByTestId("extension-format-badge")).toHaveTextContent("agent plugin");
    const skipped = screen.getByTestId("extension-skipped-components");
    expect(within(skipped).getByText("mcp: legacy-events")).toBeInTheDocument();
    expect(within(skipped).getByText('unsupported transport "sse"')).toBeInTheDocument();
  });

  it("Should carry no format signal or Skipped section for a native extension", () => {
    mocks.extensionInventory = [
      { id: "agent:dep-reviewer", kind: "agent", live: true, name: "dep-reviewer" },
    ];

    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();
    expect(screen.queryByTestId("extension-skipped-components")).not.toBeInTheDocument();
  });

  it("Should mark a declared environment name as bound when the daemon reports a binding", () => {
    mocks.extensionBoundEnvKeys = ["REGION", "LEGACY_TOKEN"];
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    const environment = screen.getByTestId("extension-environment-state");
    expect(within(environment).getByText("bound")).toBeInTheDocument();
    // A binding the manifest no longer declares is listed, never presented as in use.
    expect(within(environment).getByText("LEGACY_TOKEN")).toBeInTheDocument();
    expect(within(environment).getByText("bound · not declared")).toBeInTheDocument();
  });

  it("Should surface the network digest and its consent state when participation is declared", () => {
    mocks.extensionNetworkDigest = "sha256:6f1c0a94d3b27e58";
    mocks.extensionNetworkConfirmationRequired = true;
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.getByTestId("extension-network-consent")).toHaveTextContent(
      "confirmation required"
    );
    expect(screen.getByText("sha256:6f1c0a94d3b27e58")).toBeInTheDocument();
  });

  it("Should mount the shared confirm dialog while a confirmation is pending", () => {
    mocks.extensionNetworkConfirm = { action: "update", digest: "sha256:6f1c0a94d3b27e58" };
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.getByTestId("extension-network-confirm-dialog")).toBeInTheDocument();
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
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

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
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

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

    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    await openRailCard(user, "Provenance");
    expect(screen.queryByTestId("extension-checksum-verified-badge")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-provenance-checksum")).toHaveTextContent("not pinned");
    expect(screen.getByTestId("extension-source-badge")).toHaveTextContent("Local path");
  });

  it("Should keep update off the rail — the OS-head action owns it", () => {
    mocks.extensionUpdateAvailable = true;
    mocks.extensionRemoteVersion = "v0.5.2";
    render(
      <MarketplaceDetailExtensionInstalled
        data={extensionDetailData({ update_available: true, version: "0.6.0" })}
      />
    );

    expect(screen.queryByTestId("extension-update-action")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-enabled-switch")).toBeInTheDocument();
    expect(mocks.extensionDetailOptions).toEqual({
      logEventSourceFactory: undefined,
      updateVersion: "0.6.0",
    });
  });

  it("Should isolate published-row actions from a workspace dev overlay", async () => {
    const user = userEvent.setup();
    mocks.extensionDev = true;
    mocks.extensionUpdateAvailable = true;
    mocks.extensionOriginPath = "/Users/dev/src/ops-extension";
    mocks.extensionWorkspaceId = "ws_northstar";
    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

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

  it("Should show a recoverable state when installed extension management fails", () => {
    mocks.extensionError = new Error("Extension detail failed");

    render(<MarketplaceDetailExtensionInstalled data={extensionDetailData()} />);

    expect(screen.getByTestId("marketplace-extension-manage-error")).toHaveTextContent(
      "Extension detail failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });

  it("Should show a recoverable state when installed skill management fails", () => {
    mocks.skillError = new Error("Skill detail failed");

    render(<MarketplaceDetailSkillInstalled data={skillDetailData()} />);

    expect(screen.getByTestId("marketplace-skill-manage-error")).toHaveTextContent(
      "Skill detail failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });

  it("Should prefer the workspace MCP definition and surface its dead runtime diagnostic", () => {
    const base = {
      name: "shared-server",
      transport: "stdio",
      source_metadata: {
        available_targets: [],
        shadowed_sources: [],
      },
    };
    mocks.globalMCP.data = {
      available_scopes: ["global", "workspace"],
      collection: "mcp-servers",
      mcp_servers: [
        {
          ...base,
          scope: "global",
          runtime_status: {
            configured: true,
            initialized: true,
            state: "ready",
            probe: "succeeded",
            tool_count: 3,
          },
          source_metadata: {
            ...base.source_metadata,
            effective_source: { kind: "global-config", scope: "global" },
          },
        },
      ],
      scope: "global",
    };
    mocks.workspaceMCP.data = {
      available_scopes: ["global", "workspace"],
      collection: "mcp-servers",
      mcp_servers: [
        {
          ...base,
          scope: "workspace",
          workspace_id: "workspace-a",
          runtime_status: {
            configured: true,
            initialized: false,
            state: "dead",
            probe: "skipped",
            tool_count: 0,
            reason: "backend_dead",
            diagnostic: "process exited during initialize",
          },
          source_metadata: {
            ...base.source_metadata,
            effective_source: { kind: "workspace-config", scope: "workspace" },
          },
        },
      ],
      scope: "workspace",
    };

    render(
      <MarketplaceDetailMCPView
        data={{
          entry: {
            description: "Shared MCP server",
            entry_id: "shared-server",
            installed: true,
            kind: "mcp",
            name: "shared-server",
            source: "installed",
            update_available: false,
          },
        }}
      />
    );

    expect(screen.getAllByText("unavailable").length).toBeGreaterThan(0);
    expect(screen.getByTestId("marketplace-mcp-runtime-code")).toHaveTextContent(
      "process exited during initialize"
    );
    expect(screen.queryByText("ready")).not.toBeInTheDocument();
  });

  it("Should resolve MCP management by exact installed name before shared catalog identity", () => {
    const shared = {
      catalog_entry: "shared-catalog-entry",
      scope: "workspace" as const,
      transport: "stdio" as const,
      workspace_id: "workspace-a",
      source_metadata: {
        available_targets: ["workspace-config" as const],
        effective_source: {
          kind: "workspace-config" as const,
          scope: "workspace" as const,
          workspace_id: "workspace-a",
        },
        shadowed_sources: [],
      },
    };
    mocks.workspaceMCP.data = {
      available_scopes: ["global", "workspace"],
      collection: "mcp-servers",
      mcp_servers: [
        {
          ...shared,
          name: "first-install",
          runtime_status: {
            configured: true,
            initialized: true,
            state: "ready",
            probe: "succeeded",
            tool_count: 4,
          },
        },
        {
          ...shared,
          name: "exact-install",
          runtime_status: {
            configured: true,
            initialized: false,
            state: "config_error",
            probe: "failed",
            tool_count: 0,
          },
        },
      ],
      scope: "workspace",
      workspace_id: "workspace-a",
    };

    render(
      <MarketplaceDetailMCPView
        data={{
          entry: {
            description: "Exact installed identity",
            entry_id: "shared-catalog-entry",
            installed: true,
            installed_name: "exact-install",
            kind: "mcp",
            name: "Shared catalog entry",
            source: "installed",
            update_available: false,
          },
        }}
      />
    );

    expect(screen.getAllByText("config error").length).toBeGreaterThan(0);
    expect(screen.queryByText("ready")).not.toBeInTheDocument();
  });

  it("Should show a recoverable state when installed MCP management fails", () => {
    mocks.workspaceMCP.error = new Error("MCP inventory failed");

    render(
      <MarketplaceDetailMCPView
        data={{
          entry: {
            description: "Shared MCP server",
            entry_id: "shared-server",
            installed: true,
            kind: "mcp",
            name: "shared-server",
            source: "installed",
            update_available: false,
          },
        }}
      />
    );

    expect(screen.getByTestId("marketplace-mcp-manage-error")).toHaveTextContent(
      "MCP inventory failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });

  it("Should begin authorization from the OS head and acknowledge the polled completion", async () => {
    const user = userEvent.setup();
    const server: SettingsMCPServerEntry = {
      auth: {
        client_secret_configured: false,
        registration: "auto",
      },
      auth_status: {
        refreshable: true,
        scope: "user",
        server_name: "oauth-server",
        status: "needs_login",
        token_present: false,
      },
      name: "oauth-server",
      scope: "workspace" as const,
      workspace_id: "workspace-a",
      transport: "http" as const,
      url: "https://oauth.example.test/mcp",
      runtime_status: {
        configured: true,
        initialized: false,
        probe: "not_attempted",
        state: "auth_required",
        tool_count: 0,
      },
      source_metadata: {
        available_targets: ["global-config"],
        effective_source: {
          kind: "global-config",
          scope: "user",
        },
        shadowed_sources: [],
      },
    };
    mocks.workspaceMCP.data = {
      available_scopes: ["global", "workspace"],
      collection: "mcp-servers",
      mcp_servers: [server],
      scope: "workspace",
      workspace_id: "workspace-a",
    };
    const entry = {
      description: "OAuth MCP server",
      entry_id: "oauth-server",
      installed: true,
      installed_name: "oauth-server",
      kind: "mcp",
      name: "oauth-server",
      source: "installed",
      update_available: false,
    } as const;

    const view = render(
      <MarketplaceMCPDetailTopbarActions entry={entry} onAction={vi.fn()} pending={false} />
    );
    expect(screen.getByText("needs authorization")).toBeInTheDocument();
    expect(mocks.mcpQueryCalls).toEqual(
      expect.arrayContaining([
        {
          filter: { scope: "workspace", workspace_id: "workspace-a" },
          options: {
            enabled: true,
            refetchInterval: SETTINGS_QUERY_INTERVALS.collectionRefetchInterval,
          },
        },
        {
          filter: { scope: "workspace", workspace_id: "workspace-a" },
          options: {
            enabled: false,
            refetchInterval: SETTINGS_QUERY_INTERVALS.mcpAuthStatusPollInterval,
          },
        },
      ])
    );
    await user.click(screen.getByTestId("mcp-authorize-btn"));

    expect(screen.getByRole("status", { name: "Detail authorization scope" })).toHaveTextContent(
      "user"
    );
    expect(mocks.requestAuthorize).toHaveBeenCalledWith({ scope: "user" }, server);

    mocks.authorizePhase = "waiting";
    server.auth_status = {
      refreshable: true,
      scope: "user",
      server_name: "oauth-server",
      status: "authenticated",
      token_present: true,
    };
    view.rerender(
      <MarketplaceMCPDetailTopbarActions entry={entry} onAction={vi.fn()} pending={false} />
    );

    expect(mocks.mcpQueryCalls).toContainEqual({
      filter: { scope: "workspace", workspace_id: "workspace-a" },
      options: {
        enabled: true,
        refetchInterval: SETTINGS_QUERY_INTERVALS.mcpAuthStatusPollInterval,
      },
    });
    expect(mocks.acknowledgeStatus).toHaveBeenCalledWith("authenticated", true);
  });
});
