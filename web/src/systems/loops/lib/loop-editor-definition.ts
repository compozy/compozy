import { setAtPath, unsetAtPath } from "./loop-editor-draft";
import type { FieldPath } from "./loop-node-schema-types";
import type { LoopDefinition, LoopDetail } from "../types";

export type EditableLoopContractField = "goal" | "definition_of_done";

/** Seed editor state with the authoritative top-level version used by PATCH CAS. */
export function editorDefinitionFromLoop(loop: LoopDetail): LoopDefinition {
  return {
    ...loop.definition,
    meta: { ...loop.definition.meta, version: loop.version },
  };
}

/** Replace one authorable contract field without disturbing the rest of the definition. */
export function withLoopContractField(
  definition: LoopDefinition,
  field: EditableLoopContractField,
  value: string
): LoopDefinition {
  return {
    ...definition,
    contract: { ...definition.contract, [field]: value },
  };
}

/**
 * Writes one path inside the Loop contract, leaving everything else verbatim. The seven
 * terminal reaction lists (`contract.on_done` … `on_canceled`) are authored through this rather
 * than the per-node draft setter, and clearing prunes the emptied key so an unauthored terminal
 * publishes nothing instead of `on_done: []`.
 */
export function withLoopContractPath(
  definition: LoopDefinition,
  path: FieldPath,
  value: unknown
): LoopDefinition {
  const contract =
    value === undefined
      ? unsetAtPath(definition.contract, path)
      : setAtPath(definition.contract, path, value);
  if (contract === definition.contract) return definition;
  return { ...definition, contract };
}
