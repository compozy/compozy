import type { OsWindowRoute } from "./os-types";

export function sameOsWindowRoute(
  left: OsWindowRoute | null,
  right: OsWindowRoute | null
): boolean {
  if (left === right) return true;
  if (left === null || right === null || left.pathname !== right.pathname) return false;
  return sameJsonValue(left.search, right.search);
}

function sameJsonValue(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) return false;
    return left.every((value, index) => sameJsonValue(value, right[index]));
  }
  if (!isJsonObject(left) || !isJsonObject(right)) return false;

  const leftKeys = definedKeys(left);
  const rightKeys = definedKeys(right);
  if (leftKeys.length !== rightKeys.length) return false;
  return leftKeys.every(
    (key, index) => key === rightKeys[index] && sameJsonValue(left[key], right[key])
  );
}

function isJsonObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function definedKeys(value: Record<string, unknown>): string[] {
  return Object.keys(value)
    .filter(key => value[key] !== undefined)
    .sort();
}
