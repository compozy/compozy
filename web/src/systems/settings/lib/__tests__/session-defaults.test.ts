import { describe, expect, it, vi } from "vitest";

import {
  combineSettingsSaveSources,
  personaConfigWithRuntime,
  sessionDefaultsRuntimeValue,
  type SettingsSaveSource,
} from "../session-defaults";

function saveSource(overrides: Partial<SettingsSaveSource> = {}): SettingsSaveSource {
  return {
    isDirty: false,
    isSaving: false,
    saveError: null,
    warnings: undefined,
    handleSave: vi.fn(),
    handleReset: vi.fn(),
    ...overrides,
  };
}

describe("session defaults draft", () => {
  it("Should store a runtime choice in the daemon's field order", () => {
    // Invariant: a saved draft equals the reread envelope, so the save bar settles clean.
    const reread = {
      agent: "general",
      provider: "codex",
      model: "gpt-5.6-terra",
      reasoning_effort: "high",
    };
    const draft = personaConfigWithRuntime(
      { agent: "general" },
      { provider: " codex ", model: "gpt-5.6-terra", reasoning_effort: "high" }
    );

    expect(JSON.stringify(draft)).toBe(JSON.stringify(reread));
    expect(sessionDefaultsRuntimeValue(draft)).toEqual({
      provider: "codex",
      model: "gpt-5.6-terra",
      reasoning_effort: "high",
    });
  });

  it("Should keep model and reasoning only under a provider", () => {
    expect(
      personaConfigWithRuntime(
        { agent: "general", provider: "codex", model: "gpt-5.6-terra" },
        { provider: "", model: "gpt-5.6-terra", reasoning_effort: "high" }
      )
    ).toEqual({ agent: "general" });
    expect(
      personaConfigWithRuntime(
        { agent: "general" },
        { provider: "claude", model: "", reasoning_effort: "" }
      )
    ).toEqual({ agent: "general", provider: "claude" });
  });
});

describe("combineSettingsSaveSources", () => {
  it("Should save only dirty drafts and reset every draft", () => {
    const general = saveSource();
    const persona = saveSource({ isDirty: true, warnings: ["restart required"] });
    const save = combineSettingsSaveSources([general, persona]);

    expect(save.isDirty).toBe(true);
    expect(save.warnings).toEqual(["restart required"]);
    save.onSave();
    expect(persona.handleSave).toHaveBeenCalledOnce();
    expect(general.handleSave).not.toHaveBeenCalled();
    save.onReset();
    expect(general.handleReset).toHaveBeenCalledOnce();
    expect(persona.handleReset).toHaveBeenCalledOnce();
  });

  it("Should surface whichever draft failed to save", () => {
    const save = combineSettingsSaveSources([
      saveSource(),
      saveSource({ isDirty: true, saveError: "defaults.model requires defaults.provider" }),
    ]);
    expect(save.error).toBe("defaults.model requires defaults.provider");
    expect(save.warnings).toBeUndefined();
  });
});
