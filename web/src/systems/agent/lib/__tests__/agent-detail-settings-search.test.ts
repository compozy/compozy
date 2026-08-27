import { describe, expect, it } from "vitest";

import type { SessionPayload } from "@/systems/session";

import { filterAgentSessionsByStatus, validateAgentDetailSearch } from "../agent-detail-search";
import { validateAgentSettingsSearch } from "../agent-settings-search";
import { resolveAgentRuntimeValue } from "../agent-effective-runtime";
import {
  countPromptWords,
  formatAbsentList,
  formatAbsentListLabels,
  formatAbsentOverride,
  formatPromptWordCount,
  formatSkillsPolicyLine,
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
  it("Should default invalid tab/file/filter values", () => {
    expect(validateAgentDetailSearch({})).toEqual({
      tab: "overview",
      file: "agent",
      filter: "all",
    });
    expect(validateAgentDetailSearch({ tab: "nope", file: "x", filter: "zzz" })).toEqual({
      tab: "overview",
      file: "agent",
      filter: "all",
    });
  });

  it("Should accept valid deep-link params", () => {
    expect(
      validateAgentDetailSearch({
        tab: "instructions",
        file: "soul",
        filter: "failed",
      })
    ).toEqual({ tab: "instructions", file: "soul", filter: "failed" });
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
    expect(validateAgentSettingsSearch({ section: "danger" })).toEqual({ section: "danger" });
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

  it("Should derive prompt word count and skills policy lines", () => {
    expect(countPromptWords("one two three")).toBe(3);
    expect(formatPromptWordCount("one two")).toBe("~2 words");
    expect(formatSkillsPolicyLine(undefined)).toBe("All skills enabled");
    expect(formatSkillsPolicyLine({ disabled: ["copywriting"] })).toBe(
      "1 skill disabled (copywriting)"
    );
    expect(formatSkillsPolicyLine({ disabled: ["a", "b"] })).toBe("2 skills disabled");
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

  it("Should preserve the authored Fast default in the settings update body", () => {
    const agent = makeAgent({ speed: "fast" });
    const draft = buildSettingsDraftFromAgent(agent);

    expect(draft.speed).toBe("fast");
    expect(buildUpdateAgentParams(draft, "ws_alpha")?.agent.speed).toBe("fast");
  });
});
