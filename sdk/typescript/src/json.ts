import type { JSONValue } from "./types.js";

export function cloneJSON(value: JSONValue): JSONValue {
  if (value === null || typeof value !== "object") {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map(item => cloneJSON(item));
  }
  return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, cloneJSON(entry)]));
}
