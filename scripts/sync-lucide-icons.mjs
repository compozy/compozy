#!/usr/bin/env bun
// Generates the Lucide icon-slug allowlist embedded by internal/profile.
//
// The web picker offers the full Lucide catalog and renders profile icons from
// the lucide-static sprite; the daemon must refuse any slug that sprite cannot
// render, so both sides read the same lucide-static release (pinned in bun.lock).
import { readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join } from "node:path";

const root = process.cwd();
const args = new Set(process.argv.slice(2));
const outputPath = join(root, "internal/profile/lucide_icons.gen.txt");

const resolve = createRequire(join(root, "web/package.json")).resolve;
const tags = JSON.parse(readFileSync(resolve("lucide-static/tags.json"), "utf8"));
const slugs = Object.keys(tags).sort();
if (slugs.length === 0) throw new Error("lucide-static/tags.json contains no icons");
const next = slugs.join("\n") + "\n";
const current = readCurrent();

if (args.has("--write")) {
  if (next !== current) writeFileSync(outputPath, next);
  process.exit(0);
}
if (next === current) process.exit(0);
process.stdout.write(
  "internal/profile/lucide_icons.gen.txt is out of sync with lucide-static.\n" +
    "Run make codegen or scripts/sync-lucide-icons.mjs --write.\n" +
    describeDrift(current, next)
);
if (args.has("--check")) process.exit(1);

function readCurrent() {
  try {
    return readFileSync(outputPath, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return "";
    throw error;
  }
}

function describeDrift(before, after) {
  const parse = text => new Set(text.split("\n").filter(Boolean));
  const previous = parse(before);
  const upcoming = parse(after);
  const added = [...upcoming].filter(slug => !previous.has(slug));
  const removed = [...previous].filter(slug => !upcoming.has(slug));
  return (
    (added.length > 0 ? "+ " + added.join("\n+ ") + "\n" : "") +
    (removed.length > 0 ? "- " + removed.join("\n- ") + "\n" : "")
  );
}
