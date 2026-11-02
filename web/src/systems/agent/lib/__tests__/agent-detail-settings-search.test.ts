import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";

import { filterAgentSessionsByStatus, validateAgentDetailSearch } from "../agent-detail-search";
import { validateAgentSettingsSearch } from "../agent-settings-search";
import { resolveAgentRuntimeValue } from "../agent-effective-runtime";
import {
  formatAbsentList,
  formatAbsentListLabels,
  formatAbsentOverride,
} from "../agent-absent-value";
import {
  buildSettingsDraftFromAgent,
  buildUpdateAgentParams,
  isAgentSettingsDraftDirty,
  validateAgentSettingsDraft,
} from "../agent-settings-draft";
import type { AgentPayload } from "../../types";

function makeAgent(overrides: Partial<AgentPayload> = {}): AgentPayload {
  return {
    name: "coder",
    provider: "anthropic",
    prompt: "Code carefully.",
    origin: "workspace",
    definition_digest: "d".repeat(64),
    ...overrides,
  } as AgentPayload;
}

function makeSession(overrides: Partial<SessionPayload> = {}): SessionPayload {
  return {
    id: "sess-1",
    name: "Work",
    agent_name: "coder",
    provider: "anthropic",
    workspace_id: "ws_alpha",
    workspace_path: "/tmp",
    state: "active",
    badge: "idle",
    attachable: true,
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:00:00Z",
    ...overrides,
  } as SessionPayload;
}

describe("validateAgentDetailSearch", () => {
  it("Should default to the home with no panel", () => {
    expect(validateAgentDetailSearch({})).toEqual({});
  });

  it("Should accept a known panel and drop anything else", () => {
    expect(validateAgentDetailSearch({ panel: "tools" })).toEqual({ panel: "tools" });
    expect(validateAgentDetailSearch({ panel: "nope", tab: "overview" })).toEqual({});
  });
});

describe("filterAgentSessionsByStatus", () => {
  it("Should map All/Active/Failed/Done per existing derivations", () => {
    const sessions = [
      makeSession({ id: "a", state: "active" }),
      makeSession({
        id: "f",
        state: "stopped",
        stop_reason: "error",
        failure: { kind: "agent_crashed", summary: "boom" },
      }),
      makeSession({ id: "d", state: "stopped", badge: "stopped" }),
    ];
    expect(filterAgentSessionsByStatus(sessions, "all")).toHaveLength(3);
    expect(filterAgentSessionsByStatus(sessions, "active").map(s => s.id)).toEqual(["a"]);
    expect(filterAgentSessionsByStatus(sessions, "failed").map(s => s.id)).toEqual(["f"]);
    expect(filterAgentSessionsByStatus(sessions, "done").map(s => s.id)).toEqual(["d"]);
  });
});

describe("validateAgentSettingsSearch", () => {
  it("Should default invalid section to basics", () => {
    expect(validateAgentSettingsSearch({})).toEqual({ section: "basics" });
    expect(validateAgentSettingsSearch({ section: "nope" })).toEqual({ section: "basics" });
    expect(validateAgentSettingsSearch({ section: "mcp" })).toEqual({ section: "mcp" });
    // Delete lives at the bottom of Basics; the retired `danger` deep link lands there.
    expect(validateAgentSettingsSearch({ section: "danger" })).toEqual({ section: "basics" });
  });
});

describe("agent-absent-value", () => {
  it("Should render Default/None without em dashes", () => {
    expect(formatAbsentOverride(undefined)).toBe("Default");
    expect(formatAbsentOverride("  ")).toBe("Default");
    expect(formatAbsentOverride("claude")).toBe("claude");
    expect(formatAbsentList(0)).toBe("None");
    expect(formatAbsentListLabels([])).toBe("None");
    expect(formatAbsentListLabels(["github", "linear"])).toBe("github · linear");
  });
});

describe("agent-settings-draft", () => {
  // Invariant: typed ACP selections survive agent runtime reads and updates
  // without exposing provider transport aliases or stringified booleans.
  // Owning layer: agent runtime projection and settings request mapper.
  // Canonical suite: agent-detail-settings-search unit suite.
  it("Should preserve blank authored runtime fields while project defaults remain effective", () => {
    const agent = makeAgent({
      provider: "",
      model: undefined,
      reasoning_effort: undefined,
      effective_runtime: {
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
        sources: {
          provider: "project_default",
          model: "provider_default",
          reasoning_effort: "model_default",
        },
      },
    });

    const draft = buildSettingsDraftFromAgent(agent);
    expect(draft.provider).toBe("");
    expect(draft.model).toBe("");
    expect(draft.reasoningEffort).toBe("");
    expect(validateAgentSettingsDraft(draft).canSave).toBe(true);

    const params = buildUpdateAgentParams({ ...draft, prompt: "Updated." }, "ws_alpha");
    expect(params?.agent).not.toHaveProperty("provider");
    expect(params?.agent).not.toHaveProperty("model");
    expect(params?.agent).not.toHaveProperty("reasoning_effort");
  });

  it("Should seed a draft and detect dirty fields", () => {
    const agent = makeAgent({ model: "claude-sonnet", command: undefined });
    const draft = buildSettingsDraftFromAgent(agent);
    expect(draft.permissions).toBe("");
    expect(isAgentSettingsDraftDirty(draft, agent)).toBe(false);
    expect(isAgentSettingsDraftDirty({ ...draft, model: "other" }, agent)).toBe(true);
  });

  it("Should require explicit choice for unrecognized legacy permissions", () => {
    const agent = makeAgent({ permissions: "legacy-mode" });
    const draft = buildSettingsDraftFromAgent(agent);
    expect(draft.permissions).toBeNull();
    expect(draft.legacyPermissions).toBe("legacy-mode");
    const validation = validateAgentSettingsDraft(draft);
    expect(validation.canSave).toBe(false);
    expect(validation.fields.permissions).toContain("Unrecognized permission mode: legacy-mode");
    expect(buildUpdateAgentParams(draft, "ws_alpha")).toBeNull();
  });

  it("Should build update params with expected_digest after explicit permission choice", () => {
    const agent = makeAgent({ permissions: "legacy-mode", origin: "workspace" });
    const draft = {
      ...buildSettingsDraftFromAgent(agent),
      permissions: "approve-reads" as const,
      legacyPermissions: null,
    };
    const params = buildUpdateAgentParams(draft, "ws_alpha");
    expect(params).not.toBeNull();
    expect(params?.expected_digest).toBe(agent.definition_digest);
    expect(params?.workspace).toBe("ws_alpha");
    expect(params?.agent.permissions).toBe("approve-reads");
  });

  it("Should preserve typed ACP selections in the runtime projection and update body", () => {
    const agent = makeAgent({
      acp_options: [
        { id: "context", value_id: "1m" },
        { id: "thinking", bool_value: true },
      ],
    });

    expect(resolveAgentRuntimeValue(agent).acp_options).toEqual([
      { id: "context", value_id: "1m" },
      { id: "thinking", bool_value: true },
    ]);
    const draft = buildSettingsDraftFromAgent(agent);
    expect(draft.acpOptions).toEqual([
      { id: "context", value_id: "1m" },
      { id: "thinking", bool_value: true },
    ]);

    const params = buildUpdateAgentParams(
      { ...draft, acpOptions: [{ id: "thinking", bool_value: false }] },
      "ws_alpha"
    );
    expect(params?.agent.acp_options).toEqual([{ id: "thinking", bool_value: false }]);
  });

  // Invariant: a Web edit replaces the whole definition, so the operator's
  // `unrestricted` grant survives an unrelated edit and Default clears it.
  it("Should carry the workspace grant through an edit and clear it on Default", () => {
    const agent = makeAgent({ workspace_access: "unrestricted", layer: "user" });
    const draft = buildSettingsDraftFromAgent(agent);
    expect(draft.workspaceAccess).toBe("unrestricted");
    expect(isAgentSettingsDraftDirty(draft, agent)).toBe(false);

    expect(
      buildUpdateAgentParams({ ...draft, prompt: "Updated." }, "ws_alpha")?.agent.workspace_access
    ).toBe("unrestricted");
    const cleared = { ...draft, workspaceAccess: "" as const };
    expect(isAgentSettingsDraftDirty(cleared, agent)).toBe(true);
    expect(buildUpdateAgentParams(cleared, "ws_alpha")?.agent).not.toHaveProperty(
      "workspace_access"
    );
  });

  // Invariant: a Web edit replaces the whole definition, so the authored
  // fallback chain read from the agent must reach the replacement payload
  // unchanged (minus read-only projection fields) even without route controls.
  it("Should carry the authored fallback chain through an unrelated settings edit", () => {
    const agent = makeAgent({
      session_env: { GH_TOKEN: { ref: "env:GH_TOKEN", required: true } },
      fallback_chain: [
        {
          provider: "codex",
          model: "gpt-5.4",
          reasoning_effort: "high",
          speed: "fast",
          acp_options: [{ id: "thinking", bool_value: true }],
          command: "SEAT=2 codex --acp",
          command_fingerprint: "sha256:abc",
          session_env: { GH_TOKEN: { ref: "vault:providers/codex/github", required: true } },
        },
        { provider: "claude", model: "sonnet" },
      ],
    });

    const draft = buildSettingsDraftFromAgent(agent);
    expect(isAgentSettingsDraftDirty(draft, agent)).toBe(false);

    const params = buildUpdateAgentParams({ ...draft, prompt: "Updated." }, "ws_alpha");
    expect(params?.agent.session_env).toEqual(agent.session_env);
    expect(params?.agent.fallback_chain).toEqual([
      {
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
        speed: "fast",
        acp_options: [{ id: "thinking", bool_value: true }],
        command: "SEAT=2 codex --acp",
        session_env: { GH_TOKEN: { ref: "vault:providers/codex/github", required: true } },
      },
      { provider: "claude", model: "sonnet" },
    ]);
  });

  it("Should preserve the authored Fast default in the settings update body", () => {
    const agent = makeAgent({ speed: "fast" });
    const draft = buildSettingsDraftFromAgent(agent);

    expect(draft.speed).toBe("fast");
    expect(buildUpdateAgentParams(draft, "ws_alpha")?.agent.speed).toBe("fast");
  });
});
