import { mkdir, writeFile } from "node:fs/promises";
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
