/**
 * Writes the SDK Reference: one `content/docs/sdk/<resource>.mdx` stub per SDK resource, the
 * section's `meta.json`, and `lib/sdk-reference-data.json` (the model the stubs render). Inputs are
 * the OpenAPI document, the SDK samples, and the TypeScript and Go clients' generated sources.
 */
import path from "node:path";
import { promises as fs } from "node:fs";
import { fileURLToPath } from "node:url";

import {
  SDK_GENERATED_MARKER,
  buildSdkReference,
  renderResourceStub,
  renderSdkMeta,
  type OpenAPIDocument,
} from "../lib/sdk-reference-build";
import type { SDKSampleManifest } from "../lib/sdk-samples";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SITE = path.resolve(HERE, "..");
const REPO_ROOT = path.resolve(SITE, "../..");
const OUT_DIR = path.join(SITE, "content/docs/sdk");
const DATA_FILE = path.join(SITE, "lib/sdk-reference-data.json");
const GO_CLIENT = path.join(REPO_ROOT, "sdk/go/client");
const TS_TYPES = path.join(REPO_ROOT, "sdk/client/src/generated/sdk/types.gen.ts");
const TS_SSE_TYPES = path.join(REPO_ROOT, "sdk/client/src/generated/sse/types.gen.ts");

async function readJson<T>(file: string): Promise<T> {
  return JSON.parse(await fs.readFile(file, "utf8")) as T;
}

async function readGoSource(): Promise<string> {
  const files = (await fs.readdir(GO_CLIENT))
    .filter(file => file.endsWith(".go") && !file.endsWith("_test.go"))
    .sort();
  const sources = await Promise.all(
    files.map(file => fs.readFile(path.join(GO_CLIENT, file), "utf8"))
  );
  return sources.join("\n");
}

/** Removes stubs this generator wrote earlier, so a dropped resource does not leave a page behind. */
async function cleanGenerated(): Promise<void> {
  const entries = await fs.readdir(OUT_DIR);
  const removals: Array<Promise<void>> = [];
  const reads = entries.flatMap(entry =>
    entry.endsWith(".mdx")
      ? [fs.readFile(path.join(OUT_DIR, entry), "utf8").then(text => ({ entry, text }))]
      : []
  );
  for (const { entry, text } of await Promise.all(reads)) {
    if (text.includes(`_generated: ${SDK_GENERATED_MARKER}`)) {
      removals.push(fs.rm(path.join(OUT_DIR, entry), { force: true }));
    }
  }
  await Promise.all(removals);
}

async function main(): Promise<void> {
  const [openapi, samples, goSource, tsTypes, tsSseTypes] = await Promise.all([
    readJson<OpenAPIDocument>(path.join(REPO_ROOT, "openapi/compozy.json")),
    readJson<SDKSampleManifest>(path.join(REPO_ROOT, "openapi/sdk-samples.json")),
    readGoSource(),
    fs.readFile(TS_TYPES, "utf8"),
    fs.readFile(TS_SSE_TYPES, "utf8"),
  ]);
  const reference = buildSdkReference({ openapi, samples, goSource, tsTypes, tsSseTypes });
  await fs.mkdir(OUT_DIR, { recursive: true });
  await cleanGenerated();
  await Promise.all([
    ...reference.resources.map(resource =>
      fs.writeFile(path.join(OUT_DIR, `${resource.slug}.mdx`), renderResourceStub(resource), "utf8")
    ),
    fs.writeFile(path.join(OUT_DIR, "meta.json"), renderSdkMeta(reference), "utf8"),
    fs.writeFile(DATA_FILE, `${JSON.stringify(reference)}\n`, "utf8"),
  ]);
  console.log(
    `[generate-sdk-reference] ${reference.stats.operations} operations on ${reference.stats.resources} resources`
  );
}

main().catch(error => {
  console.error("[generate-sdk-reference] failed:", error);
  process.exit(1);
});
