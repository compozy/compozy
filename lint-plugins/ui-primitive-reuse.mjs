/**
 * OXC/ESLint Plugin: @agh/ui primitive reuse.
 *
 * `packages/ui/src/index.ts` is the surface contract for every generic UI
 * primitive. Consumer surfaces (`web/src`, `packages/site`) import those
 * primitives; redefining an exported name locally forks the design system
 * silently. Domain-specific variants must take a domain-prefixed name.
 */

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const UI_INDEX_PATH = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../packages/ui/src/index.ts"
);

const EXEMPT_PATH_SEGMENTS = ["__tests__", "/.storybook/", "/storybook-static/"];
const EXEMPT_FILE_SUFFIXES = [
  ".test.tsx",
  ".test.ts",
  ".spec.tsx",
  ".spec.ts",
  ".stories.tsx",
  ".stories.ts",
];

const EXPORT_BLOCK_RE = /export\s+(type\s+)?\{([^}]*)\}/g;
const PASCAL_CASE_VALUE_RE = /^[A-Z][A-Za-z0-9]*$/;

let cachedPrimitiveNames = null;

/**
 * Collect the PascalCase value exports (components, compound parts) from the
 * `@agh/ui` index source. Type-only exports, hooks, helpers, and ALL_CAPS
 * constants are intentionally excluded — the shadow hazard is component names.
 */
export function parsePrimitiveNames(source) {
  const names = new Set();
  for (const match of source.matchAll(EXPORT_BLOCK_RE)) {
    if (match[1]) continue;
    for (const rawSpecifier of match[2].split(",")) {
      const specifier = rawSpecifier.trim();
      if (!specifier || specifier.startsWith("type ")) continue;
      const aliasMatch = specifier.match(/\bas\s+([A-Za-z0-9_$]+)$/);
      const name = aliasMatch ? aliasMatch[1] : specifier;
      if (PASCAL_CASE_VALUE_RE.test(name) && /[a-z]/.test(name)) {
        names.add(name);
      }
    }
  }
  return names;
}

function primitiveNames() {
  if (cachedPrimitiveNames) return cachedPrimitiveNames;
  let source;
  try {
    source = readFileSync(UI_INDEX_PATH, "utf8");
  } catch (error) {
    throw new Error(
      `compozy-ui-reuse: cannot read the @agh/ui surface contract at ${UI_INDEX_PATH}. ` +
        "If the file moved, update lint-plugins/ui-primitive-reuse.mjs in the same change.",
      { cause: error }
    );
  }
  cachedPrimitiveNames = parsePrimitiveNames(source);
  return cachedPrimitiveNames;
}

function normalizeFilename(filename) {
  return typeof filename === "string" ? filename.replaceAll("\\", "/") : "";
}

function isConsumerSourcePath(filename) {
  const normalized = normalizeFilename(filename);
  if (!normalized) return false;
  if (!normalized.includes("/web/src/") && !normalized.includes("/packages/site/")) {
    return false;
  }
  for (const segment of EXEMPT_PATH_SEGMENTS) {
    if (normalized.includes(segment)) return false;
  }
  for (const suffix of EXEMPT_FILE_SUFFIXES) {
    if (normalized.endsWith(suffix)) return false;
  }
  return true;
}

// PascalCase declarators only count as components when initialized with a
// function form or a wrapper call (memo/forwardRef/cva) — data lookups like
// `const Section = sections[0]` stay out of scope.
const COMPONENT_INIT_TYPES = new Set([
  "ArrowFunctionExpression",
  "FunctionExpression",
  "CallExpression",
  "ClassExpression",
]);

const noShadowUiPrimitive = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid consumer components that redefine an identifier exported by @agh/ui. Import the primitive; domain variants take a domain-prefixed name.",
      recommended: true,
    },
    messages: {
      shadowedPrimitive:
        '"{{name}}" is an @agh/ui primitive — import it from "@agh/ui" instead of redefining it. A genuinely domain-specific variant must use a domain-prefixed name. Inventory: packages/ui/src/index.ts.',
    },
    schema: [],
  },
  create(context) {
    if (!isConsumerSourcePath(context.filename || "")) {
      return {};
    }
    const banned = primitiveNames();
    const report = idNode =>
      context.report({
        node: idNode,
        messageId: "shadowedPrimitive",
        data: { name: idNode.name },
      });
    return {
      FunctionDeclaration(node) {
        if (node.id && banned.has(node.id.name)) report(node.id);
      },
      ClassDeclaration(node) {
        if (node.id && banned.has(node.id.name)) report(node.id);
      },
      VariableDeclarator(node) {
        if (!node.id || node.id.type !== "Identifier" || !banned.has(node.id.name)) return;
        if (!node.init || !COMPONENT_INIT_TYPES.has(node.init.type)) return;
        report(node.id);
      },
    };
  },
};

const plugin = {
  meta: {
    name: "compozy-ui-reuse",
  },
  rules: {
    "no-shadow-ui-primitive": noShadowUiPrimitive,
  },
};

export default plugin;
