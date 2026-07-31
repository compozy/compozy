// Suite: Marketplace installed-detail management
// Invariant: Installed detail pages retain the owning kind's management controls after legacy shells are removed.
// Boundary IN: Kind-specific query and mutation view-models.
// Boundary OUT: Transport/cache behavior, owned by each system's hook suites.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MarketplaceDetailExtensionManage } from "../marketplace-detail-extension-manage";
import { MarketplaceDetailMCPManage } from "../marketplace-detail-mcp-manage";
import { MarketplaceDetailSkillManage } from "../marketplace-detail-skill-manage";
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
  extensionConsecutiveFailures: 0,
  extensionDev: false,
  extensionLogs: [] as Array<{ message: string; sequence: number; timestamp: string }>,
  extensionNavigate: vi.fn(),
  extensionOriginPath: undefined as string | undefined,
  extensionInstalledFrom: "marketplace_registry",
  extensionRemoteVersion: undefined as string | undefined,
  extensionDetailOptions: undefined as { updateVersion?: string } | undefined,
  extensionRequestUpdate: vi.fn(),
  extensionRestartBackoffMs: 0,
  extensionToggle: vi.fn(),
  extensionUpdateAvailable: false,
  extensionWorkspaceId: null as string | null,
  skillContent: "# Bundled skill\n\nFollow the incident checklist.",
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

vi.mock("@/systems/settings", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/settings")>();
  return {
    ...actual,
    MCPAuthorizeDialog: ({ scope }: { scope: string }) => (
      <output aria-label="Detail authorization scope">{scope}</output>
    ),
    useSettingsMCPServers: (
      filter: { scope?: string; workspace_id?: string },
      options?: { enabled?: boolean; refetchInterval?: number }
    ) => {
      mocks.mcpQueryCalls.push({ filter, options });
      return filter.scope === "global" ? mocks.globalMCP : mocks.workspaceMCP;
    },
    useMCPAuthorize: () => ({
      acknowledgeStatus: mocks.acknowledgeStatus,
      requestAuthorize: mocks.requestAuthorize,
      phase: mocks.authorizePhase,
    }),
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

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "workspace-a" }),
}));

vi.mock("@/systems/skill", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/skill")>();
  const { useState } = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    skillSourceTone: () => "neutral",
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
            enabled: true,
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
  };
});

vi.mock("@/systems/extensions", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/extensions")>();
  return {
    ...actual,
    ExtensionProvenanceDialog: () => null,
    RemoveExtensionDialog: () => null,
    useExtensionDetailState: (_name: string, options?: { updateVersion?: string }) => {
      mocks.extensionDetailOptions = options;
      return extensionDetailStateStub();
    },
  };
});

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
    bundles: {
      data: [
        {
          bundle_name: "ops-starter",
          extension_name: "ops-extension",
          id: "activation-ops-starter",
        },
      ],
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    },
    detail: {
      data: mocks.extensionError
        ? undefined
        : {
            extension: {
              bundles: [{ description: "On-call defaults", name: "ops-starter" }],
              diagnostics: [
                {
                  id: "healthy",
                  message: "Runtime handshake passed.",
                  severity: "info",
                  title: "Healthy",
                },
              ],
              enabled: true,
              capabilities: ["tool.provider"],
              consecutive_failures: mocks.extensionConsecutiveFailures,
              daemon_running: true,
              dev: mocks.extensionDev,
              digest_matched: false,
              missing_env: ["PAGER_TOKEN"],
              name: "ops-extension",
              health: "healthy",
              health_message: "Runtime handshake is healthy.",
              origin_path: mocks.extensionOriginPath,
              overrides_published: mocks.extensionDev,
              permissions: ["network/send"],
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

describe("Marketplace installed-detail management", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.skillError = null;
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
    mocks.extensionUpdateAvailable = false;
    mocks.extensionWorkspaceId = null;
    mocks.authorizePhase = "idle";
    mocks.mcpQueryCalls.length = 0;
    mocks.globalMCP = { data: undefined, error: null, isLoading: false, refetch: vi.fn() };
    mocks.workspaceMCP = { data: undefined, error: null, isLoading: false, refetch: vi.fn() };
  });

  it("Should preserve skill enablement, content, and shadow resolution", async () => {
    const user = userEvent.setup();
    render(<MarketplaceDetailSkillManage name="bundled-skill" />);

    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByTestId("skill-detail-activation-status")).toHaveTextContent("Inactive");
    expect(screen.getByTestId("skill-detail-activation-reasons")).toHaveTextContent(
      "Missing capability: browser.operate"
    );
    expect(screen.getByTestId("skill-enabled-switch")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("skill-capabilities-list")).toHaveTextContent("git.inspect");
    expect(screen.getByTestId("skill-recent-calls-table")).toHaveTextContent("Review pull request");
    expect(screen.getByTestId("skill-provenance-table")).toHaveTextContent("ops-extension");
    expect(screen.getByTestId("skill-shadow-table")).toHaveTextContent("workspace");
    await user.click(screen.getByTestId("view-full-content-btn"));
    expect(screen.getByTestId("content-body")).toHaveTextContent("Follow the incident checklist");
    await user.click(screen.getByTestId("skill-enabled-switch"));
    expect(mocks.disableSkill).toHaveBeenCalledWith({
      name: "bundled-skill",
      workspace: "workspace-a",
    });
  });

  it("Should surface a rejected skill availability update", async () => {
    const user = userEvent.setup();
    mocks.disableSkillError = new Error("Skill policy rejected the update");
    render(<MarketplaceDetailSkillManage name="bundled-skill" />);

    await user.click(screen.getByTestId("skill-enabled-switch"));

    expect(await screen.findByRole("alert")).toHaveTextContent("Skill policy rejected the update");
  });

  it("Should preserve extension enablement, environment, diagnostics, provenance, and activation link", async () => {
    const user = userEvent.setup();
    render(<MarketplaceDetailExtensionManage name="ops-extension" />);

    expect(screen.getByText("PAGER_TOKEN")).toBeInTheDocument();
    expect(screen.getByText("missing")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake passed.")).toBeInTheDocument();
    expect(screen.getByText("tool.provider")).toBeInTheDocument();
    expect(screen.getByText("network/send")).toBeInTheDocument();
    expect(screen.getByText("Runtime handshake is healthy.")).toBeInTheDocument();
    expect(screen.getByText("4242")).toBeInTheDocument();
    expect(screen.getByText("1h 1m")).toBeInTheDocument();
    expect(screen.getByText("official")).toBeInTheDocument();
    expect(screen.getByText("verified")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open active bundle →" })).toHaveAttribute(
      "href",
      "/marketplace/bundles/activations/activation-ops-starter"
    );
    await user.click(screen.getByTestId("extension-enabled-switch"));
    expect(mocks.extensionToggle).toHaveBeenCalledWith({ enabled: false, name: "ops-extension" });
  });

  it("Should report crash-loop counters, integrity evidence, and the log stream state", () => {
    mocks.extensionConsecutiveFailures = 3;
    mocks.extensionRestartBackoffMs = 4000;
    mocks.extensionLogs = [
      { message: "listening on :7788", sequence: 12, timestamp: "2026-07-20T10:00:00Z" },
    ];
    render(<MarketplaceDetailExtensionManage name="ops-extension" />);

    expect(screen.getByTestId("extension-consecutive-failures")).toHaveTextContent("3");
    expect(screen.getByTestId("extension-restart-backoff")).toHaveTextContent("4000 ms");
    expect(screen.getByTestId("extension-provenance-digest")).toHaveTextContent(
      "no digest recorded"
    );
    expect(screen.getByTestId("extension-checksum-verified-badge")).toBeInTheDocument();
    expect(screen.queryByTestId("extension-digest-matched-badge")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-logs-lines")).toHaveTextContent("listening on :7788");
    expect(screen.getByTestId("extension-logs-status")).toHaveTextContent("Following live");
  });

  it("Should reserve verified treatment for a curated pinned checksum", () => {
    mocks.extensionInstalledFrom = "local_path";

    render(<MarketplaceDetailExtensionManage name="ops-extension" />);

    expect(screen.queryByTestId("extension-checksum-verified-badge")).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-provenance-checksum")).toHaveTextContent("not pinned");
    expect(screen.getByTestId("extension-source-badge")).toHaveTextContent("Local path");
  });

  it("Should offer the update action only for a marketplace-managed published row", async () => {
    const user = userEvent.setup();
    mocks.extensionUpdateAvailable = true;
    mocks.extensionRemoteVersion = "v0.5.2";
    render(
      <MarketplaceDetailExtensionManage
        catalogUpdateAvailable
        catalogVersion="0.6.0"
        name="ops-extension"
      />
    );

    const updateAction = screen.getByTestId("extension-update-action");
    expect(updateAction).toHaveTextContent("Update to v0.6.0");
    expect(mocks.extensionDetailOptions).toEqual({
      logEventSourceFactory: undefined,
      updateVersion: "0.6.0",
    });
    await user.click(updateAction);

    expect(mocks.extensionRequestUpdate).toHaveBeenCalled();
  });

  it("Should isolate published-row actions from a workspace dev overlay", async () => {
    const user = userEvent.setup();
    mocks.extensionDev = true;
    mocks.extensionUpdateAvailable = true;
    mocks.extensionOriginPath = "/Users/dev/src/ops-extension";
    mocks.extensionWorkspaceId = "ws_northstar";
    render(<MarketplaceDetailExtensionManage name="ops-extension" />);

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

    render(<MarketplaceDetailExtensionManage name="ops-extension" />);

    expect(screen.getByTestId("marketplace-extension-manage-error")).toHaveTextContent(
      "Extension detail failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });

  it("Should show a recoverable state when installed skill management fails", () => {
    mocks.skillError = new Error("Skill detail failed");

    render(<MarketplaceDetailSkillManage name="bundled-skill" />);

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
      <MarketplaceDetailMCPManage
        entry={{
          description: "Shared MCP server",
          entry_id: "shared-server",
          installed: true,
          kind: "mcp",
          name: "shared-server",
          source: "installed",
          update_available: false,
        }}
      />
    );

    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(screen.getByTestId("marketplace-mcp-runtime-code")).toHaveTextContent(
      "process exited during initialize"
    );
    expect(screen.queryByText("Ready")).not.toBeInTheDocument();
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
      <MarketplaceDetailMCPManage
        entry={{
          description: "Exact installed identity",
          entry_id: "shared-catalog-entry",
          installed: true,
          installed_name: "exact-install",
          kind: "mcp",
          name: "Shared catalog entry",
          source: "installed",
          update_available: false,
        }}
      />
    );

    expect(screen.getByText("Config error")).toBeInTheDocument();
    expect(screen.queryByText("Ready")).not.toBeInTheDocument();
  });

  it("Should show a recoverable state when installed MCP management fails", () => {
    mocks.workspaceMCP.error = new Error("MCP inventory failed");

    render(
      <MarketplaceDetailMCPManage
        entry={{
          description: "Shared MCP server",
          entry_id: "shared-server",
          installed: true,
          kind: "mcp",
          name: "shared-server",
          source: "installed",
          update_available: false,
        }}
      />
    );

    expect(screen.getByTestId("marketplace-mcp-manage-error")).toHaveTextContent(
      "MCP inventory failed"
    );
    expect(screen.getByRole("button", { name: "Retry management" })).toBeInTheDocument();
  });

  it("Should begin authorization with prior state and acknowledge the polled completion", async () => {
    const user = userEvent.setup();
    const server: SettingsMCPServerEntry = {
      auth: {
        client_secret_configured: false,
        registration: "auto",
      },
      auth_status: {
        refreshable: true,
        scope: "global",
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
          scope: "global",
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

    const view = render(<MarketplaceDetailMCPManage entry={entry} />);
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
      "global"
    );
    expect(mocks.requestAuthorize).toHaveBeenCalledWith({ scope: "global" }, server);

    mocks.authorizePhase = "waiting";
    server.auth_status = {
      refreshable: true,
      scope: "global",
      server_name: "oauth-server",
      status: "authenticated",
      token_present: true,
    };
    view.rerender(<MarketplaceDetailMCPManage entry={entry} />);

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
