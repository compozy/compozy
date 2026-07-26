import type {
  ConfigApplyRecordsResponse,
  SettingsAutomationSection,
  SettingsSandboxCollection,
  SettingsSandboxEntry,
  SettingsApplyResponse,
  SettingsGeneralSection,
  SettingsHookCollection,
  SettingsHookEntry,
  SettingsHooksExtensionsSection,
  SettingsMCPServerCollection,
  SettingsMCPServerEntry,
  SettingsMemorySection,
  SettingsMutationResult,
  SettingsNetworkSection,
  SettingsNotificationPresetCollection,
  SettingsObservabilitySection,
  SettingsProviderCollection,
  SettingsProviderEntry,
  SettingsRestartResponse,
  SettingsRestartStatus,
  SettingsSkillsSection,
} from "@/systems/settings";
import { storyAgentNames, storyCompany, storyWorkspacePaths } from "@/storybook/fintech-scenario";

export const settingsGeneralSectionFixture: SettingsGeneralSection = {
  section: "general",
  scope: "global",
  available_scopes: ["global"],
  actions: {
    restart: { available: true, behavior: "action_trigger", name: "restart" },
  },
  config: {
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/agh.sock",
    },
    defaults: { agent: storyAgentNames.product, provider: "claude", sandbox: "local" },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" },
    redact: { enabled: true },
    session_timeout: "0s",
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.agh/config.toml",
    global_mcp_sidecar: "~/.agh/mcp.json",
    home_dir: "~/.agh",
    log_file: "~/.agh/agh.log",
  },
  runtime: {
    active_agents: 7,
    active_sessions: 4,
    available: true,
    http_host: "127.0.0.1",
    http_port: 2123,
    socket: "~/.agh/daemon.sock",
    total_sessions: 12,
    uptime_seconds: 3600,
  },
};

export const settingsNetworkSectionFixture: SettingsNetworkSection = {
  section: "network",
  scope: "global",
  available_scopes: ["global"],
  config: {
    enabled: true,
    max_replay_age: 300,
    live: {
      defaults: {
        max_wakes: 8,
        max_wake_wall_time: "5m",
        max_total_wall_time: "30m",
        max_input_tokens: 200_000,
        max_output_tokens: 50_000,
        max_wake_depth: 3,
        coalesce_window: "500ms",
      },
      limits: {
        max_wakes: 64,
        max_wake_wall_time: "15m",
        max_total_wall_time: "2h",
        max_input_tokens: 1_000_000,
        max_output_tokens: 200_000,
        max_wake_depth: 5,
        min_coalesce_window: "100ms",
        max_coalesce_window: "5s",
      },
    },
  },
  runtime: {
    available: true,
    enabled: true,
    status: "active",
    local_peers: 2,
    channels: 4,
    messages_received: 7,
    messages_delivered: 5,
    messages_rejected: 1,
  },
  links: [{ label: "network", path: "/network" }],
};

export const settingsNotificationPresetCollectionFixture: SettingsNotificationPresetCollection = {
  presets: [
    {
      name: "task_terminal",
      events: ["task.run_*"],
      targets: [
        {
          bridge_id: "bridge_slack_ops",
          canonical_route: "channel:ops",
          display_name: "#ops",
          delivery_mode: "direct-send",
        },
      ],
      filter: "outcome >= warning",
      enabled: true,
      built_in: true,
      default_version: "1",
      default_hash: "hash_task_terminal_v1",
      user_modified: true,
      default_update_available: false,
      created_at: "2026-04-17T09:00:00Z",
      updated_at: "2026-04-17T11:00:00Z",
    },
    {
      name: "session_unhealthy",
      events: ["session.unhealthy", "session.hung", "session.recovered"],
      targets: [],
      filter: "",
      enabled: false,
      built_in: true,
      default_version: "1",
      default_hash: "hash_session_unhealthy_v1",
      user_modified: false,
      default_update_available: false,
      created_at: "2026-04-17T09:00:00Z",
      updated_at: "2026-04-17T09:00:00Z",
    },
    {
      name: "provider_failure",
      events: [
        "provider.auth_required",
        "provider.rate_limited",
        "provider.permission_denied",
        "provider.unavailable",
      ],
      targets: [],
      filter: "",
      enabled: false,
      built_in: true,
      default_version: "1",
      default_hash: "hash_provider_failure_v1",
      user_modified: false,
      default_update_available: false,
      created_at: "2026-04-17T09:00:00Z",
      updated_at: "2026-04-17T09:00:00Z",
    },
  ],
  total: 3,
  generated_at: "2026-04-17T11:00:00Z",
};

export const settingsAutomationSectionFixture: SettingsAutomationSection = {
  section: "automation",
  scope: "global",
  available_scopes: ["global"],
  config: {
    enabled: true,
    timezone: "UTC",
    max_concurrent_jobs: 8,
    default_fire_limit: { max: 5, window: "1m" },
  },
  runtime: {
    available: true,
    running: true,
    scheduler_running: true,
    job_enabled: 3,
    job_total: 5,
    trigger_enabled: 2,
    trigger_total: 4,
    last_synced_at: "2026-04-17T10:00:00Z",
    next_fire: "2026-04-17T12:00:00Z",
  },
  links: [
    { label: "jobs", path: "/jobs" },
    { label: "triggers", path: "/triggers" },
  ],
};

export const settingsMemoryConfigFixture: SettingsMemorySection["config"] = {
  controller: {
    default_op_on_fail: "noop",
    max_latency: "300ms",
    mode: "hybrid",
    policy: {
      allow_origins: ["cli", "http", "uds", "tool", "extractor", "dreaming", "file", "provider"],
      max_content_chars: 4096,
      max_writes_per_min: 60,
    },
  },
  daily: {
    archive_path: "_system/archive",
    cold_archive_days: 30,
    dreaming_window: 7,
    hard_delete_days: 0,
    max_archive_bytes: 1073741824,
    max_bytes: 1048576,
    max_lines: 5000,
    rotate_format: "{date}.{seq}.md",
    sweep_hour: 3,
  },
  decisions: {
    keep_audit_summary: true,
    max_post_content_bytes: 65536,
    prune_after_applied_days: 90,
  },
  dream: {
    check_interval: "30m",
    debounce: "10m",
    gates: {
      min_recall_count: 2,
      min_score: 0.75,
      min_unpromoted: 5,
    },
    min_hours: 24,
    min_sessions: 3,
    prompt_version: "v1",
    scoring: {
      recency_half_life_days: 14,
      weights: {
        frequency: 0.3,
        freshness: 0.15,
        recency: 0.2,
        relevance: 0.35,
      },
    },
  },
  enabled: true,
  extractor: {
    deadline: "60s",
    dlq_path: "~/.agh/memory/_system/extractor/failures",
    inbox_path: "~/.agh/memory/_inbox",
    mode: "post_message",
    queue: {
      capacity: 1,
      coalesce_max: 16,
    },
    sandbox_inbox_only: true,
    throttle_turns: 1,
  },
  file: {
    max_bytes: 25600,
    max_lines: 200,
  },
  global_dir: "~/.agh/memory",
  provider: {
    cooldown: "30s",
    failure_threshold: 5,
    name: "",
    timeout: "2s",
  },
  recall: {
    freshness: {
      banner_after_days: 1,
    },
    fusion: "weighted",
    include_already_surfaced: false,
    include_system: false,
    raw_candidates: 50,
    signals: {
      metrics_enabled: true,
      queue_capacity: 256,
      worker_retry_max: 3,
    },
    top_k: 5,
    weights: {
      bm25_trigram: 0.2,
      bm25_unicode: 0.55,
      recall_signal: 0.1,
      recency: 0.15,
    },
  },
  session: {
    cold_archive_days: 30,
    events_purge_grace: "24h",
    hard_delete_days: 0,
    ledger_format: "jsonl",
    ledger_root: "~/.agh/sessions",
    max_archive_bytes: 10737418240,
    unbound_partition: "_unbound",
  },
  workspace: {
    auto_create: true,
    toml_path: "<workspace>/.agh/workspace.toml",
  },
};

export const settingsMemorySectionFixture: SettingsMemorySection = {
  section: "memory",
  scope: "global",
  available_scopes: ["global"],
  actions: {
    consolidate: {
      available: true,
      behavior: "action_trigger",
      name: "consolidate",
    },
  },
  config: settingsMemoryConfigFixture,
  health: {
    available: true,
    dream_enabled: true,
    file_count: 42,
    last_consolidated_at: "2026-04-17T14:00:00Z",
  },
};

export const settingsObservabilitySectionFixture: SettingsObservabilitySection = {
  section: "observability",
  scope: "global",
  available_scopes: ["global"],
  config: {
    enabled: true,
    max_global_bytes: 1024 * 1024 * 1024,
    retention_days: 7,
    transcripts: {
      enabled: true,
      max_bytes_per_session: 256 * 1024 * 1024,
      segment_bytes: 1024 * 1024,
    },
  },
  log_tail: {
    available: true,
    stream_url: "/api/settings/observability/log-tail",
    transport: "sse",
  },
  runtime: {
    active_agents: 2,
    active_sessions: 4,
    available: true,
    global_db_size_bytes: 180 * 1024 * 1024,
    session_db_size_bytes: 132 * 1024 * 1024,
    uptime_seconds: 3600,
  },
};

export const settingsSkillsSectionFixture: SettingsSkillsSection = {
  section: "skills",
  scope: "global",
  available_scopes: ["global"],
  runtime_available: true,
  discovered_count: 12,
  disabled_count: 2,
  config: {
    enabled: true,
    disabled_skills: ["alpha", "beta"],
    poll_interval: "5m",
    marketplace: {
      registry: "agh",
      base_url: storyCompany.registryBaseUrl,
    },
    allowed_marketplace_mcp: ["merchant-docs"],
    allowed_marketplace_hooks: [],
  },
  links: [{ label: "skills", path: "/marketplace/skills?tab=installed" }],
};

export const settingsHooksExtensionsSectionFixture: SettingsHooksExtensionsSection = {
  section: "hooks-extensions",
  scope: "global",
  available_scopes: ["global"],
  config: {
    marketplace: {
      registry: "northstar",
      base_url: storyCompany.hooksMarketplaceBaseUrl,
      allow_unverified: false,
    },
    resources: {
      allowed_kinds: ["snapshot", "artifact"],
      max_scope: "workspace",
      snapshot_rate_limit: { queue: 100, requests: 30, window: "5m" },
      operator_write_rate_limit: { queue: 20, requests: 10, window: "1m" },
    },
  },
  hooks: [
    {
      name: "pre-commit-lint",
      declaration: {
        name: "pre-commit-lint",
        event: "tool.pre_call",
        mode: "sync",
        command: "make",
        args: ["lint"],
        matcher: { tool_name: "Bash" },
        required: true,
      },
      source_metadata: {
        available_targets: ["global-config"],
        effective_source: { kind: "global-config", scope: "global" },
      },
    },
    {
      name: "slack-notify",
      declaration: {
        name: "slack-notify",
        event: "permission.denied",
        mode: "async",
        command: "node",
        args: ["./hooks/slack.js"],
        matcher: { agent_name: storyAgentNames.support },
        required: false,
      },
      source_metadata: {
        available_targets: ["global-config"],
        effective_source: { kind: "global-config", scope: "global" },
      },
    },
  ],
  installed: [
    {
      name: "daytona",
      enabled: true,
      version: "1.2.3",
      state: "running",
      health: "healthy",
      requires_env: ["DAYTONA_TOKEN"],
      missing_env: ["DAYTONA_TOKEN"],
    },
  ],
  transport_parity: {
    known: true,
    settings_http: true,
    settings_uds: true,
    extensions_http: true,
    extensions_uds: true,
  },
};

export const settingsProviderFixtures: SettingsProviderEntry[] = [
  {
    name: "claude",
    default: true,
    command_available: true,
    settings: {
      command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
      display_name: "Claude Code",
      models: {
        default: "claude-sonnet-5",
        curated: [
          {
            id: "claude-fable-5",
            supports_reasoning: true,
            reasoning_efforts: ["low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "high",
          },
          {
            id: "claude-opus-4-8",
            supports_reasoning: true,
            reasoning_efforts: ["low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "high",
          },
          {
            id: "claude-sonnet-5",
            supports_reasoning: true,
            reasoning_efforts: ["low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "high",
          },
          { id: "claude-haiku-4-5-20251001", supports_reasoning: true },
        ],
        reasoning: { apply: "acp_option" },
      },
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      auth_status_command: "claude auth status",
      auth_login_command: "claude login",
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      code: "provider_classification_unknown",
      message:
        "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
      status_command: "claude auth status",
      login_command: "claude login",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "global" },
      shadowed_sources: [{ kind: "builtin-provider", scope: "global" }],
    },
    fallback: {
      settings: {
        command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
        models: {
          default: "claude-sonnet-5",
          curated: [
            { id: "claude-fable-5" },
            { id: "claude-opus-4-8" },
            { id: "claude-sonnet-5" },
            { id: "claude-haiku-4-5-20251001" },
          ],
          reasoning: { apply: "acp_option" },
        },
        auth_mode: "native_cli",
        env_policy: "filtered",
        home_policy: "operator",
      },
      source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "codex",
    default: false,
    command_available: true,
    settings: {
      command: "npx -y @agentclientprotocol/codex-acp@latest",
      models: {
        default: "gpt-5.6-sol",
        curated: [
          {
            id: "gpt-5.6-sol",
            supports_reasoning: true,
            reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "medium",
          },
          {
            id: "gpt-5.6-terra",
            supports_reasoning: true,
            reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "medium",
          },
          {
            id: "gpt-5.6-luna",
            supports_reasoning: true,
            reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
            default_reasoning_effort: "medium",
          },
        ],
        reasoning: { apply: "acp_option" },
      },
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      auth_status_command: "codex auth status",
      auth_login_command: "codex login",
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      code: "provider_classification_unknown",
      message:
        "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
      status_command: "codex auth status",
      login_command: "codex login",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "openrouter",
    default: false,
    command_available: true,
    settings: {
      command: "npx -y pi-acp@latest",
      display_name: "OpenRouter",
      models: {
        default: "openai/gpt-5.4",
        curated: [
          { id: "openai/gpt-5.4", supports_reasoning: true },
          { id: "anthropic/claude-sonnet-4-6" },
        ],
      },
      harness: "pi_acp",
      runtime_provider: "openrouter",
      auth_mode: "bound_secret",
      env_policy: "filtered",
      home_policy: "operator",
      credential_slots: [
        {
          name: "api_key",
          target_env: "OPENROUTER_API_KEY",
          secret_ref: "env:OPENROUTER_API_KEY",
          kind: "api_key",
          required: true,
        },
      ],
    },
    auth_status: {
      mode: "bound_secret",
      env_policy: "filtered",
      home_policy: "operator",
      state: "missing_credential",
      code: "provider_credential_unresolved",
      message: 'Required AGH-managed provider credential "OPENROUTER_API_KEY" is unresolved.',
    },
    credentials: [
      {
        name: "api_key",
        target_env: "OPENROUTER_API_KEY",
        secret_ref: "env:OPENROUTER_API_KEY",
        kind: "api_key",
        required: true,
        present: false,
        source: "env",
      },
    ],
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "blackbox",
    default: false,
    command_available: true,
    settings: {
      command: "blackbox --experimental-acp",
      display_name: "BLACKBOX AI",
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      code: "provider_classification_unknown",
      message:
        "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "cline",
    default: false,
    command_available: true,
    settings: { command: "npx -y cline@latest --acp", display_name: "Cline", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "goose",
    default: false,
    command_available: true,
    settings: { command: "goose acp", display_name: "Goose", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "hermes",
    default: false,
    command_available: true,
    settings: { command: "hermes acp", display_name: "Hermes", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "junie",
    default: false,
    command_available: true,
    settings: {
      command: "junie --acp true",
      display_name: "Junie",
      harness: "acp",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "kimi-cli",
    default: false,
    command_available: true,
    settings: {
      command: "kimi acp",
      display_name: "Kimi CLI",
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      code: "provider_classification_unknown",
      message:
        "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "openclaw",
    default: false,
    command_available: true,
    settings: { command: "openclaw acp", display_name: "OpenClaw", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "openhands",
    default: false,
    command_available: true,
    settings: { command: "openhands acp", display_name: "OpenHands", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "qoder",
    default: false,
    command_available: true,
    settings: {
      command: "npx -y @qoder-ai/qodercli@latest --acp",
      display_name: "Qoder CLI",
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      code: "provider_classification_unknown",
      message:
        "Provider owns authentication through its native CLI; run a provider auth probe for live status.",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
  {
    name: "qwen-code",
    default: false,
    command_available: true,
    settings: {
      command: "npx -y @qwen-code/qwen-code@latest --acp --experimental-skills",
      display_name: "Qwen Code",
      models: { default: "qwen3.6-plus", curated: [{ id: "qwen3.6-plus" }] },
      harness: "acp",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
];

export const settingsSandboxFixtures: SettingsSandboxEntry[] = [
  {
    name: "local",
    workspace_usage_count: 3,
    profile: {
      backend: "local",
      sync_mode: "none",
      persistence: "transient",
      runtime_root: "~",
      network: {
        allow_outbound: true,
        allow_public_ingress: false,
        allow_list: [],
        deny_list: [],
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "global" },
    },
  },
  {
    name: "daytona-eu",
    workspace_usage_count: 2,
    profile: {
      backend: "daytona",
      sync_mode: "session-bidir",
      persistence: "reuse",
      runtime_root: "/workspace",
      daytona: {
        image: "agh/daytona:latest",
        target: "eu-central",
        auto_stop: "30",
        auto_archive: "120",
      },
      network: {
        allow_outbound: true,
        allow_public_ingress: false,
        allow_list: ["api.github.com", "registry.npmjs.org"],
        deny_list: [],
      },
      env: { NODE_ENV: "production" },
      secret_env: { DAYTONA_API_KEY: "vault:providers/daytona/api-key" },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "global" },
    },
  },
  {
    name: "builtin-local",
    workspace_usage_count: 0,
    profile: {
      backend: "local",
      sync_mode: "none",
      persistence: "transient",
      runtime_root: "~",
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "global" },
    },
  },
];

export const settingsMCPServerFixtures: SettingsMCPServerEntry[] = [
  {
    name: "filesystem",
    transport: "stdio",
    command: "npx -y @modelcontextprotocol/server-filesystem",
    args: [storyWorkspacePaths.risk],
    scope: "global",
    source_metadata: {
      available_targets: ["global-mcp-sidecar", "global-config"],
      effective_source: { kind: "global-mcp-sidecar", scope: "global" },
      shadowed_sources: [{ kind: "global-config", scope: "global" }],
    },
  },
  {
    name: "github",
    transport: "stdio",
    command: "npx -y @modelcontextprotocol/server-github",
    env_keys: ["GITHUB_TOKEN"],
    scope: "global",
    source_metadata: {
      available_targets: ["global-mcp-sidecar"],
      effective_source: { kind: "global-mcp-sidecar", scope: "global" },
    },
  },
];

export const settingsHookFixtures: SettingsHookEntry[] =
  settingsHooksExtensionsSectionFixture.hooks ?? [];

export const settingsProvidersCollectionFixture = {
  available_scopes: ["global"],
  collection: "providers",
  providers: settingsProviderFixtures,
  scope: "global",
} satisfies SettingsProviderCollection;

export const settingsSandboxesCollectionFixture = {
  available_scopes: ["global"],
  collection: "sandboxes",
  sandboxes: settingsSandboxFixtures,
  scope: "global",
} satisfies SettingsSandboxCollection;

export const settingsHooksCollectionFixture = {
  available_scopes: ["global"],
  collection: "hooks",
  hooks: settingsHookFixtures,
  scope: "global",
} satisfies SettingsHookCollection;

export const settingsMCPServersCollectionFixture = {
  available_scopes: ["global", "workspace"],
  collection: "mcp-servers",
  mcp_servers: settingsMCPServerFixtures,
  scope: "global",
} satisfies SettingsMCPServerCollection;

// The reference visual-contract matrix: nine servers exercising every auth x
// runtime x probe cell (docs/design/opendesign/mcp-management.html SERVERS).
function mcpConfigSource(kind: string, scope: "global" | "workspace") {
  return {
    available_targets: [kind] as SettingsMCPServerEntry["source_metadata"]["available_targets"],
    effective_source:
      scope === "workspace" ? { kind, scope, workspace_id: "ws-platform" } : { kind, scope },
  } as SettingsMCPServerEntry["source_metadata"];
}

export const mcpManagementServerFixtures: SettingsMCPServerEntry[] = [
  {
    name: "github-local",
    transport: "stdio",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-github"],
    env_keys: ["LOG_LEVEL"],
    secret_env_keys: ["GITHUB_PERSONAL_ACCESS_TOKEN"],
    scope: "workspace",
    workspace_id: "ws-platform",
    runtime_status: {
      configured: true,
      initialized: true,
      state: "ready",
      probe: "succeeded",
      tool_count: 12,
    },
    source_metadata: mcpConfigSource("workspace-mcp-sidecar", "workspace"),
  },
  {
    name: "linear",
    transport: "http",
    url: "https://mcp.linear.app/mcp",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-linear-public",
      client_secret_configured: true,
      issuer_url: "https://auth.linear.app",
    },
    auth_status: {
      server_name: "linear",
      scope: "workspace",
      status: "needs_login",
      token_present: false,
      refreshable: true,
      diagnostic: "token absent",
    },
    runtime_status: {
      configured: true,
      initialized: false,
      state: "auth_required",
      probe: "skipped",
      tool_count: 0,
    },
    catalog_entry: "linear",
    catalog_version: "1.4.0",
    scope: "workspace",
    workspace_id: "ws-platform",
    source_metadata: mcpConfigSource("workspace-config", "workspace"),
  },
  {
    name: "sentry",
    transport: "sse",
    url: "https://mcp.sentry.dev/sse",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-sentry-public",
      client_secret_configured: false,
      issuer_url: "https://auth.sentry.io",
    },
    auth_status: {
      server_name: "sentry",
      scope: "workspace",
      status: "authenticated",
      token_present: true,
      refreshable: true,
      diagnostic: "token present",
    },
    runtime_status: {
      configured: true,
      initialized: true,
      state: "ready",
      probe: "succeeded",
      tool_count: 18,
    },
    catalog_entry: "sentry",
    catalog_version: "2.1.0",
    scope: "workspace",
    workspace_id: "ws-platform",
    source_metadata: mcpConfigSource("workspace-config", "workspace"),
  },
  {
    name: "notion",
    transport: "http",
    url: "https://mcp.notion.com/mcp",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-notion-public",
      client_secret_configured: false,
      issuer_url: "https://auth.notion.com",
    },
    auth_status: {
      server_name: "notion",
      scope: "workspace",
      status: "expired",
      token_present: true,
      refreshable: true,
      diagnostic: "token expired",
    },
    runtime_status: {
      configured: true,
      initialized: false,
      state: "auth_expired",
      probe: "skipped",
      tool_count: 0,
    },
    scope: "workspace",
    workspace_id: "ws-platform",
    source_metadata: mcpConfigSource("workspace-config", "workspace"),
  },
  {
    name: "figma",
    transport: "sse",
    url: "https://mcp.figma.com/sse",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-figma-public",
      client_secret_configured: false,
      issuer_url: "https://auth.figma.com",
    },
    auth_status: {
      server_name: "figma",
      scope: "global",
      status: "invalid",
      token_present: true,
      refreshable: false,
      diagnostic: "token rejected",
    },
    runtime_status: {
      configured: true,
      initialized: false,
      state: "auth_invalid",
      probe: "skipped",
      tool_count: 0,
    },
    scope: "global",
    source_metadata: mcpConfigSource("global-config", "global"),
  },
  {
    name: "github-remote",
    transport: "http",
    url: "https://api.githubcopilot.com/mcp",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-github-public",
      client_secret_configured: false,
      issuer_url: "https://github.com/login/oauth",
    },
    auth_status: {
      server_name: "github-remote",
      scope: "global",
      status: "authenticated",
      token_present: true,
      refreshable: true,
      diagnostic: "prior token retained",
    },
    runtime_status: {
      configured: true,
      initialized: false,
      state: "auth_refresh_failed",
      probe: "skipped",
      tool_count: 0,
    },
    scope: "global",
    source_metadata: mcpConfigSource("global-config", "global"),
  },
  {
    name: "filesystem",
    transport: "stdio",
    command: "node",
    args: ["./bin/filesystem-mcp.js"],
    scope: "workspace",
    workspace_id: "ws-platform",
    runtime_status: {
      configured: true,
      initialized: false,
      state: "config_error",
      probe: "skipped",
      tool_count: 0,
    },
    source_metadata: mcpConfigSource("workspace-config", "workspace"),
  },
  {
    name: "pagerduty",
    transport: "http",
    url: "https://mcp.pagerduty.com/mcp",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-pagerduty-public",
      client_secret_configured: false,
      issuer_url: "https://auth.pagerduty.com",
    },
    auth_status: {
      server_name: "pagerduty",
      scope: "workspace",
      status: "authenticated",
      token_present: true,
      refreshable: true,
      diagnostic: "token present",
    },
    runtime_status: {
      configured: true,
      initialized: true,
      state: "permission_denied",
      probe: "failed",
      tool_count: 0,
    },
    scope: "workspace",
    workspace_id: "ws-platform",
    source_metadata: mcpConfigSource("workspace-config", "workspace"),
  },
  {
    name: "buildkite",
    transport: "sse",
    url: "https://mcp.buildkite.com/sse",
    auth: {
      type: "oauth2_pkce",
      client_id: "agh-buildkite-public",
      client_secret_configured: false,
      issuer_url: "https://auth.buildkite.com",
    },
    auth_status: {
      server_name: "buildkite",
      scope: "global",
      status: "authenticated",
      token_present: true,
      refreshable: true,
      diagnostic: "token present",
    },
    runtime_status: {
      configured: true,
      initialized: true,
      state: "runtime_unavailable",
      probe: "failed",
      tool_count: 0,
    },
    scope: "global",
    source_metadata: mcpConfigSource("global-config", "global"),
  },
];

export const mcpManagementCollectionFixture = {
  available_scopes: ["global", "workspace"],
  collection: "mcp-servers",
  mcp_servers: mcpManagementServerFixtures,
  scope: "workspace",
  workspace_id: "ws-platform",
} satisfies SettingsMCPServerCollection;

export const mcpAuthBeginFixture = {
  authorization_url:
    "https://auth.linear.app/oauth/authorize?client_id=agh-linear-public&code_challenge=J6lRkq8l9lZ3tZpQf7uYx2AqY7M3xv2b9R6n0ZsVn4A&code_challenge_method=S256&redirect_uri=http%3A%2F%2F127.0.0.1%3A2123%2Fapi%2Fmcp%2Foauth%2Fcallback&state=agh_mcp_7m3p9q",
  callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
  expires_at: new Date(Date.now() + 5 * 60_000).toISOString(),
  manual_supported: true,
  state: "agh_mcp_7m3p9q",
};

export const mcpAuthStatusAuthenticatedFixture = {
  server_name: "linear",
  scope: "workspace",
  workspace_id: "ws-platform",
  status: "authenticated",
  token_present: true,
  refreshable: true,
  diagnostic: "token present",
};

export const settingsRestartResponseFixture: SettingsRestartResponse = {
  operation_id: "restart_northstar_pay",
  status: "pending",
  active_session_count: 2,
  status_url: "/api/settings/actions/restart/restart_northstar_pay",
};

export const settingsRestartStatusFixture: SettingsRestartStatus = {
  operation_id: "restart_northstar_pay",
  status: "ready",
  active_session_count: 0,
  old_pid: 1000,
  old_socket_path: "/tmp/agh.sock",
  old_started_at: "2026-04-17T10:00:00Z",
  started_at: "2026-04-17T10:05:00Z",
  updated_at: "2026-04-17T10:05:05Z",
};

export const settingsAppliedMutationFixture: SettingsMutationResult = {
  active_config_hash: "sha256:active-live",
  active_generation: 42,
  applied: true,
  apply_record_id: "cfg_apply_live",
  section: "general",
  scope: "global",
  lifecycle: "live",
  next_action: "none",
  restart_required: false,
  restart_scope: "none",
  warnings: [],
  write_target: "global-config",
};

export const settingsRestartRequiredMutationFixture: SettingsMutationResult = {
  active_config_hash: "sha256:active-blocked",
  active_generation: 41,
  applied: true,
  apply_record_id: "cfg_apply_blocked",
  section: "general",
  scope: "global",
  lifecycle: "restart-required",
  next_action: "restart-daemon",
  restart_required: true,
  restart_scope: "daemon",
  warnings: [],
  write_target: "global-config",
};

export const settingsReloadAppliedFixture: SettingsApplyResponse = {
  active_config_hash: "sha256:active-live",
  active_generation: 42,
  applied: true,
  apply_record_id: "cfg_apply_live",
  lifecycle: "live",
  next_action: "none",
  restart_required: false,
  warnings: [],
};

export const settingsReloadBlockedFixture: SettingsApplyResponse = {
  active_config_hash: "sha256:active-blocked",
  active_generation: 41,
  applied: false,
  apply_record_id: "cfg_apply_blocked",
  lifecycle: "restart-required",
  next_action: "restart-daemon",
  restart_required: true,
  restart_scope: "daemon",
  warnings: ["restart the daemon to activate config.toml"],
};

export const settingsApplyRecordsFixture: ConfigApplyRecordsResponse = {
  entries: [
    {
      id: "cfg_apply_blocked",
      desired_config_hash: "sha256:desired-restart-required",
      active_config_hash: "sha256:active-blocked",
      generation: 41,
      actor: "http",
      diff_class: "restart-required",
      status: "blocked",
      lifecycle: "restart-required",
      next_action: "restart-daemon",
      diagnostics: [
        {
          id: "diag_restart_required",
          code: "config.apply.restart_required",
          title: "Restart required",
          message: "daemon.socket changes require a daemon restart.",
          severity: "warning",
          category: "configuration",
          data_freshness: "current",
          suggested_command: "agh config reload",
        },
      ],
      created_at: "2026-05-20T13:10:00Z",
      updated_at: "2026-05-20T13:10:03Z",
    },
    {
      id: "cfg_apply_live",
      desired_config_hash: "sha256:desired-live",
      active_config_hash: "sha256:active-live",
      generation: 42,
      actor: "web",
      diff_class: "live",
      status: "applied",
      lifecycle: "live",
      next_action: "none",
      created_at: "2026-05-20T13:18:00Z",
      applied_at: "2026-05-20T13:18:01Z",
      updated_at: "2026-05-20T13:18:01Z",
    },
    {
      id: "cfg_apply_failed",
      desired_config_hash: "sha256:desired-failed",
      active_config_hash: "sha256:active-live",
      generation: 42,
      actor: "autonomy",
      diff_class: "session-rebind",
      status: "failed",
      lifecycle: "session-rebind",
      next_action: "retry",
      diagnostics: [
        {
          id: "diag_invalid_config",
          code: "config.invalid",
          title: "Invalid config",
          message: "provider timeout must be a positive duration.",
          severity: "error",
          category: "configuration",
          data_freshness: "current",
          suggested_command: "agh config validate",
        },
      ],
      created_at: "2026-05-20T13:24:00Z",
      updated_at: "2026-05-20T13:24:02Z",
    },
  ],
};
