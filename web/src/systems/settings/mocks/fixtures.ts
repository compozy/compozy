import type {
  ConfigApplyRecordsResponse,
  SettingsAutomationSection,
  SettingsAttentionSection,
  SettingsCmdPaletteSection,
  SettingsSandboxCollection,
  SettingsSandboxEntry,
  SettingsApplyResponse,
  SettingsGeneralSection,
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
  SettingsSkillSource,
  SettingsSkillSourceRoot,
  SettingsSkillsSection,
  TerminalSettingsConfig,
} from "@/systems/settings";
import { storyAgentNames, storyCompany, storyWorkspacePaths } from "@/storybook/fintech-scenario";

export const terminalSettingsFixture: TerminalSettingsConfig = {
  default_shell: "",
  shell_integration: true,
  scrollback_bytes: 1_048_576,
  detached_ttl: "24h",
  exit_retention: "15m",
  recording: false,
  recording_retention_days: 30,
  max_per_workspace: 8,
  max_per_daemon: 32,
  max_subscribers: 16,
};

export const settingsGeneralSectionFixture: SettingsGeneralSection = {
  section: "general",
  scope: "user",
  available_scopes: ["user"],
  actions: {
    restart: { available: true, behavior: "action_trigger", name: "restart" },
  },
  config: {
    busy_input: { default_mode: "steer" },
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/compozy.sock",
    },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" },
    redact: { enabled: true },
    session_timeout: "0s",
    terminal: terminalSettingsFixture,
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.compozy/config.toml",
    global_mcp_sidecar: "~/.compozy/mcp.json",
    home_dir: "~/.compozy",
    log_file: "~/.compozy/compozy.log",
  },
  runtime: {
    active_agents: 7,
    active_sessions: 4,
    available: true,
    http_host: "127.0.0.1",
    http_port: 2123,
    socket: "~/.compozy/daemon.sock",
    total_sessions: 12,
    uptime_seconds: 3600,
  },
};

export const settingsNetworkSectionFixture: SettingsNetworkSection = {
  section: "network",
  scope: "user",
  available_scopes: ["user"],
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

export const settingsAttentionSectionFixture: SettingsAttentionSection = {
  section: "attention",
  scope: "user",
  available_scopes: ["user"],
  config: {
    toasts: true,
    sound: true,
    system: false,
    muted_workspaces: [],
  },
};

export const settingsCmdPaletteSectionFixture: SettingsCmdPaletteSection = {
  section: "cmd-palette",
  scope: "user",
  available_scopes: ["user", "profile", "workspace"],
  aliases: {},
  fallback_agent_enabled: true,
  personalization: true,
};

export const settingsNotificationPresetCollectionFixture: SettingsNotificationPresetCollection = {
  presets: [
    {
      profile: "default",
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
      profile: "default",
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
      profile: "default",
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
  scope: "user",
  available_scopes: ["user"],
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
    dlq_path: "~/.compozy/memory/_system/extractor/failures",
    inbox_path: "~/.compozy/memory/_inbox",
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
  global_dir: "~/.compozy/memory",
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
    ledger_root: "~/.compozy/sessions",
    max_archive_bytes: 10737418240,
    unbound_partition: "_unbound",
  },
  workspace: {
    auto_create: true,
    toml_path: "<workspace>/.compozy/workspace.toml",
  },
};

export const settingsMemorySectionFixture: SettingsMemorySection = {
  section: "memory",
  scope: "user",
  available_scopes: ["user"],
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
  scope: "user",
  available_scopes: ["user"],
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

const emptyRootDiagnostics: Pick<
  SettingsSkillSourceRoot,
  "collisions" | "skipped_links" | "verification"
> = {
  collisions: [],
  skipped_links: [],
  verification: { blocked: 0, warned: 0 },
};

/**
 * Default daemon-measured sources. Tests derive unreadable, truncated, and
 * runtime-unavailable variants from these measured rows.
 */
export const settingsSkillSourcesFixture: SettingsSkillSource[] = [
  {
    slug: "compozy",
    label: "CompozyOS",
    kind: "builtin",
    enabled: true,
    always_on: true,
    workspace_path: ".compozy/skills",
    global_path: "~/.compozy/skills",
    roots: [
      {
        ...emptyRootDiagnostics,
        root_id: "r_w_builtin_04c9",
        path: `${storyWorkspacePaths.hq}/.compozy/skills`,
        exists: true,
        readable: true,
        scanned_count: 8,
        skill_count: 8,
        truncated: false,
        native_readers: [],
      },
      {
        ...emptyRootDiagnostics,
        root_id: "r_u_builtin_11ab",
        path: "/Users/ana/.compozy/skills",
        exists: true,
        readable: true,
        scanned_count: 4,
        skill_count: 4,
        truncated: false,
        native_readers: [],
      },
    ],
  },
  {
    slug: "agents",
    label: "Universal (.agents)",
    kind: "preset",
    enabled: true,
    always_on: false,
    default: true,
    workspace_path: ".agents/skills",
    global_path: "~/.agents/skills",
    roots: [
      {
        root_id: "r_w_agents_9f31",
        path: `${storyWorkspacePaths.hq}/.agents/skills`,
        exists: true,
        readable: true,
        scanned_count: 5,
        skill_count: 3,
        truncated: false,
        native_readers: ["openclaw", "hermes"],
        skipped_links: [
          { path: `${storyWorkspacePaths.hq}/.agents/skills/review-old`, reason: "dangling" },
          { path: `${storyWorkspacePaths.hq}/.agents/skills/vendor-link`, reason: "escape" },
        ],
        collisions: [
          {
            name: "frontend-qa",
            winner_root_id: "r_w_builtin_04c9",
            qualified_form: "agents:frontend-qa",
          },
        ],
        verification: { blocked: 0, warned: 1 },
      },
      {
        ...emptyRootDiagnostics,
        root_id: "r_u_agents_7c02",
        path: "/Users/ana/.agents/skills",
        exists: true,
        readable: true,
        scanned_count: 2,
        skill_count: 2,
        truncated: false,
        native_readers: ["openclaw"],
      },
    ],
  },
  {
    slug: "claude",
    label: "Claude (.claude)",
    kind: "preset",
    enabled: false,
    always_on: false,
    default: false,
    workspace_path: ".claude/skills",
    global_path: "~/.claude/skills",
    roots: [
      {
        ...emptyRootDiagnostics,
        root_id: "r_w_claude_3ad1",
        path: `${storyWorkspacePaths.hq}/.claude/skills`,
        exists: false,
        readable: true,
        truncated: false,
        native_readers: ["claude"],
      },
      {
        ...emptyRootDiagnostics,
        root_id: "r_u_claude_8be4",
        path: "/Users/ana/.claude/skills",
        exists: true,
        readable: true,
        scanned_count: 1,
        skill_count: 1,
        truncated: false,
        native_readers: ["claude"],
      },
    ],
  },
  {
    slug: "team-skills",
    label: "team-skills",
    kind: "custom",
    enabled: true,
    always_on: false,
    path: "~/team-skills",
    roots: [
      {
        ...emptyRootDiagnostics,
        root_id: "r_u_custom_a41c",
        path: "/Users/ana/team-skills",
        exists: true,
        readable: true,
        scanned_count: 3,
        skill_count: 3,
        truncated: false,
        native_readers: [],
      },
    ],
  },
];

export const settingsSkillsSectionFixture: SettingsSkillsSection = {
  section: "skills",
  scope: "user",
  available_scopes: ["user", "profile", "workspace", "agent"],
  runtime_available: true,
  discovered_count: 12,
  disabled_count: 2,
  config: {
    enabled: true,
    disabled_skills: ["alpha", "beta"],
    poll_interval: "5m",
    marketplace: {
      registry: "compozy",
      base_url: storyCompany.registryBaseUrl,
    },
    allowed_marketplace_mcp: ["merchant-docs"],
    allowed_marketplace_hooks: [],
    sources: ["agents"],
    custom_sources: ["~/team-skills"],
  },
  sources: settingsSkillSourcesFixture,
  links: [{ label: "skills", path: "/marketplace/skills" }],
};

export const settingsHooksExtensionsSectionFixture: SettingsHooksExtensionsSection = {
  section: "hooks-extensions",
  scope: "user",
  available_scopes: ["user"],
  config: {
    trust: {
      allow_unverified: false,
    },
    sources: {
      github: { enabled: true, base_url: "https://api.github.com" },
      git: { enabled: false },
    },
    dev: {
      watch_interval: "2s",
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
        effective_source: { kind: "global-config", scope: "user" },
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
        effective_source: { kind: "global-config", scope: "user" },
      },
    },
  ],
  installed: [
    {
      name: "notes",
      enabled: true,
      version: "0.1.0",
      state: "running",
      health: "healthy",
      palette: {
        commands: [
          {
            id: "ext.notes.capture",
            title: "Capture note",
            bindings: ["alt+shift+KeyN"],
            default_binding: "alt+shift+KeyN",
            default_dormant: false,
            available: true,
          },
          {
            id: "ext.notes.recent",
            title: "Recent notes",
            bindings: [],
            default_binding: "meta+KeyN",
            default_dormant: true,
            conflict_with: "session.new",
            available: true,
          },
          {
            id: "ext.notes.purge",
            title: "Purge archived notes",
            bindings: [],
            default_dormant: false,
            available: true,
          },
        ],
        views: [
          { id: "ext.notes.recent", title: "Recent notes", available: true },
          { id: "ext.notes.browse", title: "Browse notes", available: true },
        ],
      },
    },
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
      login: {
        configured: true,
        executable: "claude",
        presence: "unknown",
        recommended_action: "inspect",
        source: "auth_login_command",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "user" },
      shadowed_sources: [{ kind: "builtin-provider", scope: "user" }],
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
      source: { kind: "builtin-provider", scope: "user" },
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
      login: {
        configured: true,
        executable: "codex",
        presence: "unknown",
        recommended_action: "inspect",
        source: "auth_login_command",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      message: 'Required CompozyOS-managed provider credential "OPENROUTER_API_KEY" is unresolved.',
      login: {
        configured: false,
        presence: "unknown",
        recommended_action: "bind_secret",
      },
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
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      login: {
        configured: false,
        presence: "unknown",
        recommended_action: "inspect",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
  {
    name: "cline",
    default: false,
    command_available: true,
    settings: { command: "npx -y cline@latest --acp", display_name: "Cline", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
  {
    name: "goose",
    default: false,
    command_available: true,
    settings: { command: "goose acp", display_name: "Goose", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
  {
    name: "hermes",
    default: false,
    command_available: true,
    settings: { command: "hermes acp", display_name: "Hermes", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      login: {
        configured: false,
        presence: "unknown",
        recommended_action: "inspect",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
  {
    name: "openclaw",
    default: false,
    command_available: true,
    settings: { command: "openclaw acp", display_name: "OpenClaw", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
  {
    name: "openhands",
    default: false,
    command_available: true,
    settings: { command: "openhands acp", display_name: "OpenHands", harness: "acp" },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      login: {
        configured: false,
        presence: "unknown",
        recommended_action: "inspect",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      effective_source: { kind: "builtin-provider", scope: "user" },
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
      effective_source: { kind: "global-config", scope: "user" },
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
        image: "compozy/daytona:latest",
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
      effective_source: { kind: "global-config", scope: "user" },
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
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  },
];

export const settingsMCPServerFixtures: SettingsMCPServerEntry[] = [
  {
    name: "filesystem",
    transport: "stdio",
    command: "npx -y @modelcontextprotocol/server-filesystem",
    args: [storyWorkspacePaths.risk],
    scope: "user",
    source_metadata: {
      available_targets: ["global-mcp-sidecar", "global-config"],
      effective_source: { kind: "global-mcp-sidecar", scope: "user" },
      shadowed_sources: [{ kind: "global-config", scope: "user" }],
    },
  },
  {
    name: "github",
    transport: "stdio",
    command: "npx -y @modelcontextprotocol/server-github",
    env_keys: ["GITHUB_TOKEN"],
    scope: "user",
    source_metadata: {
      available_targets: ["global-mcp-sidecar"],
      effective_source: { kind: "global-mcp-sidecar", scope: "user" },
    },
  },
];

export const settingsHookFixtures: SettingsHookEntry[] =
  settingsHooksExtensionsSectionFixture.hooks ?? [];

export const settingsProvidersCollectionFixture = {
  available_scopes: ["user"],
  collection: "providers",
  providers: settingsProviderFixtures,
  scope: "user",
} satisfies SettingsProviderCollection;

export const settingsSandboxesCollectionFixture = {
  available_scopes: ["user"],
  collection: "sandboxes",
  sandboxes: settingsSandboxFixtures,
  scope: "user",
} satisfies SettingsSandboxCollection;

export const settingsMCPServersCollectionFixture = {
  available_scopes: ["user", "profile", "workspace"],
  collection: "mcp-servers",
  mcp_servers: settingsMCPServerFixtures,
  scope: "user",
} satisfies SettingsMCPServerCollection;

// The reference visual-contract matrix: nine servers exercising every auth x
// runtime x probe cell (docs/design/opendesign/mcp-management.html SERVERS).
function mcpConfigSource(kind: string, scope: "user" | "profile" | "workspace") {
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
      client_secret_configured: false,
      registration: "auto",
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
    transport: "http",
    url: "https://mcp.sentry.dev/mcp",
    auth: {
      client_secret_configured: false,
      registration: "auto",
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
      protocol_version: "2026-07-28",
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
      client_secret_configured: false,
      registration: "auto",
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
    name: "grafana",
    transport: "http",
    url: "https://mcp.grafana.com/mcp",
    auth: {
      client_secret_configured: false,
      registration: "auto",
    },
    auth_status: {
      server_name: "grafana",
      scope: "user",
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
    scope: "user",
    source_metadata: mcpConfigSource("global-config", "user"),
  },
  {
    name: "github-remote",
    transport: "http",
    url: "https://api.githubcopilot.com/mcp",
    auth: {
      client_secret_configured: false,
      registration: "auto",
    },
    auth_status: {
      server_name: "github-remote",
      scope: "user",
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
    scope: "user",
    source_metadata: mcpConfigSource("global-config", "user"),
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
      client_secret_configured: false,
      registration: "auto",
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
    transport: "http",
    url: "https://mcp.buildkite.com/mcp",
    auth: {
      client_secret_configured: false,
      registration: "auto",
    },
    auth_status: {
      server_name: "buildkite",
      scope: "user",
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
    scope: "user",
    source_metadata: mcpConfigSource("global-config", "user"),
  },
];

export const mcpManagementCollectionFixture = {
  available_scopes: ["user", "workspace"],
  collection: "mcp-servers",
  mcp_servers: mcpManagementServerFixtures,
  scope: "workspace",
  workspace_id: "ws-platform",
} satisfies SettingsMCPServerCollection;

export const mcpAuthBeginFixture = {
  authorization_url:
    "https://auth.linear.app/oauth/authorize?client_id=compozy-linear-public&code_challenge=J6lRkq8l9lZ3tZpQf7uYx2AqY7M3xv2b9R6n0ZsVn4A&code_challenge_method=S256&redirect_uri=http%3A%2F%2F127.0.0.1%3A2123%2Fapi%2Fmcp%2Foauth%2Fcallback&state=compozy_mcp_7m3p9q",
  callback_url: "http://127.0.0.1:2123/api/mcp/oauth/callback",
  expires_at: new Date(Date.now() + 5 * 60_000).toISOString(),
  manual_supported: true,
  state: "compozy_mcp_7m3p9q",
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
  old_socket_path: "/tmp/compozy.sock",
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
  scope: "user",
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
  scope: "user",
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
          suggested_command: "compozy config reload",
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
          suggested_command: "compozy config validate",
        },
      ],
      created_at: "2026-05-20T13:24:00Z",
      updated_at: "2026-05-20T13:24:02Z",
    },
  ],
};
