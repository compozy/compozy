import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const directory = path.dirname(fileURLToPath(import.meta.url));
const read = file => fs.readFileSync(path.join(directory, file), "utf8");
const exists = file => fs.existsSync(path.join(directory, file));
const failures = [];

const surfaces = [
  "create-agent.html",
  "create-bridge.html",
  "edit-bridge.html",
  "start-session.html",
  "create-knowledge.html",
  "edit-knowledge.html",
  "create-network-channel.html",
  "edit-network-channel.html",
  "create-mcp-server.html",
  "edit-mcp-server.html",
  "create-vault-secret.html",
  "create-sandbox-profile.html",
  "edit-sandbox-profile.html",
  "add-workspace.html",
  "create-provider-sheet.html",
  "edit-provider-sheet.html",
];

const componentMatrix = {
  "create-agent.html": ["dialog", "scope-selector", "workspace-select", "runtime-selector", "radio-card", "command-select", "dialog-footer"],
  "create-bridge.html": ["dialog", "scope-selector", "workspace-select", "radio-card", "secret-field", "dialog-footer"],
  "edit-bridge.html": ["dialog", "immutable-identity", "secret-field", "native-select", "dialog-footer"],
  "start-session.html": ["dialog", "agent-select", "workspace-select", "runtime-selector", "command-select", "dialog-footer"],
  "create-knowledge.html": ["dialog", "radio-card", "dialog-footer"],
  "edit-knowledge.html": ["dialog", "immutable-identity", "dialog-footer"],
  "create-network-channel.html": ["dialog", "agent-multi-select", "radio-card", "command-select", "dialog-footer"],
  "edit-network-channel.html": ["dialog", "immutable-identity", "radio-card", "command-select", "dialog-footer"],
  "create-mcp-server.html": ["dialog", "scope-selector", "workspace-select", "radio-card", "native-select", "secret-field", "dialog-footer"],
  "edit-mcp-server.html": ["dialog", "immutable-identity", "radio-card", "native-select", "secret-field", "dialog-footer"],
  "create-vault-secret.html": ["dialog", "secret-field", "dialog-footer"],
  "create-sandbox-profile.html": ["dialog", "radio-card", "native-select", "secret-field", "dialog-footer"],
  "edit-sandbox-profile.html": ["dialog", "immutable-identity", "radio-card", "native-select", "secret-field", "dialog-footer"],
  "add-workspace.html": ["dialog", "directory-browser", "agent-select", "command-select", "dialog-footer"],
  "create-provider-sheet.html": ["sheet", "settings-field-row", "radio-card", "secret-field", "native-select", "dialog-footer"],
  "edit-provider-sheet.html": ["sheet", "settings-field-row", "immutable-identity", "radio-card", "secret-field", "native-select", "dialog-footer"],
};

const required = [
  ...surfaces,
  ...surfaces.map(file => `${file}.artifact.json`),
  "index.html",
  "modal-system.css",
  "modal-system.js",
  "MODAL-STANDARD.md",
  "STATE-MATRIX.md",
  "CHECKLIST.md",
  "VISUAL-VALIDATION.md",
  "critique.json",
];

for (const file of required) {
  if (!exists(file)) failures.push(`${file}: missing`);
}

const voidTags = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"]);
function checkStructure(file, source) {
  const stripped = source.replace(/<!--([\s\S]*?)-->/g, "").replace(/<!doctype[^>]*>/gi, "");
  const stack = [];
  for (const match of stripped.matchAll(/<\/?([a-z][\w-]*)\b[^>]*>/gi)) {
    const raw = match[0];
    const name = match[1].toLowerCase();
    if (voidTags.has(name) || /\/\s*>$/.test(raw)) continue;
    if (raw.startsWith("</")) {
      const open = stack.pop();
      if (open !== name) {
        failures.push(`${file}: closing </${name}> does not match <${open ?? "none"}>`);
        return;
      }
    } else {
      stack.push(name);
    }
  }
  if (stack.length) failures.push(`${file}: unclosed <${stack.at(-1)}> tag`);
}

function checkUniqueIds(file, source) {
  const ids = [...source.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]);
  const duplicates = [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))];
  if (duplicates.length) failures.push(`${file}: duplicate id ${duplicates.join(", ")}`);
  const artifactIds = [...source.matchAll(/data-od-id="([^"]+)"/g)].map(match => match[1]);
  const duplicateArtifactIds = [...new Set(artifactIds.filter((id, index) => artifactIds.indexOf(id) !== index))];
  if (duplicateArtifactIds.length) failures.push(`${file}: duplicate data-od-id ${duplicateArtifactIds.join(", ")}`);
}

function checkReferences(file, source) {
  const ids = new Set([...source.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]));
  for (const match of source.matchAll(/\b(?:aria-controls|aria-labelledby|aria-describedby)="([^"]+)"/g)) {
    for (const reference of match[1].split(/\s+/).filter(Boolean)) {
      if (!ids.has(reference)) failures.push(`${file}: unresolved ARIA reference ${reference}`);
    }
  }
  for (const match of source.matchAll(/<label\b[^>]*\bfor="([^"]+)"/g)) {
    if (!ids.has(match[1])) failures.push(`${file}: unresolved label target ${match[1]}`);
  }
}

function checkComponentRoots(file, source) {
  for (const marker of ["command-select", "agent-select", "agent-multi-select", "workspace-select"]) {
    const matches = [...source.matchAll(new RegExp(`<([a-z][\\w-]*)\\b([^>]*)data-od-component="${marker}"([^>]*)>`, "gi"))];
    for (const match of matches) {
      const tag = match[1].toLowerCase();
      const attributes = `${match[2]} ${match[3]}`;
      if (tag !== "button") failures.push(`${file}: ${marker} marker must be on the real button trigger, found <${tag}>`);
      const controls = attributes.match(/aria-controls="([^"]+)"/)?.[1];
      if (!controls || !/data-component-trigger="(?:command-select|agent-multi-select)"/.test(attributes)) {
        failures.push(`${file}: ${marker} requires a data-component-trigger button with aria-controls`);
        continue;
      }
      const popoverPattern = new RegExp(`<div\\b(?=[^>]*id="${controls.replace(/[.*+?^${}()|[\\]\\]/g, "\\$&")}")(?=[^>]*class="[^"]*component-popover)[^>]*role="listbox"`, "i");
      if (!popoverPattern.test(source)) failures.push(`${file}: ${marker} trigger does not control a listbox popover (${controls})`);
    }
  }
  for (const match of source.matchAll(/<div class="sec"[^>]*>/g)) {
    if (!match[0].includes('data-od-component="form-section"')) failures.push(`${file}: .sec root missing form-section marker`);
  }
  for (const match of source.matchAll(/<div class="settings-row field"[^>]*>/g)) {
    if (!match[0].includes('data-od-component="settings-field-row"')) failures.push(`${file}: .settings-row root missing settings-field-row marker`);
  }
  for (const match of source.matchAll(/<div class="(?:notice|note)(?:\s[^"]*)?"[^>]*>/g)) {
    if (!match[0].includes('data-od-component="alert"')) failures.push(`${file}: .notice/.note root missing alert marker`);
  }
  if (/<div class="settings-rows"[^>]*data-od-component="settings-field-row"/.test(source)) failures.push(`${file}: settings-field-row marker must not sit on .settings-rows wrapper`);
}

const launcher = read("index.html");
checkStructure("index.html", launcher);
checkUniqueIds("index.html", launcher);
checkReferences("index.html", launcher);
checkComponentRoots("index.html", launcher);

for (const file of surfaces) {
  const source = read(file);
  checkStructure(file, source);
  checkUniqueIds(file, source);
  checkReferences(file, source);
  checkComponentRoots(file, source);
  if (!/<meta\s+name="viewport"/i.test(source)) failures.push(`${file}: viewport metadata missing`);
  if (!source.includes("modal-system.css") || !source.includes("modal-system.js")) failures.push(`${file}: shared assets missing`);
  if ((source.match(/<h1\b/gi) || []).length !== 1) failures.push(`${file}: expected exactly one h1`);
  if (/<main\b/i.test(source)) failures.push(`${file}: modal artifact must not expose a second main landmark`);
  if (!/role="dialog"/.test(source) || !/aria-modal="true"/.test(source) || !/aria-label="[^"]+"/.test(source)) failures.push(`${file}: named modal semantics missing`);
  if (/<script(?!\s+src="modal-system\.js")/i.test(source)) failures.push(`${file}: page-local script forbidden`);
  for (const component of componentMatrix[file]) {
    if (!source.includes(`data-od-component="${component}"`)) failures.push(`${file}: canonical component missing: ${component}`);
  }
  if (!file.includes("provider-sheet")) {
    if (!/<div class="head-icon"[^>]*>/.test(source)) failures.push(`${file}: DialogHeader missing icon well (head-icon)`);
    if (!/<div class="eyebrow">/.test(source)) failures.push(`${file}: DialogHeader missing eyebrow`);
  } else if (!/class="head-icon/.test(source) || !/<div class="eyebrow">/.test(source)) {
    failures.push(`${file}: provider sheet header must keep head-icon + eyebrow`);
  }
  const links = launcher.match(new RegExp(`href="${file.replace(".", "\\.")}"`, "g")) || [];
  if (links.length !== 1) failures.push(`index.html: ${file} linked ${links.length} times`);
}

for (const href of launcher.matchAll(/href="([^"]+\.html)"/g)) {
  if (!exists(href[1])) failures.push(`index.html: unresolved link ${href[1]}`);
}

// The modal design-system catalog was absorbed into ../design-system/
// (patterns.html §05 owns the contract summary; this suite keeps owning
// the runnable library: modal-system.css/.js + the 16 surfaces).

const css = read("modal-system.css");
const geometry = [
  ["--width-modal-sm: 560px", "modal sm width"],
  ["--width-modal-md: 720px", "modal md width"],
  ["--width-modal-lg: 880px", "modal lg width"],
  ["--width-modal-xl: 1180px", "modal xl width"],
  ["--height-modal-tall: 900px", "tall modal height"],
  ["--height-modal-wizard: 960px", "wizard modal height"],
  [".dialog--sheet { width: min(576px, 100%)", "provider sheet literal width"],
  [".input, .textarea, select {\n  width: 100%; min-height: var(--height-input)", "input height token"],
  ["--height-button-sm: 22px", "small button height"],
  ["--height-button-default: 26px", "default button height"],
  ["--height-button-lg: 30px", "large button height"],
  ["--height-switch-default: 18px", "switch height"],
  ["--width-switch-default: 32px", "switch width"],
  ["--height-pill-group-segment-md: 24px", "segment height"],
  ["--height-editor-footer: 52px", "footer height"],
  ["--overlay-blur: 3px", "overlay blur"],
];
for (const [needle, label] of geometry) {
  if (!css.includes(needle)) failures.push(`modal-system.css: ${label} drifted`);
}
for (const alias of ["--surface:", "--surface-2:", "--fg:", "--line:", "--accent:", "--success:", "--warning:", "--info:", "--dur:", "--ease:", "--scrim:", "--highlight:", "--focus-ring:"]) {
  if (new RegExp(`(?:^|[\\s{;])${alias.replace(":", "\\s*:")}`, "m").test(css)) {
    failures.push(`modal-system.css: parallel token alias forbidden: ${alias}`);
  }
}
// --danger: as a custom property (not .btn--danger class names)
if (/(?:^|[\s{;])--danger\s*:/m.test(css)) failures.push("modal-system.css: parallel token alias forbidden: --danger:");
for (const invented of ["--width-provider-sheet", "--height-switch:", "--width-switch:", "--height-segment"]) {
  if (css.includes(invented)) failures.push(`modal-system.css: invented token alias forbidden: ${invented}`);
}
if (!css.includes("--height-input: 36px")) failures.push("modal-system.css: --height-input contract missing");
if (!css.includes(":focus-visible") || !css.includes("outline: 2px")) failures.push("modal-system.css: 2px focus-visible contract missing");
if (!css.includes("prefers-reduced-motion")) failures.push("modal-system.css: reduced-motion contract missing");
for (const mobileTarget of [".input, select, .command-select { min-height: 44px", ".runtime-trigger { grid-template-columns: minmax(0, 1fr) auto; min-height: 44px", ".component-popover__rail button, .component-popover__option, .favorite-toggle { min-height: 44px", ".switch::before { position: absolute; inset: -13px -6px"]) {
  if (!css.includes(mobileTarget)) failures.push(`modal-system.css: mobile target contract missing: ${mobileTarget}`);
}

const combined = [...surfaces, "index.html"].map(read).join("\n");
const forbidden = [
  [/\sstyle="/i, "inline style"],
  [/linear-gradient|radial-gradient|conic-gradient/i, "gradient"],
  [/class="[^"]*agent-field/i, "obsolete agent-field"],
  // Numbered FormSection ordinals are allowed in design-system anatomy demos only.
  // Surface modals must not ship .sec__num — checked separately below.
  [/<select[^>]+id="agent"/i, "native agent selector"],
  [/<select[^>]+id="provider"/i, "separate provider selector"],
  [/<select[^>]+id="model"/i, "separate model selector"],
  [/<select[^>]+data-od-component="(?:command-select|agent-select|agent-multi-select|workspace-select)"/i, "native select carrying command selector marker"],
];
for (const [pattern, label] of forbidden) {
  if (pattern.test(combined)) failures.push(`artifacts: forbidden ${label} found`);
}
for (const file of surfaces) {
  if (/class="sec__num"/i.test(read(file))) failures.push(`${file}: numbered editorial FormSection ordinal forbidden on product surfaces`);
}
if (/\.tpl\[[^\]]+\][^{]*\{[^}]*var\(--color-accent/i.test(css)) failures.push("modal-system.css: selected RadioCard must not use accent");
if (/border-(?:left|right):\s*[2-9]/i.test(css)) failures.push("modal-system.css: side-stripe accent pattern forbidden");

for (const providerFile of ["create-provider-sheet.html", "edit-provider-sheet.html"]) {
  if (read(providerFile).includes('data-od-component="runtime-selector"')) failures.push(`${providerFile}: RuntimeSelector forbidden in provider settings`);
}

const sharedScript = read("modal-system.js");
for (const marker of ["activeDialog", "data-component-trigger", "agent-multi-select", "data-safe-json", "data-members-list", "secretRequiredToggle", "data-absolute-path", "data-delivery-test", "data-auth-refresh", "previewName", "aria-describedby"]) {
  if (!sharedScript.includes(marker)) failures.push(`modal-system.js: shared behavior marker missing: ${marker}`);
}
for (const behavior of ["event.stopPropagation()", "event.key === 'Home'", "event.key === 'End'", "syncMultiTrigger", "data-favorite-key", "state === 'no-model'", "state === 'disabled'", "state === 'stale'"]) {
  if (!sharedScript.includes(behavior)) failures.push(`modal-system.js: behavior contract missing: ${behavior}`);
}

// The RuntimeSelector reference prototype moved to _done/agents/ and was
// later rewritten (approved reasoning-slider redesign), so the old 8-level
// pill contract no longer applies. The living RuntimeSelector contract is
// owned by ../design-system/patterns.html §05 and production
// web/src/systems/runtime/components.

if (failures.length) {
  console.error(failures.join("\n"));
  process.exit(1);
}

console.log(`PASS: ${surfaces.length} modal surfaces, canonical component matrix, and shared assets verified.`);
