#!/usr/bin/env node
// detect-token-drift.mjs — scan source files for raw color/spacing values that bypass the token system.
// Zero-dependency. Walks one or more directories, reports raw hex / rgb / hsl / oklch usages
// outside files designated as token sources.
//
// Usage:
//   node detect-token-drift.mjs <dir> [<dir>...] [--token-paths "tokens,theme,variables.css"]
//
// Exit codes: 0 = no drift; 1 = drift found; 2 = bad input.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, sep, relative } from "node:path";
import { argv, exit } from "node:process";

const DEFAULT_TOKEN_PATH_PARTS = ["tokens", "theme", "themes", "variables.css", "design-tokens"];
const DEFAULT_EXT = new Set([
  ".css", ".scss", ".sass", ".less",
  ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
  ".vue", ".svelte", ".astro",
  ".html",
]);
const SKIP_DIRS = new Set([
  "node_modules", ".git", "dist", "build", ".next", ".turbo", ".cache",
  "coverage", "out", ".output", ".vercel", "storybook-static",
]);

// Allow a small set of legitimate raw values to avoid noise on the common cases.
// transparent / currentColor / 0 / white-mix() variables are always permitted.
const ALLOWED_LITERAL = new Set([
  "transparent", "currentcolor", "inherit", "initial", "unset", "none",
]);

// Patterns to flag.
const HEX_RE = /(?<![\w-])#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})\b/g;
const RGB_RE = /\brgba?\([^)]+\)/gi;
const HSL_RE = /\bhsla?\([^)]+\)/gi;
const OKLCH_RE = /\boklch\([^)]+\)/gi;
const OKLAB_RE = /\boklab\([^)]+\)/gi;
const LAB_RE = /\blab\([^)]+\)/gi;
const LCH_RE = /\blch\([^)]+\)/gi;

const SUGGESTIONS = {
  hex: "Replace with a semantic color token (e.g. var(--color-bg-elevated)).",
  rgb: "Replace with a semantic color token; if alpha is needed, prefer color-mix(in oklch, var(--token) X%, transparent).",
  hsl: "Replace with a semantic color token; prefer OKLCH in primitive layer.",
  oklch: "Allowed only inside the primitive/theme layer — move out of components.",
  oklab: "Allowed only inside the primitive/theme layer — move out of components.",
  lab: "Allowed only inside the primitive/theme layer — move out of components.",
  lch: "Allowed only inside the primitive/theme layer — move out of components.",
};

function parseArgs(args) {
  const out = { dirs: [], tokenParts: DEFAULT_TOKEN_PATH_PARTS.slice(), help: false };
  for (let i = 2; i < args.length; i++) {
    const a = args[i];
    if (a === "--token-paths") {
      const v = args[++i] || "";
      out.tokenParts = v.split(",").map(s => s.trim()).filter(Boolean);
    } else if (a === "-h" || a === "--help") {
      out.help = true;
    } else {
      out.dirs.push(a);
    }
  }
  return out;
}

function help() {
  console.log(`detect-token-drift.mjs — find raw color values bypassing tokens.

Usage:
  node detect-token-drift.mjs <dir> [<dir>...] [--token-paths "tokens,theme,variables.css"]

Flags:
  --token-paths   Comma-separated path fragments that mark token-definition files.
                  Files matching these are skipped (they ARE the tokens).
                  Default: ${DEFAULT_TOKEN_PATH_PARTS.join(", ")}

Examples:
  node detect-token-drift.mjs src/
  node detect-token-drift.mjs apps/web/src packages/ui --token-paths "tokens,theme"

Exit codes: 0 = clean; 1 = drift found; 2 = bad input.`);
}

function shouldSkipDir(name) {
  return SKIP_DIRS.has(name) || name.startsWith(".");
}

function isTokenFile(filePath, tokenParts) {
  const norm = filePath.split(sep).join("/").toLowerCase();
  return tokenParts.some(p => norm.includes(`/${p.toLowerCase()}/`) || norm.endsWith(`/${p.toLowerCase()}`));
}

function* walk(dir) {
  let entries;
  try { entries = readdirSync(dir); } catch { return; }
  for (const name of entries) {
    if (shouldSkipDir(name)) continue;
    const full = join(dir, name);
    let st;
    try { st = statSync(full); } catch { continue; }
    if (st.isDirectory()) {
      yield* walk(full);
    } else if (st.isFile()) {
      yield full;
    }
  }
}

function extOf(p) {
  const i = p.lastIndexOf(".");
  return i === -1 ? "" : p.slice(i).toLowerCase();
}

function scanFile(filePath) {
  let text;
  try { text = readFileSync(filePath, "utf8"); } catch { return []; }
  const findings = [];
  const lines = text.split(/\r?\n/);

  for (let lineNo = 0; lineNo < lines.length; lineNo++) {
    const line = lines[lineNo];
    if (!line) continue;
    pushMatches(line, HEX_RE, "hex", filePath, lineNo + 1, findings);
    pushMatches(line, RGB_RE, "rgb", filePath, lineNo + 1, findings);
    pushMatches(line, HSL_RE, "hsl", filePath, lineNo + 1, findings);
    pushMatches(line, OKLCH_RE, "oklch", filePath, lineNo + 1, findings);
    pushMatches(line, OKLAB_RE, "oklab", filePath, lineNo + 1, findings);
    pushMatches(line, LAB_RE, "lab", filePath, lineNo + 1, findings);
    pushMatches(line, LCH_RE, "lch", filePath, lineNo + 1, findings);
  }
  return findings;
}

function pushMatches(line, regex, kind, file, lineNo, out) {
  regex.lastIndex = 0;
  let m;
  while ((m = regex.exec(line))) {
    const literal = m[0];
    if (ALLOWED_LITERAL.has(literal.toLowerCase())) continue;
    out.push({ file, lineNo, kind, literal, snippet: line.trim() });
  }
}

function main() {
  const args = parseArgs(argv);
  if (args.help || args.dirs.length === 0) {
    if (args.dirs.length === 0 && !args.help) {
      console.error("Provide at least one directory to scan. See --help.\n");
    }
    help();
    exit(args.dirs.length === 0 && !args.help ? 2 : 0);
  }

  const allFindings = [];
  for (const dir of args.dirs) {
    let exists = false;
    try { exists = statSync(dir).isDirectory(); } catch {}
    if (!exists) {
      console.error(`Not a directory: ${dir}`);
      exit(2);
    }
    for (const file of walk(dir)) {
      if (!DEFAULT_EXT.has(extOf(file))) continue;
      if (isTokenFile(file, args.tokenParts)) continue;
      allFindings.push(...scanFile(file));
    }
  }

  if (allFindings.length === 0) {
    console.log("No raw color values detected outside token paths. Clean.");
    exit(0);
  }

  const byKind = {};
  for (const f of allFindings) {
    (byKind[f.kind] ||= []).push(f);
  }

  console.log(`Token drift detected — ${allFindings.length} raw color value(s) outside token paths:\n`);
  const root = process.cwd();
  for (const kind of Object.keys(byKind)) {
    console.log(`-- ${kind.toUpperCase()} (${byKind[kind].length}) — ${SUGGESTIONS[kind]}`);
    for (const f of byKind[kind]) {
      const rel = relative(root, f.file) || f.file;
      console.log(`  ${rel}:${f.lineNo}  ${f.literal}`);
      console.log(`    ${f.snippet}`);
    }
    console.log("");
  }

  console.log(`Total: ${allFindings.length} finding(s). See ai-slop-patterns.md #9 (DesignSystemDrift).`);
  exit(1);
}

main();
