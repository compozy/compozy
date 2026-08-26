#!/usr/bin/env bun
// Mirrors the Emojibase JSON the identity emoji picker fetches into web/public,
// so Vite serves it natively in dev and build with no CDN and no copy plugin.
// The mirrored files must match the emojibase-data release pinned in bun.lock.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const root = process.cwd();
const args = new Set(process.argv.slice(2));
const resolve = createRequire(join(root, "web/package.json")).resolve;
const files = [
  { source: "emojibase-data/en/data.json", output: "web/public/vendor/emojibase/en/data.json" },
  {
    source: "emojibase-data/en/messages.json",
    output: "web/public/vendor/emojibase/en/messages.json",
  },
];

let drifted = [];
for (const file of files) {
  const next = readFileSync(resolve(file.source), "utf8");
  const outputPath = join(root, file.output);
  if (readCurrent(outputPath) === next) continue;
  drifted.push(file.output);
  if (args.has("--write")) {
    mkdirSync(dirname(outputPath), { recursive: true });
    writeFileSync(outputPath, next);
  }
}

if (args.has("--write") || drifted.length === 0) process.exit(0);
process.stdout.write(
  "web/public/vendor/emojibase is out of sync with emojibase-data:\n" +
    drifted.map(file => "  " + file + "\n").join("") +
    "Run make codegen or scripts/sync-emojibase.mjs --write.\n"
);
if (args.has("--check")) process.exit(1);

function readCurrent(path) {
  try {
    return readFileSync(path, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return "";
    throw error;
  }
}
