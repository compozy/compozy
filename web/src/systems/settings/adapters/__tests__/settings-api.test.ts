import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  deleteSettingsSandbox,
  deleteSettingsHook,
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  getSettingsGeneral,
  getSettingsCmdPalette,
  getSettingsObservability,
  getSettingsPersona,
  getRolesStatus,
  getSettingsRestartStatus,
  getSettingsRoles,
  getSettingsSkills,
  listSettingsSandboxes,
  listSettingsApplyRecords,
  listSettingsHooks,
  listSettingsMCPServers,
  listSettingsProviders,
  putSettingsSandbox,
  putSettingsHook,
  putSettingsMCPServer,
  putSettingsProvider,
  reloadSettings,
  SettingsApiError,
  settingsObservabilityLogTailPath,
  triggerSettingsRestart,
  updateSettingsAutomation,
  updateSettingsCmdPalette,
  updateSettingsGeneral,
  updateSettingsPersona,
  updateSettingsRoles,
  updateSettingsSkills,
} from "../settings-api";
import {
  rolesStatusFixture,
  settingsRolesConfigWithFallbackFixture,
  settingsRolesSectionFixture,
} from "../../mocks/roles-fixtures";

const generalSectionFixture = {
  section: "general" as const,
  scope: "user" as const,
  available_scopes: ["user" as const],
  actions: {
    restart: {
      available: true,
      behavior: "action_trigger" as const,
      name: "restart",
    },
  },
  config: {
    daemon: { socket: "/tmp/compozy.sock" },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 4 },
    permissions: { mode: "approve-reads" as const },
    session_timeout: "30m",
  },
  config_paths: {
    daemon_info: "/home/compozy/.compozy/daemon.json",
    global_config: "/home/compozy/.compozy/config.toml",
    global_mcp_sidecar: "/home/compozy/.compozy/mcp.json",
    home_dir: "/home/compozy/.compozy",
    log_file: "/home/compozy/.compozy/compozy.log",
  },
  runtime: {
    active_agents: 2,
    active_sessions: 3,
    available: true,
    total_sessions: 100,
    uptime_seconds: 12000,
  },
};

const mutationFixture = {
  active_config_hash: "sha256:active-live",
  active_generation: 42,
  section: "general" as const,
  scope: "user" as const,
  applied: true,
  apply_record_id: "cfg_apply_001",
  lifecycle: "restart-required" as const,
  next_action: "restart-daemon" as const,
  restart_required: true,
  restart_scope: "daemon",
  warnings: ["restart the daemon"],
  write_target: "global-config" as const,
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("SettingsApiError", () => {
  it("captures status code and name", () => {
    const err = new SettingsApiError("boom", 500);
    expect(err.name).toBe("SettingsApiError");
    expect(err.status).toBe(500);
  });
});

describe("section reads and updates", () => {
  it("loads the general section envelope", async () => {
    mockJsonResponse(generalSectionFixture);

    const result = await getSettingsGeneral();

    expect(result).toEqual(generalSectionFixture);
    await expectFetchRequest({ path: "/api/settings/general" });
  });

  it("loads the observability section and exposes log tail URL", async () => {
    const observability = {
      section: "observability" as const,
      scope: "user" as const,
      available_scopes: ["user" as const],
      config: {
        enabled: true,
        max_global_bytes: 1024,
        retention_days: 7,
        transcripts: {
          enabled: true,
          max_bytes_per_session: 4096,
          segment_bytes: 2048,
        },
      },
      log_tail: {
        available: true,
        stream_url: "/api/settings/observability/log-tail",
        transport: "sse" as const,
      },
      runtime: {
        active_agents: 0,
        active_sessions: 0,
        available: true,
        global_db_size_bytes: 128,
        session_db_size_bytes: 64,
        uptime_seconds: 1000,
      },
    };

    mockJsonResponse(observability);

    const result = await getSettingsObservability();

    expect(result.log_tail.available).toBe(true);
    expect(settingsObservabilityLogTailPath()).toBe("/api/settings/observability/log-tail");
    await expectFetchRequest({ path: "/api/settings/observability" });
  });

  it("updates the general section and returns the mutation result", async () => {
    mockJsonResponse(mutationFixture);

    const body = {
      config: {
        daemon: {
          memory_report_interval: "10m",
          reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
          socket: "/tmp/next.sock",
        },
        http: { host: "127.0.0.1", port: 2123 },
        limits: { max_concurrent_agents: 4 },
        permissions: { mode: "approve-reads" as const },
        redact: { enabled: false },
        session_timeout: "45m",
      },
    };

    const result = await updateSettingsGeneral(body);

    expect(result).toEqual(mutationFixture);
    expect(result.restart_required).toBe(true);
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/settings/general",
    });
  });

  it("reads persona defaults from the selected profile layer", async () => {
    const persona = {
      section: "persona" as const,
      scope: "profile" as const,
      profile: "marketing",
      available_scopes: ["user", "profile", "workspace"] as const,
      config: { agent: "campaigns", provider: "openai", sandbox: "browser" },
    };
    mockJsonResponse(persona);

    const result = await getSettingsPersona({ scope: "profile", profile: " marketing " });

    expect(result).toEqual(persona);
    await expectFetchRequest({
      path: "/api/settings/persona?scope=profile&profile=marketing",
    });
  });

  it("updates persona defaults in the selected layer", async () => {
    mockJsonResponse({
      ...mutationFixture,
      section: "persona",
      scope: "profile",
      profile: "marketing",
      write_target: "profile-config",
    });
    const body = {
      config: { agent: "campaigns", provider: "openai", sandbox: "browser" },
    };

    const result = await updateSettingsPersona(body, {
      scope: "profile",
      profile: "marketing",
    });

    expect(result).toMatchObject({
      scope: "profile",
      profile: "marketing",
      write_target: "profile-config",
    });
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/settings/persona?scope=profile&profile=marketing",
    });
  });

  it("updates the automation section", async () => {
    mockJsonResponse({ ...mutationFixture, section: "automation" as const });

    const body = {
      config: {
        default_fire_limit: { max: 12, window: "1h" },
        enabled: true,
        max_concurrent_jobs: 5,
        timezone: "UTC",
      },
    };

    const result = await updateSettingsAutomation(body);

    expect(result.section).toBe("automation");
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/settings/automation",
    });
  });

  it("loads the skills section with agent-scope query params", async () => {
    const skillsSection = {
      section: "skills" as const,
      scope: "agent" as const,
      available_scopes: ["user" as const, "agent" as const],
      agent_name: "coder",
      workspace_id: "ws-polybot",
      runtime_available: true,
      discovered_count: 3,
      disabled_count: 1,
      config: {
        enabled: true,
        disabled_skills: ["review"],
        poll_interval: "5m",
        marketplace: { registry: "compozy" },
      },
      links: [{ label: "skills", path: "/marketplace/skills" }],
    };

    mockJsonResponse(skillsSection);

    const result = await getSettingsSkills({
      scope: "agent",
      agent_name: " coder ",
      workspace_id: " ws-polybot ",
    });

    expect(result).toEqual(skillsSection);
    await expectFetchRequest({
      path: "/api/settings/skills?scope=agent&workspace_id=ws-polybot&agent_name=coder",
    });
  });

  it("updates the skills section with agent-scope query params", async () => {
    mockJsonResponse({
      ...mutationFixture,
      section: "skills" as const,
      scope: "agent" as const,
      agent_name: "coder",
      workspace_id: "ws-polybot",
      write_target: "workspace-agent-file" as const,
    });

    const body = {
      config: {
        enabled: true,
        disabled_skills: ["review"],
        poll_interval: "5m",
        marketplace: { registry: "compozy" },
        sources: ["agents"],
        custom_sources: [],
      },
    };

    const result = await updateSettingsSkills(body, {
      scope: "agent",
      agent_name: " coder ",
      workspace_id: " ws-polybot ",
    });

    expect(result.scope).toBe("agent");
    expect(result.write_target).toBe("workspace-agent-file");
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/settings/skills?scope=agent&workspace_id=ws-polybot&agent_name=coder",
    });
  });

  it("throws typed errors on failed section reads", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(getSettingsGeneral()).rejects.toBeInstanceOf(SettingsApiError);
    await expect(getSettingsGeneral()).rejects.toThrow("Failed to load general settings: 500");
  });
});

describe("roles section", () => {
  it("loads the effective roles projection", async () => {
    mockJsonResponse(rolesStatusFixture);

    const result = await getRolesStatus();

    expect(result.roles).toHaveLength(6);
    await expectFetchRequest({ path: "/api/roles" });
  });

  it("loads the editable roles settings section", async () => {
    mockJsonResponse(settingsRolesSectionFixture);

    const result = await getSettingsRoles();

    expect(result.section).toBe("roles");
    expect(result.config.memory_controller.timeout).toBe("250ms");
    await expectFetchRequest({ path: "/api/settings/roles" });
  });

  it("submits the full roles section through the config-apply plane", async () => {
    mockJsonResponse({ ...mutationFixture, section: "roles" as const });

    const body = { config: settingsRolesConfigWithFallbackFixture };
    const result = await updateSettingsRoles(body);

    expect(result.section).toBe("roles");
    await expectFetchRequest({ body, method: "PATCH", path: "/api/settings/roles" });
  });
});

describe("collection endpoints", () => {
  it("lists providers", async () => {
    const providers = {
      collection: "providers" as const,
      scope: "user" as const,
      available_scopes: ["user" as const],
      providers: [
        {
          name: "openai",
          default: true,
          command_available: true,
          settings: {
            command: "codex",
            auth_mode: "native_cli",
            env_policy: "filtered",
            home_policy: "operator",
            auth_status_command: "codex auth status",
            credential_slots: [
              {
                name: "api_key",
                target_env: "OPENAI_API_KEY",
                secret_ref: "env:OPENAI_API_KEY",
                kind: "api_key",
                required: false,
              },
            ],
          },
          source_metadata: {
            available_targets: ["global-config" as const],
            effective_source: { kind: "global-config" as const, scope: "user" as const },
          },
          auth_status: {
            mode: "native_cli",
            env_policy: "filtered",
            home_policy: "operator",
            state: "unknown",
            code: "provider_classification_unknown",
            login: {
              configured: true,
              executable: "codex",
              presence: "unknown",
              recommended_action: "inspect",
              source: "auth_login_command",
            },
          },
        },
      ],
    };

    mockJsonResponse(providers);
    const result = await listSettingsProviders();

    expect(result.providers).toHaveLength(1);
    expect(result.providers[0]?.source_metadata.effective_source.kind).toBe("global-config");
    expect(result.providers[0]?.settings.auth_mode).toBe("native_cli");
    expect(result.providers[0]?.auth_status?.state).toBe("unknown");
    expect(result.providers[0]?.settings).not.toHaveProperty("auth_login_command");
    expect(result.providers[0]?.auth_status?.login).toEqual({
      configured: true,
      executable: "codex",
      presence: "unknown",
      recommended_action: "inspect",
      source: "auth_login_command",
    });
    await expectFetchRequest({ path: "/api/settings/providers" });
  });

  it("puts a provider overlay and reports write target", async () => {
    mockJsonResponse({ ...mutationFixture, section: "general" as const });

    const result = await putSettingsProvider("openai", {
      settings: {
        command: "codex",
        models: {
          default: "gpt-5",
          curated: [{ id: "gpt-5" }],
        },
        auth_mode: "native_cli",
        env_policy: "filtered",
        home_policy: "operator",
        auth_login_command: "codex login",
      },
    });

    expect(result.write_target).toBe("global-config");
    await expectFetchRequest({
      body: {
        settings: {
          command: "codex",
          models: {
            default: "gpt-5",
            curated: [{ id: "gpt-5" }],
          },
          auth_mode: "native_cli",
          env_policy: "filtered",
          home_policy: "operator",
          auth_login_command: "codex login",
        },
      },
      method: "PUT",
      path: "/api/settings/providers/openai",
    });
  });

  it("maps 404 to a not-found delete error for providers", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(deleteSettingsProvider("missing")).rejects.toThrow("Provider not found: missing");
  });

  it("lists sandboxes", async () => {
    const sandboxes = {
      collection: "sandboxes" as const,
      scope: "user" as const,
      available_scopes: ["user" as const],
      sandboxes: [
        {
          name: "local",
          profile: { backend: "host" },
          workspace_usage_count: 2,
          source_metadata: {
            available_targets: ["global-config" as const],
            effective_source: { kind: "global-config" as const, scope: "user" as const },
          },
        },
      ],
    };

    mockJsonResponse(sandboxes);
    const result = await listSettingsSandboxes();

    expect(result.sandboxes[0]?.workspace_usage_count).toBe(2);
    await expectFetchRequest({ path: "/api/settings/sandboxes" });
  });

  it("deletes sandboxes and propagates mutation responses", async () => {
    mockJsonResponse({ ...mutationFixture, section: "general" as const });

    const result = await deleteSettingsSandbox("local");

    expect(result.applied).toBe(true);
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/settings/sandboxes/local",
    });
  });

  it("puts sandboxes", async () => {
    mockJsonResponse({ ...mutationFixture, section: "general" as const });

    const body = { profile: { backend: "daytona" } };
    await putSettingsSandbox("cloud", body);

    await expectFetchRequest({
      body,
      method: "PUT",
      path: "/api/settings/sandboxes/cloud",
    });
  });

  it("lists hooks", async () => {
    const hooks = {
      collection: "hooks" as const,
      scope: "user" as const,
      available_scopes: ["user" as const],
      hooks: [],
    };

    mockJsonResponse(hooks);
    await listSettingsHooks();
    await expectFetchRequest({ path: "/api/settings/hooks" });
  });

  it("serializes the selected profile for hook list, put, and delete", async () => {
    mockJsonResponse({ collection: "hooks", scope: "profile", hooks: [] });
    await listSettingsHooks({ scope: "profile", profile: " marketing " });
    await expectFetchRequest({ path: "/api/settings/hooks?scope=profile&profile=marketing" });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse(mutationFixture);
    const body = {
      declaration: { event: "tool.pre_call" as const, matcher: {}, name: "preflight" },
    };
    await putSettingsHook("preflight", body, { scope: "profile", profile: " marketing " });
    await expectFetchRequest({
      body,
      method: "PUT",
      path: "/api/settings/hooks/preflight?scope=profile&profile=marketing",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse(mutationFixture);
    await deleteSettingsHook("preflight", { scope: "profile", profile: " marketing " });
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/settings/hooks/preflight?scope=profile&profile=marketing",
    });
  });

  it("lists MCP servers with scope filter and preserves workspace identifier", async () => {
    const mcp = {
      collection: "mcp-servers" as const,
      scope: "workspace" as const,
      available_scopes: ["user" as const, "workspace" as const],
      mcp_servers: [],
      workspace_id: "ws_alpha",
    };

    mockJsonResponse(mcp);

    await listSettingsMCPServers({ scope: "workspace", workspace_id: "  ws_alpha  " });

    await expectFetchRequest({
      path: "/api/settings/mcp-servers?scope=workspace&workspace_id=ws_alpha",
    });
  });

  it("puts MCP server with target selector", async () => {
    mockJsonResponse({ ...mutationFixture, section: "general" as const });

    await putSettingsMCPServer(
      "github",
      { server: { name: "github", command: "gh" } },
      { scope: "user", target: "sidecar" }
    );

    await expectFetchRequest({
      body: { server: { name: "github", command: "gh" } },
      method: "PUT",
      path: "/api/settings/mcp-servers/github?scope=user&target=sidecar",
    });
  });

  it("deletes MCP server with auto target", async () => {
    mockJsonResponse({ ...mutationFixture, section: "general" as const });

    await deleteSettingsMCPServer("github", { scope: "user", target: "auto" });

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/settings/mcp-servers/github?scope=user&target=auto",
    });
  });

  it("serializes the selected profile for MCP list, put, and delete", async () => {
    mockJsonResponse({ collection: "mcp-servers", scope: "profile", mcp_servers: [] });
    await listSettingsMCPServers({ scope: "profile", profile: " marketing " });
    await expectFetchRequest({
      path: "/api/settings/mcp-servers?scope=profile&profile=marketing",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse(mutationFixture);
    const body = { server: { name: "github", command: "gh" } };
    await putSettingsMCPServer("github", body, {
      scope: "profile",
      profile: " marketing ",
      target: "sidecar",
    });
    await expectFetchRequest({
      body,
      method: "PUT",
      path: "/api/settings/mcp-servers/github?scope=profile&profile=marketing&target=sidecar",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse(mutationFixture);
    await deleteSettingsMCPServer("github", {
      scope: "profile",
      profile: " marketing ",
      target: "auto",
    });
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/settings/mcp-servers/github?scope=profile&profile=marketing&target=auto",
    });
  });
});

describe("command palette settings", () => {
  it("serializes the selected profile for reads and writes", async () => {
    const section = {
      section: "cmd-palette" as const,
      scope: "profile" as const,
      profile: "marketing",
      aliases: {},
      fallback_agent_enabled: true,
      personalization: true,
      available_scopes: ["user", "profile", "workspace"] as const,
    };
    mockJsonResponse(section);
    await getSettingsCmdPalette({ scope: "profile", profile: " marketing " });
    await expectFetchRequest({
      path: "/api/settings/cmd-palette?scope=profile&profile=marketing",
    });

    vi.mocked(globalThis.fetch).mockClear();
    mockJsonResponse(section);
    const body = { aliases: {}, personalization: true };
    await updateSettingsCmdPalette(body, { scope: "profile", profile: " marketing " });
    await expectFetchRequest({
      body,
      method: "PATCH",
      path: "/api/settings/cmd-palette?scope=profile&profile=marketing",
    });
  });
});

describe("restart action", () => {
  it("triggers restart and returns operation info", async () => {
    mockJsonResponse(
      {
        operation_id: "op_001",
        status: "pending" as const,
        status_url: "/api/settings/actions/restart/op_001",
        active_session_count: 2,
      },
      { status: 202 }
    );

    const result = await triggerSettingsRestart();

    expect(result.operation_id).toBe("op_001");
    expect(result.status).toBe("pending");
    await expectFetchRequest({
      method: "POST",
      path: "/api/settings/actions/restart",
    });
  });

  it("gets restart status by operation id", async () => {
    mockJsonResponse({
      operation_id: "op_001",
      status: "ready" as const,
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 0,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:05Z",
    });

    const result = await getSettingsRestartStatus("op_001");
    expect(result.status).toBe("ready");
    await expectFetchRequest({
      path: "/api/settings/actions/restart/op_001",
    });
  });

  it("maps 404 to a typed not-found error", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));
    await expect(getSettingsRestartStatus("missing")).rejects.toThrow(
      "Restart operation not found: missing"
    );
  });
});

describe("apply records", () => {
  it("lists apply records with normalized filters", async () => {
    mockJsonResponse({
      entries: [
        {
          id: "cfg_apply_001",
          desired_config_hash: "sha256:desired",
          active_config_hash: "sha256:active",
          generation: 42,
          actor: "http",
          diff_class: "restart-required" as const,
          status: "blocked" as const,
          lifecycle: "restart-required" as const,
          next_action: "restart-daemon" as const,
          diagnostics: [],
          created_at: "2026-05-20T13:00:00Z",
          updated_at: "2026-05-20T13:00:01Z",
        },
      ],
    });

    const result = await listSettingsApplyRecords({
      status: "blocked",
      actor: " http ",
      limit: 5,
    });

    expect(result.entries[0]?.status).toBe("blocked");
    await expectFetchRequest({
      path: "/api/settings/apply?status=blocked&actor=http&limit=5",
    });
  });

  it("reloads settings and returns apply metadata", async () => {
    mockJsonResponse(mutationFixture);

    const result = await reloadSettings();

    expect(result.next_action).toBe("restart-daemon");
    expect(result.apply_record_id).toBe("cfg_apply_001");
    await expectFetchRequest({
      method: "POST",
      path: "/api/settings/reload",
    });
  });
});
