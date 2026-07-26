#!/usr/bin/env node
/*
 * tokens-bare-utilities.mjs
 *
 * Converts Tailwind v4 arbitrary-value token usage `<prefix>-(--<token>)` to
 * the equivalent bare utility `<prefix>-<token>` based on the canonical
 * `@theme` declarations in `packages/ui/src/tokens.css`.
 *
 * Usage:
 *   node scripts/codemod/tokens-bare-utilities.mjs [--write]
 *
 * Algorithm:
 *   1. Parse the `@theme { ... }` block and extract every declared
 *      `--<namespace>-<stem>` (or `--<namespace>`) name.
 *   2. Build OLD→NEW alias map:
 *        --<stem>            ⇢ --color-<stem>   (color namespace)
 *        --<text-x>          ⇢ --text-x         (identity, namespace already
 *                                                 part of name)
 *        --<radius-x>        ⇢ --radius-x       (identity)
 *        --<tracking-x>      ⇢ --tracking-x     (identity)
 *        --<duration-x>      ⇢ --duration-x     (identity)
 *        --<ease-x>          ⇢ --ease-x         (identity)
 *        --<shadow-x>        ⇢ --shadow-x       (identity)
 *      Plus explicit renames the new tokens.css introduced:
 *        --dur               ⇢ --duration-base
 *        --dur-slow          ⇢ --duration-slow
 *        --ease              ⇢ --ease-out
 *        --highlight         ⇢ --shadow-highlight
 *   3. For each `<prefix>-(--<oldname>)` in TS/TSX source:
 *        - Resolve OLD→NEW for `<oldname>` (skip if unknown — token kept
 *          in :root or runtime var injected by Radix/JS).
 *        - Verify `<prefix>` is a valid utility prefix for the NEW token's
 *          namespace (e.g. `text` works for color/text, `bg` only for
 *          color, `duration` only for duration).
 *        - Replace with bare utility `<prefix>-<stem>` (or `<prefix>`
 *          alone when stem is empty, e.g. `rounded-(--radius)` → `rounded`).
 *
 * Safety:
 *   - Replacements happen only when BOTH the old token has a known mapping
 *     AND the prefix matches the namespace. Cross-namespace mismatches
 *     (`text-(--radius)`) are left untouched.
 *   - Component-internal tokens still living in `:root` (--width-modal-*,
 *     --height-pill-group-*, --space-pill-group-*, --width-catalog-logo,
 *     --width-provider-logo-well, --width-pill-group-badge) are NEVER in
 *     the alias map and therefore stay as arbitrary syntax.
 *   - Runtime vars (--anchor-*, --available-*, --accordion-*,
 *     --detail-inspector-*, --radix-*, --overlay-blur) are likewise
 *     untouched.
 *   - Scope: `.tsx` / `.ts` files under `web/src/**`,
 *     `packages/ui/src/**`, and `packages/site/**`. Tests and stories
 *     included (they exercise the same className contract).
 *
 *   Dry-run is the default; pass `--write` to apply.
 */

import fs from "node:fs";
import path from "node:path";
import url from "node:url";

const __filename = url.fileURLToPath(import.meta.url);
const REPO_ROOT = path.resolve(path.dirname(__filename), "..", "..");
const TOKENS_CSS = path.join(REPO_ROOT, "packages", "ui", "src", "tokens.css");
const SITE_GLOBAL_CSS = path.join(REPO_ROOT, "packages", "site", "app", "global.css");

const WRITE = process.argv.includes("--write");

// Tailwind v4 utility prefixes per theme namespace.
// `text` lives in both `color` and `text` namespaces (Tailwind chooses based
// on the property; here we accept either when checking compatibility).
const PREFIXES_BY_NAMESPACE = {
  color: [
    "text",
    "bg",
    "border",
    "border-t",
    "border-r",
    "border-b",
    "border-l",
    "border-x",
    "border-y",
    "border-s",
    "border-e",
    "outline",
    "ring",
    "ring-offset",
    "divide",
    "fill",
    "stroke",
    "accent",
    "caret",
    "decoration",
    "placeholder",
    "from",
    "via",
    "to",
  ],
  text: ["text"],
  font: ["font"],
  "font-weight": ["font"],
  tracking: ["tracking"],
  leading: ["leading"],
  radius: ["rounded"],
  shadow: ["shadow"],
  "inset-shadow": ["inset-shadow"],
  "drop-shadow": ["drop-shadow"],
  duration: ["duration"],
  ease: ["ease"],
  container: ["max-w", "min-w", "w"],
};

// Namespaces sorted longest-first so multi-segment prefixes match before
// shorter ones (font-weight before font).
const NAMESPACES_SORTED = Object.keys(PREFIXES_BY_NAMESPACE).sort((a, b) => b.length - a.length);

// Explicit renames the new tokens.css introduced.
const EXPLICIT_RENAMES = new Map([
  ["dur", "duration-base"],
  ["dur-slow", "duration-slow"],
  ["ease", "ease-out"],
  ["highlight", "shadow-highlight"],
]);

/**
 * Parses the `@theme { ... }` block and returns a Map keyed by the FULL
 * declared name (without leading `--`). Each value is { namespace, stem }.
 */
function parseTheme(cssSource) {
  const themeBlocks = [...cssSource.matchAll(/@theme(?:\s+inline)?\s*\{([\s\S]*?)\n\}/g)].map(
    match => match[1]
  );
  if (themeBlocks.length === 0) {
    throw new Error("Could not locate any `@theme { ... }` or `@theme inline { ... }` blocks");
  }
  const body = themeBlocks.join("\n");
  const declRegex = /--([a-zA-Z0-9-]+)\s*:/g;
  const result = new Map();
  for (const m of body.matchAll(declRegex)) {
    const name = m[1];
    // Skip CSS-only companion properties like `--text-eyebrow--line-height`.
    if (name.endsWith("--line-height")) continue;
    let namespace = null;
    let stem = "";
    for (const ns of NAMESPACES_SORTED) {
      if (name === ns) {
        namespace = ns;
        stem = "";
        break;
      }
      if (name.startsWith(`${ns}-`)) {
        namespace = ns;
        stem = name.slice(ns.length + 1);
        break;
      }
    }
    if (!namespace) continue; // not a namespaced token
    result.set(name, { namespace, stem });
  }
  return result;
}

/**
 * Build OLD → { newFullName, namespace, stem } lookup.
 *
 * For each token `--<ns>-<stem>` in @theme:
 *   - Add identity mapping: `<ns>-<stem>` (or `<ns>` if stem empty)
 *   - For color namespace ONLY: also add bare-stem alias `<stem>`
 *     (old code referenced colors as `--fg`, `--muted` without prefix).
 *
 * Plus explicit renames from EXPLICIT_RENAMES.
 */
function buildAliasMap(themeTokens) {
  const aliases = new Map();
  for (const [fullName, info] of themeTokens) {
    // Identity
    aliases.set(fullName, { newFullName: fullName, ...info });
    // Color short-alias
    if (info.namespace === "color" && info.stem) {
      // Only alias to the bare stem if the stem doesn't itself collide with
      // another namespace prefix (avoid `--text` collisions etc.).
      const collidesWithOtherNs = NAMESPACES_SORTED.some(
        ns => ns !== "color" && (info.stem === ns || info.stem.startsWith(`${ns}-`))
      );
      if (!collidesWithOtherNs) {
        if (!aliases.has(info.stem)) {
          aliases.set(info.stem, { newFullName: fullName, ...info });
        }
      }
    }
  }
  // Explicit renames: <oldStem> ⇢ <newFullName>
  for (const [oldStem, newFullName] of EXPLICIT_RENAMES) {
    const info = themeTokens.get(newFullName);
    if (info) aliases.set(oldStem, { newFullName, ...info });
  }
  return aliases;
}

function isPrefixValidForNamespace(prefix, namespace) {
  const list = PREFIXES_BY_NAMESPACE[namespace];
  if (!list) return false;
  if (list.includes(prefix)) return true;
  // `text` prefix is also valid for `color` namespace (text color).
  if (prefix === "text" && namespace === "color") return true;
  return false;
}

function buildBareUtility(prefix, namespace, stem) {
  // Empty stem (e.g. `--radius` itself) → use prefix alone.
  if (stem === "") return prefix;
  return `${prefix}-${stem}`;
}

function rewriteSource(source, aliasMap) {
  // Pattern: a Tailwind-class-like prefix (1-3 hyphenated segments) followed
  // by `-(--<name>)`. The prefix is "<seg>(-<seg>){0,2}" to allow `min-h`,
  // `max-w`, `border-x` style multi-segment prefixes.
  const pattern = /\b([a-zA-Z][a-zA-Z0-9]*(?:-[a-zA-Z][a-zA-Z0-9]*){0,2})-\(--([a-zA-Z0-9-]+)\)/g;
  const counts = new Map();
  const sample = new Map();
  const out = source.replace(pattern, (full, prefix, oldName) => {
    const alias = aliasMap.get(oldName);
    if (!alias) return full;
    if (!isPrefixValidForNamespace(prefix, alias.namespace)) return full;
    const utility = buildBareUtility(prefix, alias.namespace, alias.stem);
    const key = `${full}→${utility}`;
    counts.set(key, (counts.get(key) || 0) + 1);
    if (!sample.has(key)) sample.set(key, { from: full, to: utility });
    return utility;
  });
  const replaced = out !== source;
  const changes = [...counts.entries()].map(([key, count]) => ({
    ...sample.get(key),
    count,
  }));
  return { replaced, output: out, changes };
}

function walkDir(dir, results = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (["node_modules", ".turbo", "dist", "storybook-static"].includes(entry.name)) continue;
      walkDir(path.join(dir, entry.name), results);
    } else if (entry.isFile() && /\.(tsx?|jsx?)$/.test(entry.name)) {
      results.push(path.join(dir, entry.name));
    }
  }
  return results;
}

function main() {
  const themeFiles = [TOKENS_CSS, SITE_GLOBAL_CSS].filter(file => fs.existsSync(file));
  const cssSource = themeFiles.map(file => fs.readFileSync(file, "utf8")).join("\n");
  const themeTokens = parseTheme(cssSource);
  const aliasMap = buildAliasMap(themeTokens);

  console.log(`Parsed ${themeTokens.size} tokens from ${themeFiles.length} @theme source file(s).`);
  console.log(`Built ${aliasMap.size} OLD→NEW alias entries.\n`);

  const targets = [
    path.join(REPO_ROOT, "web", "src"),
    path.join(REPO_ROOT, "packages", "ui", "src"),
    path.join(REPO_ROOT, "packages", "site"),
  ];
  const files = targets.flatMap(dir => (fs.existsSync(dir) ? walkDir(dir) : []));
  console.log(`Scanning ${files.length} TS/TSX files…\n`);

  const aggregateCounts = new Map();
  const aggregateSample = new Map();
  let modifiedFiles = 0;
  let totalReplacements = 0;

  for (const file of files) {
    const source = fs.readFileSync(file, "utf8");
    const { replaced, output, changes } = rewriteSource(source, aliasMap);
    if (!replaced) continue;
    modifiedFiles += 1;
    for (const { from, to, count } of changes) {
      const key = `${from}→${to}`;
      aggregateCounts.set(key, (aggregateCounts.get(key) || 0) + count);
      totalReplacements += count;
      if (!aggregateSample.has(key)) aggregateSample.set(key, { from, to });
    }
    if (WRITE) fs.writeFileSync(file, output, "utf8");
  }

  console.log(`${modifiedFiles} file(s) ${WRITE ? "modified" : "would be modified"}.`);
  console.log(`${totalReplacements} replacement(s) ${WRITE ? "applied" : "queued"}.\n`);
  const sorted = [...aggregateCounts.entries()].sort((a, b) => b[1] - a[1]);
  console.log("Top conversions:");
  for (const [key, count] of sorted.slice(0, 40)) {
    const { from, to } = aggregateSample.get(key);
    console.log(`  ${String(count).padStart(5)}  ${from}  →  ${to}`);
  }
  if (sorted.length > 40) console.log(`  … and ${sorted.length - 40} more`);
  console.log("");
  if (!WRITE) console.log("(dry-run — no files written; pass --write to apply)");
}

main();
