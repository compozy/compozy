import type { RolesStatusResponse, SettingsRolesSection } from "../types";

/** Editable `[roles]` section — defaults preserving current runtime behavior. */
export const settingsRolesConfigFixture: SettingsRolesSection["config"] = {
  coordinator: {
    agent: "",
    enabled: false,
    fallback_chain: [],
    max_active_sessions_per_workspace: 5,
    max_children: 5,
    model: "",
    provider: "",
    reasoning_effort: "",
    ttl: "2h",
  },
  dream: {
    agent: "",
    enabled: true,
    fallback_chain: [],
    model: "",
    provider: "",
    reasoning_effort: "",
  },
  checkpoint_summary: {
    agent: "",
    enabled: true,
    fallback_chain: [],
    model: "",
    provider: "",
    reasoning_effort: "",
  },
  memory_extractor: {
    agent: "",
    enabled: true,
    fallback_chain: [],
    model: "",
    provider: "",
    reasoning_effort: "",
  },
  auto_title: {
    agent: "",
    enabled: true,
    fallback_chain: [],
    model: "",
    provider: "",
    reasoning_effort: "",
  },
  memory_controller: {
    enabled: true,
    fallback_chain: [],
    max_tokens_out: 256,
    model: "anthropic/claude-haiku-4",
    prompt_version: "v1",
    provider: "pi",
    reasoning_effort: "",
    timeout: "250ms",
    top_k: 5,
  },
};

export const settingsRolesSectionFixture: SettingsRolesSection = {
  available_scopes: ["global"],
  config: settingsRolesConfigFixture,
  scope: "global",
  section: "roles",
};

/**
 * Effective projection returned by `GET /api/roles`, in the daemon's lexical
 * order (the panel reorders to product order). Coordinator is disabled by
 * default; auto_title/memory_extractor/memory_controller resolve at invocation.
 */
export const rolesStatusFixture: RolesStatusResponse = {
  roles: [
    {
      role: "auto_title",
      enabled: true,
      resolution_mode: "inherit",
      agent: null,
      provider: null,
      model: null,
      reasoning_effort: null,
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default" },
      diagnostics: [],
    },
    {
      role: "checkpoint_summary",
      enabled: true,
      resolution_mode: "builtin",
      agent: "dreaming-curator",
      provider: null,
      model: null,
      reasoning_effort: null,
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default", agent: "default" },
      diagnostics: [],
    },
    {
      role: "coordinator",
      enabled: false,
      resolution_mode: "builtin",
      agent: "coordinator",
      provider: null,
      model: null,
      reasoning_effort: null,
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default", agent: "default" },
      diagnostics: [],
    },
    {
      role: "dream",
      enabled: true,
      resolution_mode: "builtin",
      agent: "dreaming-curator",
      provider: null,
      model: null,
      reasoning_effort: null,
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default", agent: "default" },
      diagnostics: [],
    },
    {
      role: "memory_controller",
      enabled: true,
      resolution_mode: "inherit",
      agent: null,
      provider: "pi",
      model: "anthropic/claude-haiku-4",
      reasoning_effort: null,
      timeout: "250ms",
      fallback_chain: [],
      provenance: {
        enabled: "default",
        fallback_chain: "default",
        model: "default",
        provider: "default",
        timeout: "default",
      },
      diagnostics: [],
    },
    {
      role: "memory_extractor",
      enabled: true,
      resolution_mode: "inherit",
      agent: null,
      provider: null,
      model: null,
      reasoning_effort: null,
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default" },
      diagnostics: [],
    },
  ],
};

/** Dream routed to a missing catalog agent — surfaces `role_agent_not_found`. */
export const rolesStatusWithDiagnosticFixture: RolesStatusResponse = {
  roles: rolesStatusFixture.roles.map(role =>
    role.role === "dream"
      ? {
          ...role,
          resolution_mode: "catalog",
          agent: "ghost",
          provenance: { ...role.provenance, agent: "workspace" },
          diagnostics: [
            {
              code: "role_agent_not_found",
              message: 'Agent "ghost" is not available',
              agent: "ghost",
            },
          ],
        }
      : role
  ),
};

/** Section variant with a populated dream fallback chain (fold-open evidence). */
export const settingsRolesConfigWithFallbackFixture: SettingsRolesSection["config"] = {
  ...settingsRolesConfigFixture,
  dream: {
    ...settingsRolesConfigFixture.dream,
    fallback_chain: [
      { provider: "anthropic", model: "claude-sonnet-5", reasoning_effort: "" },
      { provider: "openai", model: "gpt-5", reasoning_effort: "high" },
    ],
  },
};

export const settingsRolesSectionWithFallbackFixture: SettingsRolesSection = {
  ...settingsRolesSectionFixture,
  config: settingsRolesConfigWithFallbackFixture,
};
