#!/usr/bin/env bun
// Mirrors the Emojibase JSON the identity emoji picker fetches into web/public,
// so Vite serves it natively in dev and build with no CDN and no copy plugin.
// The mirror must carry the same DATA as the emojibase-data release pinned in
// bun.lock; comparison is semantic because the repo formatter owns the bytes.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const root = process.cwd();
const args = new Set(process.argv.slice(2));
const resolve = createRequire(join(root, "packages/ui/package.json")).resolve;
const files = [
  { source: "emojibase-data/en/data.json", output: "web/public/assets/emojibase/en/data.json" },
  {
    source: "emojibase-data/en/messages.json",
    output: "web/public/assets/emojibase/en/messages.json",
  },
];

const drifted = [];
for (const file of files) {
  const next = readFileSync(resolve(file.source), "utf8");
  const outputPath = join(root, file.output);
  if (sameData(readCurrent(outputPath), next)) continue;
  drifted.push(file.output);
  if (args.has("--write")) {
    mkdirSync(dirname(outputPath), { recursive: true });
    writeFileSync(outputPath, next);
  }
}

if (args.has("--write") || drifted.length === 0) process.exit(0);
process.stdout.write(
  "web/public/assets/emojibase is out of sync with emojibase-data:\n" +
    drifted.map(file => "  " + file + "\n").join("") +
    "Run make codegen or scripts/sync-emojibase.mjs --write.\n"
);
if (args.has("--check")) process.exit(1);

function sameData(current, next) {
  if (current === "") return false;
  try {
    return JSON.stringify(JSON.parse(current)) === JSON.stringify(JSON.parse(next));
  } catch {
    return false;
  }
}

function readCurrent(path) {
  try {
    return readFileSync(path, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return "";
    throw error;
  }
}
