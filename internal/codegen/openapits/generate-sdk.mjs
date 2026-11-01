import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { createClient } from "@hey-api/openapi-ts";
import ts from "typescript";

const runtimeArgs = process.argv.slice(1);
const args = runtimeArgs[0]?.endsWith("generate-sdk.mjs") ? runtimeArgs.slice(1) : runtimeArgs;
const [input, configPath, output] = args;
if (!input || !configPath || !output) {
  throw new Error("usage: generate-sdk.mjs <openapi-path> <config-path> <output-directory>");
}
const { createSDKConfigs, sdkOperations } = await import(pathToFileURL(resolve(configPath)).href);
const configs = createSDKConfigs(resolve(input), resolve(output));
for (const config of configs) await createClient(config);

const operations = sdkOperations(resolve(input));
for (const [job, isStream] of [
  ["sdk", false],
  ["sse", true],
]) {
  const streamPath = resolve(output, job, "core/serverSentEvents.gen.ts");
  const streamSource = await readFile(streamPath, "utf8");
  const cancellation = "            void reader.cancel().catch(() => {});";
  if (streamSource.split(cancellation).length !== 2) {
    throw new Error(`SDK job ${job} is missing the patched stream cancellation template`);
  }
  const source = await readFile(resolve(output, job, "sdk.gen.ts"), "utf8");
  const types = await readFile(resolve(output, job, "types.gen.ts"), "utf8");
  const names = new Map();
  const ast = ts.createSourceFile("sdk.gen.ts", source, ts.ScriptTarget.Latest, true);
  function visit(node) {
    if (
      (ts.isMethodDeclaration(node) || ts.isVariableDeclaration(node)) &&
      node.name &&
      ts.isIdentifier(node.name)
    ) {
      const declaration = ts.isVariableDeclaration(node) ? node.parent.parent : node;
      names.set(node.name.text, declaration.getFullText(ast));
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
  for (const operation of operations) {
    const expected = !operation.prompt && !operation.websocket && operation.stream === isStream;
    if (names.has(operation.id) !== expected) {
      throw new Error(
        `SDK job ${job} ${expected ? "omitted" : "unexpectedly includes"} ${operation.id}`
      );
    }
    if (
      expected &&
      operation.stability &&
      !names.get(operation.id).includes(`@stability ${operation.stability}`)
    ) {
      throw new Error(`SDK operation ${operation.id} lost its stability documentation`);
    }
    if (
      expected &&
      !types.includes(`export type ${operation.id[0].toUpperCase()}${operation.id.slice(1)}Data =`)
    ) {
      throw new Error(`SDK operation ${operation.id} lost its operation-id type name`);
    }
  }
  if (/export type (Get|Post|Put|Patch|Delete)Api/.test(types)) {
    throw new Error(`SDK job ${job} generated path-based type names`);
  }
}
