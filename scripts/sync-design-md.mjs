#!/usr/bin/env bun
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const q = String.fromCharCode(96);
const root = process.cwd();
const args = new Set(process.argv.slice(2));
const designPath = join(root, "DESIGN.md");
const runtimeCss = readFileSync(join(root, "packages/ui/src/tokens.css"), "utf8");
const siteCss = readFileSync(join(root, "packages/site/app/global.css"), "utf8");
const design = readFileSync(designPath, "utf8");
const colorGroups = {
  "surface-ramp": "rail canvas canvas-soft canvas-tint sidebar elevated hover disabled",
  hairlines: "line line-soft line-strong line-focus",
  "text-ladder": "fg fg-strong muted subtle faint",
  accent:
    "accent accent-hover accent-strong accent-ink accent-tint accent-tint-strong accent-dim accent-glow",
  "glaze-ladder":
    "row-hover row-selected surface-glaze bar-fill input-fill btn-default-fill btn-default-hover badge-fill",
};
const componentSizeTokenPattern = /^(container|height|size|space|spacing|width)-|^overlay-blur$/;
const runtimeTheme = parseTheme(runtimeCss, "packages/ui/src/tokens.css");
const runtimeDecls = parseDecls(runtimeCss);
const siteTheme = parseTheme(siteCss, "packages/site/app/global.css");
const siteDecls = parseDecls(siteCss);
const runtime = new Map(runtimeDecls.map(({ name, value }) => [name, value]));

const body = replaceGeneratedSections(stripFrontmatter(design));
const nextDesign = emitFrontmatter() + "\n\n" + body.trimStart();

if (args.has("--audit-site")) auditSite();
if (args.has("--write")) {
  if (nextDesign !== design) writeFileSync(designPath, nextDesign);
  process.exit(0);
}
if (nextDesign === design) process.exit(0);
process.stdout.write(diffWindow(design, nextDesign));
if (args.has("--check")) process.exit(1);

function parseTheme(css, label) {
  const match = css.match(/@theme(?:\s+inline)?\s*\{([\s\S]*?)\n\}/);
  if (!match) throw new Error("Could not locate @theme block in " + label);
  return parseDecls(match[1]);
}

function parseDecls(css) {
  return Array.from(css.matchAll(/--([a-zA-Z0-9-]+)\s*:\s*([^;]+);/g), match => ({
    name: match[1],
    value: match[2].replace(/\s+/g, " ").trim(),
  }));
}

function stripFrontmatter(text) {
  if (!text.startsWith("---\n")) return text;
  const end = text.indexOf("\n---", 4);
  return end === -1 ? text : text.slice(end + 5).replace(/^\n+/, "");
}

function replaceGeneratedSections(text) {
  const sections = [
    ...Object.entries(colorGroups).map(([id, stems]) => [id, tokenTable(colorRows(stems))]),
    ["shell-glass", tokenTable(shellGlassRows())],
    ["signal", signalTable()],
    ["owner-avatar", tokenTable(prefixRows(runtimeTheme, "color-avatar-"))],
    ["status-tone", tokenTable(prefixRows(runtimeTheme, "color-kind-"))],
    ["type-ladder", typeTable()],
    ["tracking-ladder", tokenTable(prefixRows(runtimeTheme, "tracking-"))],
    ["radii", tokenTable(prefixRows(runtimeTheme, "radius", true))],
    ["component-sizes", tokenTable(namedRows(runtimeDecls, componentSizeTokenPattern))],
    ["shadows", tokenTable(prefixRows(runtimeTheme, "shadow-"))],
    ["shell-backdrop", tokenTable(namedRows(runtimeDecls, /^(blur-shell|wallpaper-)/))],
    ["motion", tokenTable(namedRows(runtimeTheme, /^(duration|ease)-/))],
    ["site-clamps", tokenTable(namedRows(siteTheme, /^text-site-|^leading-doc-body$/))],
    ["site-layout", tokenTable(namedRows(siteDecls, /^site-/))],
  ];
  for (const [id, content] of sections) text = replaceSection(text, id, content);
  return text;
}

function replaceSection(text, id, content) {
  const re = new RegExp("<!-- BEGIN:tokens:" + id + " -->[\\s\\S]*?<!-- END:tokens:" + id + " -->");
  if (!re.test(text)) throw new Error("DESIGN.md is missing marker pair tokens:" + id);
  return text.replace(
    re,
    "<!-- BEGIN:tokens:" + id + " -->\n\n" + content.trim() + "\n\n<!-- END:tokens:" + id + " -->"
  );
}

function colorRows(stems) {
  return stems.split(" ").map(stem => ["--color-" + stem, runtime.get("color-" + stem)]);
}

// The shell-glass family pairs each canonical `:root --shell-glass*` literal with
// its `@theme --color-*` adapter, so DESIGN.md documents both the contract name and
// the utility-facing token that references it (single literal, no duplication).
function shellGlassRows() {
  return ["shell-glass", "shell-glass-pop"].flatMap(stem => [
    ["--" + stem, runtime.get(stem)],
    ["--color-" + stem, runtime.get("color-" + stem)],
  ]);
}

function namedRows(decls, re) {
  return decls.filter(({ name }) => re.test(name)).map(({ name, value }) => ["--" + name, value]);
}

function prefixRows(decls, prefix, includeBase = false) {
  return decls
    .filter(
      ({ name }) => (name === prefix || name.startsWith(prefix)) && (includeBase || name !== prefix)
    )
    .map(({ name, value }) => ["--" + name, value]);
}

function signalTable() {
  const rows = "success warning danger info neutral"
    .split(" ")
    .map(stem => [
      stem.charAt(0).toUpperCase() + stem.slice(1),
      code("--color-" + stem),
      code(runtime.get("color-" + stem)),
      code("--color-" + stem + "-tint"),
      code(runtime.get("color-" + stem + "-tint")),
    ]);
  return markdownTable(["Role", "Token", "Value", "Tint token", "Tint value"], rows);
}

function typeTable() {
  const rows = [];
  for (const { name, value } of runtimeTheme) {
    if (!name.startsWith("text-") || name.endsWith("--line-height")) continue;
    const stem = name.slice("text-".length);
    rows.push([
      code("--" + name),
      code(value),
      code(runtime.get("text-" + stem + "--line-height")),
      code(runtime.get("tracking-" + stem)),
    ]);
  }
  return markdownTable(["Token", "Size", "Line", "Tracking"], rows);
}

function tokenTable(rows) {
  rows = rows.map(([name, value]) => [code(name), code(value)]);
  const pairs = rows.length > 20 ? 3 : rows.length > 10 ? 2 : 1;
  const body = [];
  for (let idx = 0; idx < rows.length; idx += pairs) {
    const chunk = rows.slice(idx, idx + pairs).flat();
    while (chunk.length < pairs * 2) chunk.push("");
    body.push(chunk);
  }
  return markdownTable(Array.from({ length: pairs }, () => ["Token", "Value"]).flat(), body);
}

function markdownTable(headers, rows) {
  const allRows = [headers, ...rows];
  const widths = headers.map((_, idx) =>
    Math.max(3, ...allRows.map(row => String(row[idx] ?? "").length))
  );
  const separator = widths.map(width => "-".repeat(width));
  return [headers, separator, ...rows]
    .map(
      row =>
        "| " + row.map((cell, idx) => String(cell ?? "").padEnd(widths[idx])).join(" | ") + " |"
    )
    .join("\n");
}

function code(value) {
  return value ? q + value + q : "";
}

function emitFrontmatter() {
  const lines = [
    "---",
    "# Generated from:",
    "#   packages/ui/src/tokens.css         (runtime)",
    "#   packages/site/app/global.css       (site extensions)",
    "# by scripts/sync-design-md.mjs.",
    "# Do not edit by hand. Run make codegen to refresh.",
    "spec_version: 1",
    "name: AGH",
    "tokens:",
    "  runtime:",
    yamlMap("colors", namespaceMap(runtimeTheme, "color"), 4),
    yamlMap("typography", typographyMap(), 4),
    yamlMap("rounded", radiusMap(), 4),
    "    motion:",
    yamlMap("duration", namespaceMap(runtimeTheme, "duration"), 6),
    yamlMap("ease", namespaceMap(runtimeTheme, "ease"), 6),
    yamlMap("shadow", namespaceMap(runtimeTheme, "shadow"), 4),
    yamlMap(
      "sizes",
      mapNamed(runtimeDecls, componentSizeTokenPattern, name => name),
      4
    ),
    "  site:",
    yamlMap(
      "typography-clamps",
      mapNamed(siteTheme, /^text-site-|^leading-doc-body$/, name =>
        name.replace(/^text-site-/, "")
      ),
      4
    ),
    yamlMap(
      "layout",
      mapNamed(siteDecls, /^site-/, name => name.replace(/^site-/, "")),
      4
    ),
    "---",
  ];
  return lines.join("\n");
}

function namespaceMap(decls, ns) {
  return mapNamed(decls, new RegExp("^" + ns + "-"), name => name.slice(ns.length + 1));
}

function mapNamed(decls, re, key) {
  return Object.fromEntries(
    namedRows(decls, re).map(([name, value]) => [key(name.replace(/^--/, "")), value])
  );
}

function typographyMap() {
  const rows = [];
  for (const { name, value } of runtimeTheme) {
    if (!name.startsWith("text-") || name.endsWith("--line-height")) continue;
    const stem = name.slice(5);
    const attrs = { size: value };
    const line = runtime.get("text-" + stem + "--line-height");
    const tracking = runtime.get("tracking-" + stem);
    if (line) attrs.line = line;
    if (tracking) attrs.tracking = tracking;
    rows.push([stem, attrs]);
  }
  return Object.fromEntries(rows);
}

function radiusMap() {
  const rows = [];
  for (const { name, value } of runtimeTheme) {
    if (name !== "radius" && !name.startsWith("radius-")) continue;
    rows.push([name === "radius" ? "DEFAULT" : name.slice("radius-".length), value]);
  }
  return Object.fromEntries(rows);
}

function yamlMap(name, values, indent) {
  const pad = " ".repeat(indent);
  return [
    pad + name + ":",
    ...Object.entries(values).map(([key, value]) => pad + "  " + key + ": " + yamlValue(value)),
  ].join("\n");
}

function yamlValue(value) {
  if (value && typeof value === "object") {
    return (
      "{ " +
      Object.entries(value)
        .map(([key, val]) => key + ": " + JSON.stringify(val))
        .join(", ") +
      " }"
    );
  }
  return JSON.stringify(value);
}

function auditSite() {
  for (const { name, value } of siteTheme) {
    const selfRef = value.match(/^var\(--([a-zA-Z0-9-]+)\)$/)?.[1];
    if (selfRef === name && !runtime.has(name)) {
      console.error(
        "audit-site: --" +
          name +
          " self-references a token that runtime tokens.css does not declare"
      );
    }
  }
  const darkBlock = siteCss.match(/\.dark\s*\{([\s\S]*?)\n\}/)?.[1] ?? "";
  const stale = {
    "#141312": "var(--color-canvas) / #131211",
    "#1e1c1b": "var(--color-canvas-soft) / #1a1918",
    "#2e2c2b": "var(--color-elevated) / #232220",
    "#3c3a39": "var(--color-line) / rgba(255, 255, 255, 0.055)",
    "#e5e5e7": "var(--color-fg) / #ececef",
    "#8e8e93": "var(--color-muted) / #9a9a9f",
  };
  for (const [hex, expected] of Object.entries(stale)) {
    if (darkBlock.toLowerCase().includes(hex))
      console.error("audit-site: stale " + hex + " in .dark block; expected " + expected);
  }
}

function diffWindow(before, after) {
  const a = before.split("\n");
  const b = after.split("\n");
  let first = 0;
  while (first < a.length && first < b.length && a[first] === b[first]) first += 1;
  const start = Math.max(0, first - 8);
  const oldChunk = a.slice(start, first + 24).map(line => "- " + line);
  const newChunk = b.slice(start, first + 24).map(line => "+ " + line);
  return [
    "DESIGN.md is out of sync. Run make codegen or scripts/sync-design-md.mjs --write.",
    "--- DESIGN.md:" + (start + 1),
    ...oldChunk,
    "+++ DESIGN.md:" + (start + 1),
    ...newChunk,
    "",
  ].join("\n");
}
