/**
 * OXC/ESLint Plugin: Compozy design-system rules.
 *
 * The rules below protect design-system contracts that are easy to regress with
 * inline Tailwind classes or direct lucide imports. Rules are intentionally
 * small and filename-scoped so they can be registered before their severity
 * ramp without blocking in-flight consumer migrations.
 */

const ALLOW_PATH_SEGMENTS = ["__tests__", "/.storybook/", "/storybook-static/"];

const ALLOW_FILE_SUFFIXES = [".test.tsx", ".test.ts", ".stories.tsx", ".stories.ts"];

// Files whose entire purpose is to BE a typographic primitive (avatars whose
// rendered glyph happens to use mono uppercase, custom site eyebrows, etc.).
// New consumers go through `<Eyebrow>` from `@agh/ui`; these declare it.
const ALLOW_FILE_PATHS = [
  "/web/src/systems/network/components/timeline/message-avatar.tsx",
  "/packages/site/components/blog/mono-eyebrow.tsx",
  "/packages/site/components/blog/date-stamp.tsx",
  "/packages/site/components/blog/kind-chip.tsx",
];

// Structural HTML elements that legitimately receive eyebrow typography directly
// (because they are the load-bearing semantic node for their context — `<dt>` in a
// definition list, `<label>` for form controls, `<th>` table heads, breadcrumb
// wrappers, etc.). Non-element callers (custom components like `<TableHead>` from
// shadcn) are also exempt because their typography is owned by the primitive.
const STRUCTURAL_ELEMENTS = new Set([
  "label",
  "dt",
  "dd",
  "th",
  "thead",
  "tr",
  "summary",
  "figcaption",
  "legend",
]);

const ARBITRARY_TEXT_RE = /text-\[[^\]]+\]/;

const ARBITRARY_TRACKING_RE = /tracking-\[[^\]]+\]/;

const DELETED_EYEBROW_UTILITY_RE = /\beyebrow-(?:badge|micro)\b/;

const DESIGN_GLAZE_RGBA_RE = /\bbg-\[rgba\(255,255,255,0\.0\d+\)\]/;

const BANNED_LOADER_IMPORTS = new Set(["Loader2", "Loader2Icon"]);

const DESIGN_GLAZE_ALLOW_FILE_PATHS = [
  "/web/src/components/design-system-showcase.tsx",
  "/web/src/components/stories/design-system-showcase.stories.tsx",
];

const BANNED_IMPORT_OWNER_PATHS = [
  "/packages/ui/src/components/spinner.tsx",
  "/packages/ui/src/components/sonner.tsx",
];

export function normalizeFilename(filename) {
  return typeof filename === "string" ? filename.replaceAll("\\", "/") : "";
}

function isExemptPath(filename) {
  const normalized = normalizeFilename(filename);
  if (!normalized) return true;
  for (const seg of ALLOW_PATH_SEGMENTS) {
    if (normalized.includes(seg)) return true;
  }
  for (const suf of ALLOW_FILE_SUFFIXES) {
    if (normalized.endsWith(suf)) return true;
  }
  for (const path of ALLOW_FILE_PATHS) {
    if (normalized.endsWith(path) || normalized.includes(path)) return true;
  }
  return false;
}

export function isTestOrStoryPath(filename) {
  const normalized = normalizeFilename(filename);
  if (!normalized) return true;
  for (const seg of ALLOW_PATH_SEGMENTS) {
    if (normalized.includes(seg)) return true;
  }
  for (const suffix of ALLOW_FILE_SUFFIXES) {
    if (normalized.endsWith(suffix)) return true;
  }
  return false;
}

export function isFrontendSourcePath(filename) {
  const normalized = normalizeFilename(filename);
  return (
    normalized.includes("/web/src/") ||
    normalized.includes("/packages/ui/src/") ||
    normalized.includes("/packages/site/")
  );
}

function isDesignGlazeAllowedPath(filename) {
  const normalized = normalizeFilename(filename);
  if (isTestOrStoryPath(normalized)) return true;
  return DESIGN_GLAZE_ALLOW_FILE_PATHS.some(path => normalized.endsWith(path));
}

function isBannedImportAllowedPath(filename) {
  const normalized = normalizeFilename(filename);
  if (isTestOrStoryPath(normalized)) return true;
  return BANNED_IMPORT_OWNER_PATHS.some(path => normalized.endsWith(path));
}

export function splitClassTokens(value) {
  if (!value || typeof value !== "string") return [];
  return value.split(/\s+/).filter(Boolean);
}

function getImportedName(specifier) {
  if (!specifier || specifier.type !== "ImportSpecifier") return null;
  const imported = specifier.imported;
  if (!imported) return null;
  if (imported.type === "Identifier") return imported.name;
  if (imported.type === "Literal" && typeof imported.value === "string") return imported.value;
  return null;
}

function isViolation(value) {
  if (!value || typeof value !== "string") return false;
  // Deleted utility-class literals: `eyebrow-badge` and `eyebrow-micro`
  // were collapsed into the single `eyebrow` contract; any
  // remaining className referencing them is a regression.
  if (DELETED_EYEBROW_UTILITY_RE.test(value)) return true;
  const tokens = splitClassTokens(value);
  const hasMono = tokens.includes("font-mono");
  const hasUpper = tokens.includes("uppercase");
  if (hasMono && hasUpper) return true;
  if (hasUpper && (ARBITRARY_TEXT_RE.test(value) || ARBITRARY_TRACKING_RE.test(value))) {
    return true;
  }
  if (hasMono && (ARBITRARY_TEXT_RE.test(value) || ARBITRARY_TRACKING_RE.test(value))) {
    // Allow plain mono code/badge styling without uppercase. Flag typography
    // tuples when an arbitrary text/tracking value is paired with its sibling.
    const hasTracking = tokens.some(token => token.startsWith("tracking-"));
    const hasText = tokens.some(token => token.startsWith("text-"));
    return (
      (ARBITRARY_TEXT_RE.test(value) && hasTracking) ||
      (ARBITRARY_TRACKING_RE.test(value) && hasText)
    );
  }
  return false;
}

function hasDesignGlazeRgba(value) {
  return typeof value === "string" && DESIGN_GLAZE_RGBA_RE.test(value);
}

function findInlineDesignTuple(values) {
  const tokens = splitClassTokens(values.join(" "));
  if (tokens.includes("text-[22px]") && tokens.includes("tracking-[-0.026em]")) {
    return "pageH1Tuple";
  }
  // Tailwind duration literal + rounded shorthand bans are intentionally not enforced
  // here yet. This rule currently gates the page-h1 tuple only; adding the duration
  // and radius sweeps without their codemod would gate CI on unrelated callsites.
  return null;
}

export function extractStringValues(node) {
  if (!node) return [];
  if (node.type === "Literal" && typeof node.value === "string") {
    return [node.value];
  }
  if (node.type === "TemplateLiteral") {
    return node.quasis.map(q => (q.value && q.value.cooked) || "");
  }
  if (node.type === "JSXExpressionContainer") {
    return extractStringValues(node.expression);
  }
  if (node.type === "CallExpression") {
    // cn(...) / clsx(...) / cva(...) — collect string args.
    const out = [];
    out.push(...extractStringValues(node.callee));
    for (const arg of node.arguments) {
      out.push(...extractStringValues(arg));
    }
    return out;
  }
  if (node.type === "ConditionalExpression") {
    return [...extractStringValues(node.consequent), ...extractStringValues(node.alternate)];
  }
  if (node.type === "LogicalExpression") {
    return [...extractStringValues(node.left), ...extractStringValues(node.right)];
  }
  return [];
}

export const noInlineEyebrow = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid inlining the AGH eyebrow tuple in JSX className. Use <Eyebrow> from @agh/ui instead.",
      recommended: true,
    },
    messages: {
      inlineEyebrow:
        "Inlined eyebrow tuple in className. Use <Eyebrow> from @agh/ui (Inter UC 11/600/-0.005em, single style — no case/family/tone/size props). See DESIGN.md §3 and lesson L-022.",
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || "";
    if (isExemptPath(filename)) {
      return {};
    }

    return {
      JSXAttribute(node) {
        if (!node.name || node.name.name !== "className") return;
        // Skip when the enclosing element is a structural HTML primitive
        // (label, dt, dd, th, etc.) that legitimately owns eyebrow typography.
        const opening = node.parent;
        if (opening && opening.type === "JSXOpeningElement" && opening.name) {
          const elementName = opening.name.type === "JSXIdentifier" ? opening.name.name : null;
          if (elementName && STRUCTURAL_ELEMENTS.has(elementName)) {
            return;
          }
          // Custom-component className passes typography down (e.g. <TableHead>,
          // <TreeItemLabel>, <Item>). Treat any PascalCase tag as exempt — those
          // are component surfaces, not direct DOM nodes.
          if (elementName && /^[A-Z]/.test(elementName)) {
            return;
          }
        }
        const values = extractStringValues(node.value);
        for (const value of values) {
          if (isViolation(value)) {
            context.report({
              node,
              messageId: "inlineEyebrow",
            });
            return;
          }
        }
      },
    };
  },
};

export const noDesignGlazeRgba = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid inline white rgba glaze backgrounds in frontend JSX className. Use named AGH glaze tokens instead.",
      recommended: false,
    },
    messages: {
      inlineGlaze:
        "Inline surface glaze rgba in className. Use named glaze utilities: bg-row-hover, bg-row-selected, bg-surface-glaze, bg-bar-fill, bg-input-fill, bg-btn-default-fill, bg-btn-default-hover, or bg-badge-fill. See DESIGN.md §2.5.",
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || "";
    if (!isFrontendSourcePath(filename) || isDesignGlazeAllowedPath(filename)) {
      return {};
    }

    return {
      JSXAttribute(node) {
        if (!node.name || node.name.name !== "className") return;
        const values = extractStringValues(node.value);
        for (const value of values) {
          if (hasDesignGlazeRgba(value)) {
            context.report({
              node,
              messageId: "inlineGlaze",
            });
            return;
          }
        }
      },
    };
  },
};

export const noBannedImports = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid direct Loader2/Loader2Icon imports from lucide-react in frontend source; use the Spinner primitive.",
      recommended: false,
    },
    messages: {
      bannedImport:
        "Importing {{name}} from lucide-react is banned in frontend code. Use <Spinner> from @agh/ui instead. See DESIGN.md §10.",
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || "";
    if (!isFrontendSourcePath(filename) || isBannedImportAllowedPath(filename)) {
      return {};
    }

    return {
      ImportDeclaration(node) {
        if (!node.source || node.source.value !== "lucide-react") return;
        for (const specifier of node.specifiers || []) {
          const importedName = getImportedName(specifier);
          if (BANNED_LOADER_IMPORTS.has(importedName)) {
            context.report({
              node: specifier,
              messageId: "bannedImport",
              data: { name: importedName },
            });
          }
        }
      },
    };
  },
};

export const noInlineDesignTuples = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid inline design-system type, motion, and radius tuples in JSX className. Use the canonical tokens or primitives.",
      recommended: false,
    },
    messages: {
      pageH1Tuple:
        "Inline 22px page H1 tuple in className. Use <DetailHeader> for detail surfaces or tokenized text-detail-h1 tracking-(--tracking-detail-h1). See DESIGN.md §4 and §11.",
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || "";
    if (!isFrontendSourcePath(filename) || isTestOrStoryPath(filename)) {
      return {};
    }

    return {
      JSXAttribute(node) {
        if (!node.name || node.name.name !== "className") return;
        const messageId = findInlineDesignTuple(extractStringValues(node.value));
        if (messageId) {
          context.report({
            node,
            messageId,
          });
        }
      },
    };
  },
};
