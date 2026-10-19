import { isReasoningEffort } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";

import type { SettingsPersonaSection } from "../types";

type PersonaConfig = SettingsPersonaSection["config"];

/** The saved `[defaults]` runtime as selector intent; absent fields read as "use the default". */
export function sessionDefaultsRuntimeValue(config: PersonaConfig): RuntimeSelectorValue {
  const effort = config.reasoning_effort?.trim() ?? "";
  return {
    provider: config.provider?.trim() ?? "",
    model: config.model?.trim() ?? "",
    reasoning_effort: isReasoningEffort(effort) ? effort : "",
  };
}

/**
 * Applies a runtime choice to the persona draft. Fields are rebuilt in the
 * daemon's order and empty ones stay absent, so a saved draft compares equal
 * to the reread envelope. Model and reasoning only exist under a provider.
 */
export function personaConfigWithRuntime(
  config: PersonaConfig,
  runtime: RuntimeSelectorValue
): PersonaConfig {
  const provider = runtime.provider.trim();
  const model = provider ? runtime.model.trim() : "";
  const reasoning = provider ? runtime.reasoning_effort : "";
  return {
    agent: config.agent,
    ...(provider ? { provider } : {}),
    ...(model ? { model } : {}),
    ...(reasoning ? { reasoning_effort: reasoning } : {}),
  };
}

export interface SettingsSaveSource {
  isDirty: boolean;
  isSaving: boolean;
  saveError: string | null;
  warnings: string[] | undefined;
  handleSave: () => void;
  handleReset: () => void;
}

/**
 * One save bar over several drafts that save to different scopes. Save writes
 * only the dirty ones; Reset discards all of them.
 */
export function combineSettingsSaveSources(sources: readonly SettingsSaveSource[]) {
  const warnings = sources.flatMap(source => source.warnings ?? []);
  return {
    isDirty: sources.some(source => source.isDirty),
    isSaving: sources.some(source => source.isSaving),
    error: sources.find(source => source.saveError !== null)?.saveError ?? null,
    warnings: warnings.length > 0 ? warnings : undefined,
    onSave: () => {
      for (const source of sources) if (source.isDirty) source.handleSave();
    },
    onReset: () => {
      for (const source of sources) source.handleReset();
    },
  };
}
