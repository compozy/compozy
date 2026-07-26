import { describe, expect, it } from "vitest";

import type { RuntimeProviderOption } from "@/systems/runtime";

import {
  buildCreateAgentParams,
  buildDraftFromAgentPayload,
  buildDuplicateAgentParams,
  createDefaultAgentCreateDraft,
  validateAgentCreateDraft,
  type AgentCreateDialogDraft,
  type AgentCreateValidationContext,
} from "../agent-create-draft";
import type { AgentPayload } from "../../types";

const providerOptions: readonly RuntimeProviderOption[] = [{ id: "codex", name: "Codex" }];

const context: AgentCreateValidationContext = {
  hasActiveWorkspace: false,
  providerOptions,
  providersError: null,
  providersLoading: false,
};

/** A globally-scoped draft that is valid apart from the reasoning field under test. */
function baseDraft(reasoningEffort: string): AgentCreateDialogDraft {
  return {
    ...createDefaultAgentCreateDraft(false),
    scope: "global",
    name: "release-captain",
    provider: "codex",
    prompt: "Own release readiness.",
    reasoningEffort: reasoningEffort as AgentCreateDialogDraft["reasoningEffort"],
  };
}

describe("agent-create-draft reasoning effort validation", () => {
  it("Should inherit project runtime when no provider override is authored", () => {
    const draft = {
      ...baseDraft(""),
      provider: "",
      model: "",
    };

    const validation = validateAgentCreateDraft(draft, context);
    expect(validation.fields.provider).toBeUndefined();
    expect(validation.simpleValid).toBe(true);

    const params = buildCreateAgentParams(draft, null, context);
    expect(params).not.toBeNull();
    expect(params?.agent).not.toHaveProperty("provider");
    expect(params?.agent).not.toHaveProperty("model");
    expect(params?.agent).not.toHaveProperty("reasoning_effort");
  });

  it("Should flag an off-contract reasoning effort as a runtime-step field error", () => {
    const validation = validateAgentCreateDraft(baseDraft("ultra"), context);

    expect(validation.fields.reasoningEffort).toBe("Choose a valid reasoning effort.");
    expect(validation.simpleValid).toBe(false);
    expect(validation.canSubmit).toBe(false);
  });

  it("Should return null from buildCreateAgentParams for an off-contract reasoning effort", () => {
    expect(buildCreateAgentParams(baseDraft("ultra"), null, context)).toBeNull();
  });

  it("Should accept the canonical empty effort as provider default and omit it from the request", () => {
    const validation = validateAgentCreateDraft(baseDraft(""), context);
    expect(validation.fields.reasoningEffort).toBeUndefined();
    expect(validation.simpleValid).toBe(true);

    const params = buildCreateAgentParams(baseDraft(""), null, context);
    expect(params).not.toBeNull();
    expect(params?.agent).not.toHaveProperty("reasoning_effort");
  });

  it("Should carry a canonical reasoning effort through to the built request", () => {
    const params = buildCreateAgentParams(baseDraft("high"), null, context);
    expect(params?.agent.reasoning_effort).toBe("high");
  });
});

describe("agent-create-draft disclosure tiers", () => {
  it("Should isolate an advanced-only failure from the Simple tier so submit can reveal it", () => {
    const validation = validateAgentCreateDraft({ ...baseDraft(""), tools: ["  "] }, context);

    expect(validation.fields.tools).toBe("Tool entries cannot be blank.");
    expect(validation.advancedValid).toBe(false);
    expect(validation.simpleValid).toBe(true);
    expect(validation.canSubmit).toBe(false);
  });

  it("Should fail the Simple tier for an invalid category path, which Simple now renders", () => {
    const validation = validateAgentCreateDraft(
      { ...baseDraft(""), categoryPath: "operations//incident" },
      context
    );

    expect(validation.fields.categoryPath).toBe("Category path cannot contain blank segments.");
    expect(validation.simpleValid).toBe(false);
    expect(validation.canSubmit).toBe(false);
  });

  it("Should split the mono category path into contract segments at the adapter boundary", () => {
    const params = buildCreateAgentParams(
      { ...baseDraft(""), categoryPath: " operations / incident " },
      null,
      context
    );

    expect(params?.agent.category_path).toEqual(["operations", "incident"]);
  });

  it("Should never emit an mcp_servers field, which the create payload cannot accept", () => {
    const params = buildCreateAgentParams(baseDraft("high"), null, context);

    expect(params?.agent).not.toHaveProperty("mcp_servers");
    expect(JSON.stringify(params)).not.toContain("mcp");
  });
});

describe("duplicate draft mapping", () => {
  const source: AgentPayload = {
    name: "release-captain",
    provider: "codex",
    prompt: "Own release readiness.",
    model: "gpt-5.4",
    tools: ["agh__skill_view"],
    toolsets: ["agh__catalog"],
    deny_tools: ["agh__task_*"],
    origin: "workspace",
    definition_digest: "d".repeat(64),
    category_path: ["Engineering", "Release"],
    skills: { disabled: ["copywriting"] },
  };

  it("Should map payload into a blank-name draft", () => {
    const draft = buildDraftFromAgentPayload(source, true);
    expect(draft.name).toBe("");
    expect(draft.scope).toBe("workspace");
    expect(draft.provider).toBe("codex");
    expect(draft.model).toBe("gpt-5.4");
    expect(draft.tools).toEqual(["agh__skill_view"]);
    expect(draft.categoryPath).toBe("Engineering/Release");
    expect(draft.disabledSkills).toEqual(["copywriting"]);
  });

  it("Should build override-diff duplicate params and never a create body", () => {
    const draft = {
      ...buildDraftFromAgentPayload(source, true),
      name: "release-captain-copy",
      prompt: "Own release readiness, carefully.",
    };
    const workspaceContext: AgentCreateValidationContext = {
      ...context,
      hasActiveWorkspace: true,
      providerOptions,
    };
    const params = buildDuplicateAgentParams(source, draft, "ws_alpha", workspaceContext);
    expect(params).toEqual({
      name: "release-captain-copy",
      scope: "workspace",
      workspace: "ws_alpha",
      overrides: {
        prompt: "Own release readiness, carefully.",
      },
    });
    expect(params).not.toHaveProperty("agent");
  });
});
