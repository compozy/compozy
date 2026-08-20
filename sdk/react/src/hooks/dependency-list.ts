import type { DependencyList } from "react";

const objectIds = new WeakMap<object, number>();
let nextObjectId = 1;

export function areDependenciesEqual(left: DependencyList, right: DependencyList): boolean {
  if (left.length !== right.length) return false;
  for (let index = 0; index < left.length; index++) {
    if (!Object.is(left[index], right[index])) return false;
  }
  return true;
}

export function identityCacheKey(dependencies: DependencyList): string {
  return dependencies.map(identityToken).join("\u001f");
}

function identityToken(value: unknown): string {
  if (value === null) return "null";
  const type = typeof value;
  if (type === "undefined") return "undefined";
  if (type === "number" && Number.isNaN(value)) return "NaN";
  if (type === "function" || type === "object") {
    const object = value as object;
    let id = objectIds.get(object);
    if (id === undefined) {
      id = nextObjectId;
      nextObjectId += 1;
      objectIds.set(object, id);
    }
    return `${type}:${id}`;
  }
  return `${type}:${String(value)}`;
}
