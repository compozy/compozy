import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "vite";

// Invariant: published entry points load in ESM/CJS and the main entry bundles
// without Node dependencies, while /node is unavailable to browsers.
// Owner: package distribution; canonical artifact suite: this script.
const root = dirname(dirname(fileURLToPath(import.meta.url)));
const require = createRequire(join(root, "package.json"));
const esm = await import("@compozy/client");
const cjs = require("@compozy/client");
const node = await import("@compozy/client/node");
assert.equal(typeof esm.Compozy.connect, "function");
assert.equal(typeof cjs.Compozy.connect, "function");
assert.equal(typeof node.Compozy.start, "function");

const temporary = await mkdtemp(join(root, ".package-check-"));
const bundle = entry =>
  build({
    configFile: false,
    root: temporary,
    logLevel: "silent",
    build: { lib: { entry, formats: ["es"] }, write: false, minify: false },
  });
try {
  const entry = join(temporary, "entry.ts");
  await writeFile(entry, 'export { Compozy } from "@compozy/client";\n');
  const output = await bundle(entry);
  const chunks = (Array.isArray(output) ? output : [output]).flatMap(result => result.output);
  const code = chunks
    .filter(chunk => chunk.type === "chunk")
    .map(chunk => chunk.code)
    .join("\n");
  assert.doesNotMatch(code, /(?:from\s*|import\s*\(|require\s*\()\s*["']node:/);
  await writeFile(entry, 'export { Compozy } from "@compozy/client/node";\n');
  await assert.rejects(bundle(entry));
  console.log("Client ESM, CJS, and browser package boundaries passed.");
} finally {
  await rm(temporary, { recursive: true, force: true });
}
