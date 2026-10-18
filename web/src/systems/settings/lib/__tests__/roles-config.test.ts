import { describe, expect, it } from "vitest";

import {
  rolesStatusFixture,
  settingsRolesConfigFixture,
  settingsRolesConfigWithFallbackFixture,
} from "../../mocks/roles-fixtures";
import {
  addFallbackEntry,
  applyRoleFieldEdit,
  applyRoleRuntimeEdit,
  clearRoleRuntime,
  removeFallbackEntry,
  ROLE_ORDER,
  setFallbackRuntime,
} from "../roles-config";
import { buildRolesViewModel } from "../roles-view-model";
import { collectRoleValidationErrors, fallbackFieldId } from "../roles-validation";

describe("applyRoleFieldEdit", () => {
  it("Should set one scalar field immutably without touching other roles or the original", () => {
    const next = applyRoleFieldEdit(
      settingsRolesConfigFixture,
      "auto_title",
      "model",
      "claude-haiku-4-5"
    );

    expect(next.auto_title.model).toBe("claude-haiku-4-5");
    expect(settingsRolesConfigFixture.auto_title.model).toBe("");
    expect(next.coordinator).toEqual(settingsRolesConfigFixture.coordinator);
    expect(next).not.toBe(settingsRolesConfigFixture);
  });

  it("Should apply boolean and numeric edits by kind", () => {
    const toggled = applyRoleFieldEdit(settingsRolesConfigFixture, "coordinator", "enabled", true);
    expect(toggled.coordinator.enabled).toBe(true);
    const bumped = applyRoleFieldEdit(settingsRolesConfigFixture, "coordinator", "max_children", 3);
    expect(bumped.coordinator.max_children).toBe(3);
  });
});

describe("fallback chain operations", () => {
  it("Should append an empty entry immutably", () => {
    const next = addFallbackEntry(settingsRolesConfigFixture, "auto_title");
    expect(next.auto_title.fallback_chain).toHaveLength(1);
    expect(next.auto_title.fallback_chain[0]).toEqual({
      provider: "",
      model: "",
      reasoning_effort: "",
      acp_options: [],
      command: "",
    });
    expect(settingsRolesConfigFixture.auto_title.fallback_chain).toHaveLength(0);
  });

  it("Should remove the entry at the given index", () => {
    const next = removeFallbackEntry(settingsRolesConfigWithFallbackFixture, "auto_title", 0);
    expect(next.auto_title.fallback_chain).toHaveLength(1);
    expect(next.auto_title.fallback_chain[0].provider).toBe("openai");
  });

  it("Should replace one entry's whole route immutably", () => {
    const next = setFallbackRuntime(settingsRolesConfigWithFallbackFixture, "auto_title", 1, {
      provider: "google",
      model: "gemini-3-pro",
      reasoning_effort: "high",
      speed: "fast",
      acp_options: [{ id: "thinking", bool_value: true }],
    });
    expect(next.auto_title.fallback_chain[1]).toEqual({
      provider: "google",
      model: "gemini-3-pro",
      reasoning_effort: "high",
      speed: "fast",
      acp_options: [{ id: "thinking", bool_value: true }],
      command: "",
    });
    expect(settingsRolesConfigWithFallbackFixture.auto_title.fallback_chain[1].provider).toBe(
      "openai"
    );
  });

  it("Should keep the route account command when its runtime changes", () => {
    const command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp";
    const withCommand = {
      ...settingsRolesConfigWithFallbackFixture,
      auto_title: {
        ...settingsRolesConfigWithFallbackFixture.auto_title,
        fallback_chain: settingsRolesConfigWithFallbackFixture.auto_title.fallback_chain.map(
          (entry, index) => (index === 0 ? { ...entry, command } : entry)
        ),
      },
    };
    const next = setFallbackRuntime(withCommand, "auto_title", 0, {
      provider: "anthropic",
      model: "claude-opus-4-8",
      reasoning_effort: "high",
      speed: "",
    });
    expect(next.auto_title.fallback_chain[0]?.command).toBe(command);
  });
});

describe("role runtime edits", () => {
  it("Should write provider, model and reasoning together so a route is never half-applied", () => {
    const next = applyRoleRuntimeEdit(settingsRolesConfigFixture, "auto_title", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
      reasoning_effort: "medium",
      speed: "fast",
      acp_options: [{ id: "context", value_id: "1m" }],
    });

    expect(next.auto_title.provider).toBe("anthropic");
    expect(next.auto_title.model).toBe("claude-haiku-4-5");
    expect(next.auto_title.reasoning_effort).toBe("medium");
    expect(next.auto_title.speed).toBe("fast");
    expect(next.auto_title.acp_options).toEqual([{ id: "context", value_id: "1m" }]);
    expect(settingsRolesConfigFixture.auto_title.provider).toBe("");
    expect(next.coordinator).toEqual(settingsRolesConfigFixture.coordinator);
  });

  it("Should clear all three routing keys back to inherit", () => {
    const routed = applyRoleRuntimeEdit(settingsRolesConfigFixture, "auto_title", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
      reasoning_effort: "medium",
      speed: "fast",
      acp_options: [{ id: "context", value_id: "1m" }],
    });
    const cleared = clearRoleRuntime(routed, "auto_title");

    expect(cleared.auto_title.provider).toBe("");
    expect(cleared.auto_title.model).toBe("");
    expect(cleared.auto_title.reasoning_effort).toBe("");
    expect(cleared.auto_title.speed).toBeUndefined();
    expect(cleared.auto_title.acp_options).toEqual([]);
    expect(routed.auto_title.model).toBe("claude-haiku-4-5");
  });
});

describe("buildRolesViewModel", () => {
  it("Should return both roles in fixed product order regardless of API order", () => {
    const models = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
    expect(models.map(model => model.role)).toEqual([...ROLE_ORDER]);
  });

  it("Should flatten draft values and preserve null effective values without fabrication", () => {
    const models = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
    const coordinator = models.find(model => model.role === "coordinator");
    const autoTitle = models.find(model => model.role === "auto_title");

    expect(coordinator?.values.ttl).toBe("2h");
    expect(coordinator?.values.max_children).toBe(5);
    expect(autoTitle?.runtime.model).toBe("");
    expect(autoTitle?.effective.model).toBeNull();
  });

  it("Should expose routing state the header reads without inventing a route", () => {
    const routed = applyRoleRuntimeEdit(settingsRolesConfigFixture, "auto_title", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
      reasoning_effort: "",
      speed: "",
    });
    const models = buildRolesViewModel(rolesStatusFixture.roles, routed);
    const autoTitle = models.find(model => model.role === "auto_title");
    const coordinator = models.find(model => model.role === "coordinator");

    expect(autoTitle?.hasRuntimeOverride).toBe(true);
    expect(coordinator?.hasRuntimeOverride).toBe(false);
    expect(coordinator?.routeSummary).toBeNull();
  });
});

describe("collectRoleValidationErrors", () => {
  it("Should report one error per incomplete fallback route, in product order", () => {
    const config = addFallbackEntry(
      addFallbackEntry(settingsRolesConfigFixture, "auto_title"),
      "auto_title"
    );
    const errors = collectRoleValidationErrors(config);

    expect(errors.map(error => error.id)).toEqual([
      fallbackFieldId("auto_title", 0),
      fallbackFieldId("auto_title", 1),
    ]);
    expect(errors[0].message).toBe("Choose a provider and model.");
  });

  it("Should accept a fully specified fallback entry", () => {
    expect(collectRoleValidationErrors(settingsRolesConfigWithFallbackFixture)).toHaveLength(0);
  });

  it("Should block every positive role limit when a persisted value falls below one", () => {
    const config = {
      ...settingsRolesConfigFixture,
      coordinator: {
        ...settingsRolesConfigFixture.coordinator,
        max_active_sessions_per_workspace: 0,
      },
    };

    expect(collectRoleValidationErrors(config)).toContainEqual({
      id: "coordinator.max_active_sessions_per_workspace",
      message: "Value must be 1 or greater.",
    });
  });
});
