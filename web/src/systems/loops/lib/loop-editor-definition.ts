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
