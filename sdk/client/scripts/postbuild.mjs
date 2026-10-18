import { copyFile, mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const distDir = path.resolve(here, "..", "dist");
const esmDir = path.join(distDir, "esm");
const cjsDir = path.join(distDir, "cjs");

await mkdir(esmDir, { recursive: true });
await mkdir(cjsDir, { recursive: true });

await writeFile(path.join(esmDir, "package.json"), JSON.stringify({ type: "module" }, null, 2));
await writeFile(path.join(cjsDir, "package.json"), JSON.stringify({ type: "commonjs" }, null, 2));

const openapiDir = path.join(distDir, "types", "generated", "openapi");
await mkdir(openapiDir, { recursive: true });
await copyFile(
  path.join(here, "..", "src", "generated", "openapi", "compozy-openapi.d.ts"),
  path.join(openapiDir, "compozy-openapi.d.ts")
);
