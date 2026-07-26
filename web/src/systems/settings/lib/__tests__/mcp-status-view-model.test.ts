import { describe, expect, it } from "vitest";

import type { SettingsMCPServerEntry } from "../../types";
import { deriveMCPManagementFilter } from "../mcp-management-target";
import {
  authTone,
  authorizeLabel,
  composeMCPRowStatus,
  deriveMCPAuthFilter,
  formatStatusLabel,
  isOAuthCapable,
  isOAuthRepairable,
  MCP_AUTH_STATUSES,
  MCP_PROBE_STATES,
  MCP_RUNTIME_STATES,
  probeTone,
  probeToolLabel,
  runtimeTone,
} from "../mcp-status-view-model";

type AuthStatus = NonNullable<SettingsMCPServerEntry["auth_status"]>;
type RuntimeStatus = NonNullable<SettingsMCPServerEntry["runtime_status"]>;
type AuthConfig = NonNullable<SettingsMCPServerEntry["auth"]>;

interface EntryOverrides {
  transport?: string;
  auth?: AuthConfig | null;
  authStatus?: Partial<AuthStatus> | null;
  runtimeStatus?: Partial<RuntimeStatus> | null;
  effectiveSource?: {
    kind: string;
    scope: "global" | "workspace";
    workspace_id?: string;
  };
  scope?: "global" | "workspace";
  workspaceId?: string;
}

// Minimal fixture at the type boundary: the view-model only reads
// transport/auth/auth_status/runtime_status, so unrelated required fields are
// filled once here and cast.
function makeEntry(overrides: EntryOverrides = {}): SettingsMCPServerEntry {
  const authStatus =
    overrides.authStatus === undefined
      ? null
      : overrides.authStatus === null
        ? null
        : ({
            server_name: "srv",
            scope: "workspace",
            status: "unconfigured",
            refreshable: false,
            token_present: false,
            ...overrides.authStatus,
          } as AuthStatus);
  const runtimeStatus =
    overrides.runtimeStatus === undefined
      ? null
      : overrides.runtimeStatus === null
        ? null
        : ({
            configured: true,
            initialized: false,
            state: "ready",
            probe: "skipped",
            tool_count: 0,
            ...overrides.runtimeStatus,
          } as RuntimeStatus);
  return {
    name: "srv",
    transport: overrides.transport ?? "stdio",
    scope: overrides.scope ?? "workspace",
    auth: overrides.auth ?? null,
    auth_status: authStatus,
    runtime_status: runtimeStatus,
    source_metadata: {
      effective_source: overrides.effectiveSource,
    },
    workspace_id: overrides.workspaceId,
  } as SettingsMCPServerEntry;
}

function oauthConfig(): AuthConfig {
  return { type: "oauth2_pkce", client_id: "agh-test" } as AuthConfig;
}

describe("mcp status tone mapping", () => {
  it("maps every auth status to the documented tone", () => {
    expect(authTone("authenticated", true)).toBe("success");
    expect(authTone("authenticated", false)).toBe("neutral"); // authenticated without a token is not green
    expect(authTone("invalid", true)).toBe("danger");
    expect(authTone("needs_login", false)).toBe("warning");
    expect(authTone("expired", true)).toBe("warning");
    expect(authTone("unconfigured", false)).toBe("neutral");
  });

  it("renders an unknown auth status neutral instead of inventing a dialect", () => {
    expect(authTone("some_future_state", false)).toBe("neutral");
  });

  it("maps every runtime state to the documented tone", () => {
    expect(runtimeTone("ready")).toBe("success");
    expect(runtimeTone("auth_required")).toBe("warning");
    expect(runtimeTone("auth_expired")).toBe("warning");
    for (const state of [
      "config_error",
      "auth_invalid",
      "auth_refresh_failed",
      "permission_denied",
      "runtime_unavailable",
      "dead",
    ]) {
      expect(runtimeTone(state)).toBe("danger");
    }
  });

  it("renders non-daemon runtime states neutral instead of carrying compatibility aliases", () => {
    expect(runtimeTone("refresh_failed")).toBe("neutral");
    expect(runtimeTone("some_future_state")).toBe("neutral");
  });

  it("covers the full runtime vocabulary with a defined tone", () => {
    for (const state of MCP_RUNTIME_STATES) {
      expect(["success", "warning", "danger", "neutral", "info"]).toContain(runtimeTone(state));
    }
  });

  it("maps probe states and never renders skipped as a failure", () => {
    expect(probeTone("succeeded")).toBe("success");
    expect(probeTone("failed")).toBe("danger");
    expect(probeTone("skipped")).toBe("neutral");
    expect(probeTone("skipped")).not.toBe("danger");
  });

  it("keeps the vocabularies aligned with the daemon", () => {
    expect([...MCP_AUTH_STATUSES]).toEqual([
      "unconfigured",
      "needs_login",
      "authenticated",
      "expired",
      "invalid",
    ]);
    expect([...MCP_PROBE_STATES]).toEqual(["skipped", "succeeded", "failed"]);
  });
});

describe("probeToolLabel", () => {
  it("renders the tool count only on a succeeded probe", () => {
    expect(probeToolLabel("succeeded", 12)).toBe("12 tools");
    expect(probeToolLabel("succeeded", 1)).toBe("1 tool");
  });

  it("never fabricates a count for skipped or failed probes", () => {
    expect(probeToolLabel("skipped", 5)).toBe("no probe attempted");
    expect(probeToolLabel("failed", 5)).toBe("probe attempted");
    expect(probeToolLabel(undefined, 5)).toBeUndefined();
  });
});

describe("formatStatusLabel", () => {
  it("converts snake_case daemon values to sentence case", () => {
    expect(formatStatusLabel("auth_refresh_failed")).toBe("Auth refresh failed");
    expect(formatStatusLabel("ready")).toBe("Ready");
  });
});

describe("authorize gating", () => {
  it("offers authorize only to oauth-capable remotes that are not authenticated", () => {
    const remote = makeEntry({
      transport: "http",
      auth: oauthConfig(),
      authStatus: { status: "needs_login", token_present: false },
      runtimeStatus: { state: "auth_required", probe: "skipped" },
    });
    expect(isOAuthCapable(remote)).toBe(true);
    expect(isOAuthRepairable(remote)).toBe(true);
    expect(authorizeLabel(remote)).toBe("Authorize");
  });

  it("labels a re-auth Reauthorize when the server was previously authenticated", () => {
    const refreshFailed = makeEntry({
      transport: "http",
      auth: oauthConfig(),
      authStatus: { status: "authenticated", token_present: true },
      runtimeStatus: { state: "auth_refresh_failed", probe: "skipped" },
    });
    expect(isOAuthRepairable(refreshFailed)).toBe(true);
    expect(authorizeLabel(refreshFailed)).toBe("Reauthorize");
  });

  it("does not offer authorize to a healthy authenticated remote", () => {
    const healthy = makeEntry({
      transport: "sse",
      auth: oauthConfig(),
      authStatus: { status: "authenticated", token_present: true },
      runtimeStatus: { state: "ready", probe: "succeeded", tool_count: 8 },
    });
    expect(isOAuthRepairable(healthy)).toBe(false);
    expect(authorizeLabel(healthy)).toBeNull();
  });

  it("never offers browser oauth to stdio or non-oauth remotes", () => {
    const stdio = makeEntry({
      transport: "stdio",
      authStatus: null,
      runtimeStatus: { state: "config_error", probe: "skipped" },
    });
    const headerAuth = makeEntry({
      transport: "http",
      auth: null,
      authStatus: { status: "needs_login", token_present: false },
      runtimeStatus: { state: "auth_required", probe: "skipped" },
    });
    expect(isOAuthCapable(stdio)).toBe(false);
    expect(isOAuthRepairable(stdio)).toBe(false);
    expect(authorizeLabel(stdio)).toBeNull();
    expect(isOAuthCapable(headerAuth)).toBe(false);
    expect(authorizeLabel(headerAuth)).toBeNull();
  });

  it("still offers repair for permission_denied only when it is an auth-refresh case", () => {
    // permission_denied on an authenticated server is a runtime problem, not an
    // authorization one, so no browser authorize is offered.
    const permissionDenied = makeEntry({
      transport: "http",
      auth: oauthConfig(),
      authStatus: { status: "authenticated", token_present: true },
      runtimeStatus: { state: "permission_denied", probe: "failed" },
    });
    expect(isOAuthRepairable(permissionDenied)).toBe(false);
  });
});

describe("authorization target", () => {
  it("uses a global effective source even when the collection row is workspace-scoped", () => {
    const server = makeEntry({
      effectiveSource: { kind: "global-config", scope: "global" },
      scope: "workspace",
      workspaceId: "ws-active",
    });

    expect(deriveMCPAuthFilter(server)).toEqual({ scope: "global" });
  });

  it("uses the effective workspace source identity", () => {
    const server = makeEntry({
      effectiveSource: {
        kind: "workspace-config",
        scope: "workspace",
        workspace_id: "ws-owner",
      },
      scope: "global",
    });

    expect(deriveMCPAuthFilter(server)).toEqual({
      scope: "workspace",
      workspace_id: "ws-owner",
    });
  });

  it("rejects a workspace effective source without its workspace identity", () => {
    const server = makeEntry({
      effectiveSource: { kind: "workspace-config", scope: "workspace" },
    });

    expect(deriveMCPAuthFilter(server)).toBeNull();
  });

  it.each([
    ["global-config", { scope: "global", target: "config" }],
    ["global-mcp-sidecar", { scope: "global", target: "sidecar" }],
  ] as const)("maps %s to its exact global management target", (kind, expected) => {
    const server = makeEntry({ effectiveSource: { kind, scope: "global" } });

    expect(deriveMCPManagementFilter(server)).toEqual(expected);
  });

  it.each([
    ["workspace-config", "config"],
    ["workspace-mcp-sidecar", "sidecar"],
  ] as const)("maps %s to its exact workspace management target", (kind, target) => {
    const server = makeEntry({
      effectiveSource: { kind, scope: "workspace", workspace_id: "ws-owner" },
      scope: "global",
    });

    expect(deriveMCPManagementFilter(server)).toEqual({
      scope: "workspace",
      target,
      workspace_id: "ws-owner",
    });
  });
});

describe("composeMCPRowStatus", () => {
  it("composes four independent cells for a needs-login remote with no false green", () => {
    const status = composeMCPRowStatus(
      makeEntry({
        transport: "http",
        auth: oauthConfig(),
        authStatus: { status: "needs_login", token_present: false, diagnostic: "token absent" },
        runtimeStatus: { state: "auth_required", probe: "skipped" },
      })
    );
    expect(status.config).toEqual({
      label: "Configured",
      tone: "neutral",
      code: "configured=true",
    });
    expect(status.auth).toEqual({ label: "Needs login", tone: "warning", code: "token absent" });
    expect(status.runtime.tone).toBe("warning");
    expect(status.runtime.label).toBe("Auth required");
    expect(status.probe).toEqual({ label: "Skipped", tone: "neutral", code: "no probe attempted" });
    expect(status.repairable).toBe(true);
  });

  it("renders a healthy authenticated server with a real tool count", () => {
    const status = composeMCPRowStatus(
      makeEntry({
        transport: "sse",
        auth: oauthConfig(),
        authStatus: { status: "authenticated", token_present: true },
        runtimeStatus: { state: "ready", probe: "succeeded", tool_count: 18 },
      })
    );
    expect(status.auth.tone).toBe("success");
    expect(status.runtime.tone).toBe("success");
    expect(status.probe).toEqual({ label: "Succeeded", tone: "success", code: "18 tools" });
    expect(status.repairable).toBe(false);
  });

  it("renders a dead workspace runtime as unavailable with its redacted diagnostic", () => {
    const status = composeMCPRowStatus(
      makeEntry({
        transport: "stdio",
        authStatus: null,
        runtimeStatus: {
          state: "dead",
          probe: "skipped",
          reason: "backend_dead",
          diagnostic: "process exited during initialize",
        },
      })
    );
    expect(status.runtime).toEqual({
      label: "Unavailable",
      tone: "danger",
      code: "process exited during initialize",
    });
    expect(status.probe).toEqual({
      label: "Skipped",
      tone: "neutral",
      code: "no probe attempted",
    });

    const withoutDiagnostic = composeMCPRowStatus(
      makeEntry({
        transport: "stdio",
        authStatus: null,
        runtimeStatus: { state: "dead", probe: "skipped", reason: "backend_dead" },
      })
    );
    expect(withoutDiagnostic.runtime.code).toBe("backend_dead");
  });

  it("renders a stdio server with an unconfigured auth cell and hides tool count when absent", () => {
    const status = composeMCPRowStatus(
      makeEntry({
        transport: "stdio",
        authStatus: null,
        runtimeStatus: { state: "config_error", probe: "skipped" },
      })
    );
    expect(status.auth).toEqual({
      label: "Unconfigured",
      tone: "neutral",
      code: "no OAuth configuration",
    });
    expect(status.runtime.tone).toBe("danger");
    expect(status.probe.code).toBe("no probe attempted");
    expect(status.probe.tone).toBe("neutral");
  });

  it("renders neutral cells when runtime status is entirely absent", () => {
    const status = composeMCPRowStatus(
      makeEntry({ transport: "stdio", authStatus: null, runtimeStatus: null })
    );
    expect(status.runtime.tone).toBe("neutral");
    expect(status.probe.tone).toBe("neutral");
    expect(status.probe.code).toBeUndefined();
  });
});
