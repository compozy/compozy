import type { RuntimeACPOption, RuntimeACPOptionSelection } from "./types";

const DEDICATED_OPTION_IDS = new Set(["model", "reasoning_effort", "effort", "speed", "fast"]);

function sortByID<T extends { id: string }>(values: readonly T[]): T[] {
  return [...values].sort((left, right) => left.id.localeCompare(right.id));
}

export function isAdvancedRuntimeOption(option: RuntimeACPOption): boolean {
  return !DEDICATED_OPTION_IDS.has(option.id.trim().toLowerCase());
}

export function advancedRuntimeOptions(
  options: readonly RuntimeACPOption[] | undefined
): RuntimeACPOption[] {
  if (!options || options.length === 0) return [];
  const seen = new Set<string>();
  return sortByID(
    options.filter(option => {
      const id = option.id.trim();
      if (id.length === 0 || seen.has(id) || !isAdvancedRuntimeOption(option)) return false;
      seen.add(id);
      return true;
    })
  );
}

/**
 * Keep only explicit selections that the current public ACP descriptors can
 * honor. An empty result is omitted so provider defaults remain distinguishable
 * from an explicit override.
 */
export function sanitizeRuntimeACPSelections(
  selections: readonly RuntimeACPOptionSelection[] | undefined,
  options: readonly RuntimeACPOption[] | undefined
): RuntimeACPOptionSelection[] | undefined {
  if (!selections || selections.length === 0) return undefined;
  if (!options) return sortByID(selections);

  const optionsByID = new Map<string, RuntimeACPOption>();
  for (const option of options) {
    if (option.id.trim().length > 0) optionsByID.set(option.id, option);
  }
  const seen = new Set<string>();
  const valid: RuntimeACPOptionSelection[] = [];
  for (const selection of selections) {
    const id = selection.id.trim();
    const option = optionsByID.get(id);
    if (!option || seen.has(id)) continue;
    if (option.kind === "select") {
      const valueID = selection.value_id?.trim();
      if (!valueID || !option.values?.some(value => value.value === valueID)) continue;
      valid.push({ id, value_id: valueID });
    } else if (option.kind === "boolean" && typeof selection.bool_value === "boolean") {
      valid.push({ id, bool_value: selection.bool_value });
    } else {
      continue;
    }
    seen.add(id);
  }
  return valid.length > 0 ? sortByID(valid) : undefined;
}

export function setRuntimeACPSelection(
  selections: readonly RuntimeACPOptionSelection[] | undefined,
  next: RuntimeACPOptionSelection | null,
  options: readonly RuntimeACPOption[] | undefined
): RuntimeACPOptionSelection[] | undefined {
  const current = selections ? [...selections] : [];
  const nextID = next?.id.trim() ?? "";
  const withoutNext = current.filter(selection => selection.id !== nextID);
  if (next && nextID.length > 0) withoutNext.push({ ...next, id: nextID });
  return sanitizeRuntimeACPSelections(withoutNext, options);
}
