/**
 * The repository's comment store: `web/gallery/comments/`, with keys checked against discovery:
 * a prototype slug, or `flow-<id>` for a journey flow that resolves without problems.
 */
import { join } from "node:path";

import { flowOfKey } from "../flow.ts";
import { keyPattern } from "../shell/comments/model.ts";
import { CommentError, CommentStore } from "./comments-store.ts";
import { findPrototypes } from "./discovery.ts";
import { findFlows } from "./flows.ts";
import { galleryRoot, repoRoot, webRoot } from "./paths.ts";
import { findSpecs } from "./specs.ts";

export const commentsDirectory = join(galleryRoot, "comments");

type Catalog = {
  specs: ReturnType<typeof findSpecs>;
  prototypes: ReturnType<typeof findPrototypes>["entries"];
  flows?: ReturnType<typeof findFlows>["flows"];
};

/**
 * Discovery parses every prototype with the TypeScript compiler, and one store change checks its
 * key several times; a result this young is reused. Kept short so a prototype just added is not
 * rejected for long.
 */
const catalogTtlMs = 1000;
let cached: { at: number; value: Catalog } | undefined;

function catalog(): Catalog {
  const now = Date.now();
  if (cached && now - cached.at <= catalogTtlMs) return cached.value;
  const specs = findSpecs(repoRoot);
  const prototypes = findPrototypes(webRoot, repoRoot, specs).entries;
  cached = { at: now, value: { specs, prototypes } };
  return cached.value;
}

function flowsOf(value: Catalog) {
  value.flows ??= findFlows(webRoot, repoRoot, value.prototypes, value.specs).flows;
  return value.flows;
}

export function assertCommentKey(key: string) {
  if (!keyPattern.test(key)) throw new CommentError(`Invalid comment key "${key}"`, 400);
  const value = catalog();
  const { prototypes } = value;
  const flow = flowOfKey(key);
  if (flow !== null) {
    const flows = flowsOf(value);
    if (!flows.some(entry => entry.id === flow)) {
      throw new CommentError(`Unknown flow "${flow}"`, 404);
    }
    return;
  }
  // A prototype without valid metadata never renders in the gallery, so it takes no comments.
  if (!prototypes.some(entry => entry.slug === key && entry.meta)) {
    throw new CommentError(`Unknown prototype "${key}"`, 404);
  }
}

export function localCommentStore() {
  return new CommentStore({ directory: commentsDirectory, assertKey: assertCommentKey, repoRoot });
}
