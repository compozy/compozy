import type { CmdPaletteViewFrame, CmdPaletteViewPayload } from "./cmd-palette-types";

type JSONContainer = unknown[] | Record<string, unknown>;

/** Applies the validated RFC 6902 subset accepted by the daemon. */
export function payloadFromProgramFrame(
  current: CmdPaletteViewPayload | null,
  frame: CmdPaletteViewFrame
): CmdPaletteViewPayload {
  if (frame.payload) return structuredClone(frame.payload);
  if (!frame.patch || current === null) {
    throw new Error("The view sent a patch without a base frame.");
  }
  let document: unknown = structuredClone(current);
  for (const operation of frame.patch.ops) {
    document = applyOperation(document, operation.op, operation.path, operation.value);
  }
  if (!isRecord(document) || document.view !== "v1") {
    throw new Error("The view patch did not produce a valid payload.");
  }
  return document as CmdPaletteViewPayload;
}

function applyOperation(document: unknown, op: string, pointer: string, value: unknown): unknown {
  const path = parsePointer(pointer);
  if (op === "test") {
    if (!sameJSON(readAt(document, path), value)) throw new Error("View patch test failed.");
    return document;
  }
  if (path.length === 0) {
    if (op === "remove") return null;
    if (op === "add" || op === "replace") return structuredClone(value);
    throw new Error(`Unsupported view patch operation: ${op}`);
  }
  const { parent, key } = parentAt(document, path);
  if (Array.isArray(parent)) {
    const index = arrayIndex(key, parent.length, op === "add");
    if (op === "add") parent.splice(index, 0, structuredClone(value));
    else if (op === "remove") parent.splice(index, 1);
    else if (op === "replace") parent[index] = structuredClone(value);
    else throw new Error(`Unsupported view patch operation: ${op}`);
    return document;
  }
  if (!isRecord(parent)) throw new Error("View patch target is not a container.");
  if (op === "remove") {
    if (!(key in parent)) throw new Error(`View patch member does not exist: ${key}`);
    delete parent[key];
  } else if (op === "add" || op === "replace") {
    if (op === "replace" && !(key in parent)) {
      throw new Error(`View patch member does not exist: ${key}`);
    }
    parent[key] = structuredClone(value);
  } else {
    throw new Error(`Unsupported view patch operation: ${op}`);
  }
  return document;
}

function readAt(document: unknown, path: readonly string[]): unknown {
  let current = document;
  for (const key of path) {
    if (Array.isArray(current)) current = current[arrayIndex(key, current.length, false)];
    else if (isRecord(current) && key in current) current = current[key];
    else throw new Error(`View patch member does not exist: ${key}`);
  }
  return current;
}

function parentAt(
  document: unknown,
  path: readonly string[]
): { parent: JSONContainer; key: string } {
  const parent = readAt(document, path.slice(0, -1));
  if (!Array.isArray(parent) && !isRecord(parent)) {
    throw new Error("View patch target is not a container.");
  }
  return { parent, key: path.at(-1)! };
}

function parsePointer(pointer: string): string[] {
  if (pointer === "") return [];
  if (!pointer.startsWith("/")) throw new Error("View patch path is not a JSON pointer.");
  return pointer
    .slice(1)
    .split("/")
    .map(part => part.replaceAll("~1", "/").replaceAll("~0", "~"));
}

function arrayIndex(segment: string, length: number, allowAppend: boolean): number {
  if (allowAppend && segment === "-") return length;
  const index = Number(segment);
  if (!Number.isInteger(index) || index < 0 || index >= length + (allowAppend ? 1 : 0)) {
    throw new Error(`Invalid view patch array index: ${segment}`);
  }
  return index;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/** RFC 6902 object equality is member-order independent. */
export function sameJSON(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true;
  if (left === null || right === null) return left === right;
  if (typeof left !== typeof right) return false;
  if (typeof left !== "object" || typeof right !== "object") return left === right;
  if (Array.isArray(left) || Array.isArray(right)) {
    if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) {
      return false;
    }
    return left.every((item, index) => sameJSON(item, right[index]));
  }
  const leftKeys = Object.keys(left);
  const rightRecord = right as Record<string, unknown>;
  const leftRecord = left as Record<string, unknown>;
  if (leftKeys.length !== Object.keys(rightRecord).length) return false;
  return leftKeys.every(key =>
    Object.hasOwn(rightRecord, key) ? sameJSON(leftRecord[key], rightRecord[key]) : false
  );
}
