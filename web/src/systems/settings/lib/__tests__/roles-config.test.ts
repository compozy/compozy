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
    expect(next.dream).toEqual(settingsRolesConfigFixture.dream);
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
    const next = addFallbackEntry(settingsRolesConfigFixture, "dream");
    expect(next.dream.fallback_chain).toHaveLength(1);
    expect(next.dream.fallback_chain[0]).toEqual({ provider: "", model: "", reasoning_effort: "" });
    expect(settingsRolesConfigFixture.dream.fallback_chain).toHaveLength(0);
  });

  it("Should remove the entry at the given index", () => {
    const next = removeFallbackEntry(settingsRolesConfigWithFallbackFixture, "dream", 0);
    expect(next.dream.fallback_chain).toHaveLength(1);
    expect(next.dream.fallback_chain[0].provider).toBe("openai");
  });

  it("Should replace one entry's whole route immutably", () => {
    const next = setFallbackRuntime(settingsRolesConfigWithFallbackFixture, "dream", 1, {
      provider: "google",
      model: "gemini-3-pro",
      reasoning_effort: "high",
    });
    expect(next.dream.fallback_chain[1]).toEqual({
      provider: "google",
      model: "gemini-3-pro",
      reasoning_effort: "high",
    });
    expect(settingsRolesConfigWithFallbackFixture.dream.fallback_chain[1].provider).toBe("openai");
  });
});

describe("role runtime edits", () => {
  it("Should write provider, model and reasoning together so a route is never half-applied", () => {
    const next = applyRoleRuntimeEdit(settingsRolesConfigFixture, "auto_title", {
      provider: "anthropic",
      model: "claude-haiku-4-5",
      reasoning_effort: "medium",
    });

    expect(next.auto_title.provider).toBe("anthropic");
    expect(next.auto_title.model).toBe("claude-haiku-4-5");
    expect(next.auto_title.reasoning_effort).toBe("medium");
    expect(settingsRolesConfigFixture.auto_title.provider).toBe("");
    expect(next.dream).toEqual(settingsRolesConfigFixture.dream);
  });

  it("Should clear all three routing keys back to inherit", () => {
    const cleared = clearRoleRuntime(settingsRolesConfigFixture, "memory_controller");

    expect(cleared.memory_controller.provider).toBe("");
    expect(cleared.memory_controller.model).toBe("");
    expect(cleared.memory_controller.reasoning_effort).toBe("");
    expect(settingsRolesConfigFixture.memory_controller.model).toBe("anthropic/claude-haiku-4");
  });
});

describe("buildRolesViewModel", () => {
  it("Should return the six roles in fixed product order regardless of API order", () => {
    const models = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
    expect(models.map(model => model.role)).toEqual([...ROLE_ORDER]);
  });

  it("Should flatten draft values and preserve null effective values without fabrication", () => {
    const models = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
    const controller = models.find(model => model.role === "memory_controller");
    const dream = models.find(model => model.role === "dream");

    expect(controller?.runtime.model).toBe("anthropic/claude-haiku-4");
    expect(controller?.values.timeout).toBe("250ms");
    expect(controller?.effective.timeout).toBe("250ms");
    expect(dream?.effective.model).toBeNull();
  });

  it("Should expose routing state the header reads without inventing a route", () => {
    const models = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
    const controller = models.find(model => model.role === "memory_controller");
    const dream = models.find(model => model.role === "dream");

    expect(controller?.hasRuntimeOverride).toBe(true);
    expect(controller?.supportsAgent).toBe(false);
    expect(dream?.hasRuntimeOverride).toBe(false);
    expect(dream?.routeSummary).toBeNull();
    expect(dream?.supportsAgent).toBe(true);
  });
});

describe("collectRoleValidationErrors", () => {
  it("Should report one error per incomplete fallback route, in product order", () => {
    const config = addFallbackEntry(addFallbackEntry(settingsRolesConfigFixture, "dream"), "dream");
    const errors = collectRoleValidationErrors(config);

    expect(errors.map(error => error.id)).toEqual([
      fallbackFieldId("dream", 0),
      fallbackFieldId("dream", 1),
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
