#!/usr/bin/env bun
// Generates the tailwind-merge font-size class group consumed by `cn`.
//
// tailwind-merge classifies an unrecognized `text-*` class as a text COLOR, so a
// project font-size utility paired with a color utility inside the same `cn()`
// call is treated as a conflict and silently dropped. Literal entries registered
// in the `font-size` group resolve ahead of the color validator, so the list must
// cover every `--text-*` theme token both surfaces declare.
//
// It also emits the project radius scale: tailwind-merge only knows Tailwind's
// default radius names, so `rounded-pill` and `rounded-xs` in one `cn()` call
// would both survive and the winner would be CSS source order, not the caller.
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const args = new Set(process.argv.slice(2));
const outputPath = join(root, "packages/ui/src/lib/font-size-classes.generated.ts");
const sources = ["packages/ui/src/tokens.css", "packages/site/app/global.css"];

const classes = sources.flatMap(source =>
  fontSizeClasses(readFileSync(join(root, source), "utf8"))
);
const unique = [...new Set(classes)].sort();
if (unique.length === 0) throw new Error("No --text-* theme tokens found in " + sources.join(", "));
const radii = [
  ...new Set(sources.flatMap(source => radiusStems(readFileSync(join(root, source), "utf8")))),
].sort();
const next = emit(unique, radii);
const current = readCurrent();

if (args.has("--write")) {
  if (next !== current) writeFileSync(outputPath, next);
  process.exit(0);
}
if (next === current) process.exit(0);
process.stdout.write(
  "packages/ui/src/lib/font-size-classes.generated.ts is out of sync with the CSS theme tokens.\n" +
    "Run make codegen or scripts/sync-font-size-classes.mjs --write.\n" +
    describeDrift(current, next)
);
if (args.has("--check")) process.exit(1);

/** Collects `text-<stem>` class names from every `@theme` block in a stylesheet. */
function fontSizeClasses(css) {
  const blocks = [...css.matchAll(/@theme(?:\s+inline)?\s*\{([\s\S]*?)\n\}/g)];
  return blocks.flatMap(block =>
    Array.from(block[1].matchAll(/--text-([a-zA-Z0-9-]+?)\s*:/g), match => match[1])
      .filter(stem => !stem.endsWith("--line-height"))
      .map(stem => "text-" + stem)
  );
}

/** Collects the `<stem>` of every `--radius-<stem>` token in a stylesheet's `@theme` blocks. */
function radiusStems(css) {
  const blocks = [...css.matchAll(/@theme(?:\s+inline)?\s*\{([\s\S]*?)\n\}/g)];
  return blocks.flatMap(block =>
    Array.from(block[1].matchAll(/--radius-([a-zA-Z0-9-]+?)\s*:/g), match => match[1])
  );
}

function emit(names, radii) {
  return (
    [
      "// Generated from:",
      ...sources.map(source => "//   " + source),
      "// by scripts/sync-font-size-classes.mjs.",
      "// Do not edit by hand. Run make codegen to refresh.",
      "//",
      "// tailwind-merge reads an unrecognized `text-*` class as a color. Without these",
      "// literals a font-size utility loses to a color utility in the same `cn()` call.",
      "export const fontSizeClasses = [",
      ...names.map(name => '  "' + name + '",'),
      "];",
      "",
      "// tailwind-merge only knows Tailwind's default radius names. Registering the",
      "// project scale lets a caller's `rounded-*` replace a variant's `rounded-pill`.",
      "export const radiusScale = [",
      ...radii.map(name => '  "' + name + '",'),
      "];",
    ].join("\n") + "\n"
  );
}

function readCurrent() {
  try {
    return readFileSync(outputPath, "utf8");
  } catch (error) {
    if (error.code === "ENOENT") return "";
    throw error;
  }
}

function describeDrift(before, after) {
  const parse = text => new Set(Array.from(text.matchAll(/^ {2}"([a-zA-Z0-9-]+)",$/gm), m => m[1]));
  const previous = parse(before);
  const upcoming = parse(after);
  const added = [...upcoming].filter(name => !previous.has(name));
  const removed = [...previous].filter(name => !upcoming.has(name));
  return (
    (added.length > 0 ? "+ " + added.join("\n+ ") + "\n" : "") +
    (removed.length > 0 ? "- " + removed.join("\n- ") + "\n" : "")
  );
}
