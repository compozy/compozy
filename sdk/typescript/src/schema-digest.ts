import { createHash } from "node:crypto";

import type { JSONValue } from "./base-types.js";

export function canonicalJSON(value: JSONValue): string {
  return appendCanonicalValue(value);
}

export function schemaDigest(schema: JSONValue): string {
  if (!isJSONObject(schema)) {
    throw new Error("schema must be a JSON object");
  }
  return createHash("sha256").update(canonicalJSON(schema)).digest("hex");
}

function appendCanonicalValue(value: JSONValue): string {
  if (value === null) {
    return "null";
  }
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (typeof value === "string") {
    return JSON.stringify(value);
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) {
      throw new Error("number must be finite");
    }
    if (Object.is(value, -0) || value === 0) {
      return "0";
    }
    return String(value).replace("e+", "e");
  }
  if (Array.isArray(value)) {
    return `[${value.map(item => appendCanonicalValue(item)).join(",")}]`;
  }
  const keys = Object.keys(value).sort();
  return `{${keys
    .map(key => `${JSON.stringify(key)}:${appendCanonicalValue(value[key] as JSONValue)}`)
    .join(",")}}`;
}

function isJSONObject(value: JSONValue): value is { [key: string]: JSONValue } {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
