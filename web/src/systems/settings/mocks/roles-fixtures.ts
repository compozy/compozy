import type { RolesStatusResponse, SettingsRolesSection } from "../types";

/** Editable `[roles]` section — defaults preserving current runtime behavior. */
export const settingsRolesConfigFixture: SettingsRolesSection["config"] = {
  coordinator: {
    acp_options: [],
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
  auto_title: {
    acp_options: [],
    agent: "",
    enabled: true,
    fallback_chain: [],
    model: "",
    provider: "",
    reasoning_effort: "",
  },
};

export const settingsRolesSectionFixture: SettingsRolesSection = {
  available_scopes: ["user"],
  config: settingsRolesConfigFixture,
  scope: "user",
  section: "roles",
};

/**
 * Effective projection returned by `GET /api/roles`, in the daemon's lexical
 * order (the panel reorders to product order). Coordinator is disabled by
 * default; auto_title resolves at invocation.
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
      speed: null,
      acp_options: [],
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default" },
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
      speed: null,
      acp_options: [],
      fallback_chain: [],
      provenance: { enabled: "default", fallback_chain: "default", agent: "default" },
      diagnostics: [],
    },
  ],
};

/** Auto title routed to a missing catalog agent — surfaces `role_agent_not_found`. */
export const rolesStatusWithDiagnosticFixture: RolesStatusResponse = {
  roles: rolesStatusFixture.roles.map(role =>
    role.role === "auto_title"
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

/** Section variant with a populated auto_title fallback chain (fold-open evidence). */
export const settingsRolesConfigWithFallbackFixture: SettingsRolesSection["config"] = {
  ...settingsRolesConfigFixture,
  auto_title: {
    ...settingsRolesConfigFixture.auto_title,
    fallback_chain: [
      {
        provider: "anthropic",
        model: "claude-sonnet-5",
        reasoning_effort: "",
        acp_options: [],
        command: "",
      },
      {
        provider: "openai",
        model: "gpt-5",
        reasoning_effort: "high",
        acp_options: [],
        command: "",
      },
    ],
  },
};

export const settingsRolesSectionWithFallbackFixture: SettingsRolesSection = {
  ...settingsRolesSectionFixture,
  config: settingsRolesConfigWithFallbackFixture,
};
