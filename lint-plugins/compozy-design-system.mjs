/**
 * OXC/ESLint Plugin: Compozy design-system rules.
 *
 * The rules below protect design-system contracts that are easy to regress with
 * inline Tailwind classes or direct lucide imports. Rules are intentionally
 * small and filename-scoped so they can be registered before their severity
 * ramp without blocking in-flight consumer migrations.
 */

import {
  extractStringValues,
  isFrontendSourcePath,
  isTestOrStoryPath,
  noBannedImports,
  noDesignGlazeRgba,
  noInlineDesignTuples,
  noInlineEyebrow,
  normalizeFilename,
  splitClassTokens,
} from "./compozy-design-system-core-rules.mjs";

/**
 * Tokens whose `<prefix>-(--<name>)` arbitrary form is flagged because the
 * bare utility `<prefix>-<name>` (or a renamed canonical form) exists in
 * `packages/ui/src/tokens.css` `@theme`. The lint rule does NOT attempt the
 * rewrite — that is the job of `scripts/codemod/tokens-bare-utilities.mjs`.
 * The whitelist below covers runtime vars (Radix-injected, JS-resolved
 * sizing, or component-internal tokens kept in `:root`) that legitimately
 * stay in arbitrary form.
 */
const PREFER_BARE_UTILITY_WHITELIST = new Set([
  // Radix-injected (popover, tooltip, dropdown, select, menu)
  "anchor-width",
  "anchor-height",
  "available-width",
  "available-height",
  "transform-origin",
  "accordion-panel-height",
  "fd-docs-row-1",
  // App-injected runtime
  "detail-inspector-width",
  "tree-padding",
  // Component-internal sizing kept in :root (NOT in @theme — see L-023)
  "width-modal-sm",
  "width-modal-md",
  "width-modal-lg",
  "width-modal-xl",
  "size-catalog-logo",
  "size-provider-logo-well",
  "size-pill-group-badge",
  "height-pill-group-segment-md",
  "height-pill-group-segment-sm",
  "space-pill-group-track-gap",
  "space-pill-group-track-padding",
  "space-pill-group-segment-sm-x",
  "space-pill-group-segment-md-x",
  "space-pill-group-badge-x",
  // Eyebrow utility uses arbitrary length syntax internally (allowed)
  "length:--text-eyebrow",
]);

const PREFER_BARE_RE =
  /\b([a-zA-Z][a-zA-Z0-9]*(?:-[a-zA-Z][a-zA-Z0-9]*){0,2})-\(--([a-zA-Z0-9-]+)\)/g;

function findArbitraryTokenViolations(value) {
  if (!value || typeof value !== "string") return [];
  const out = [];
  for (const match of value.matchAll(PREFER_BARE_RE)) {
    const [, prefix, name] = match;
    if (PREFER_BARE_UTILITY_WHITELIST.has(name)) continue;
    if (name.startsWith("radix-")) continue;
    out.push({ prefix, name, full: match[0] });
  }
  return out;
}

const preferBareTokenUtility = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Prefer the bare Tailwind v4 utility over arbitrary-value `(--token)` syntax when the token exists in @theme. Run scripts/codemod/tokens-bare-utilities.mjs to auto-fix.",
      recommended: false,
    },
    messages: {
      preferBare:
        'Use the bare utility instead of "{{full}}". Run `node scripts/codemod/tokens-bare-utilities.mjs --write` from the repo root. See L-023.',
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || "";
    if (!isFrontendSourcePath(filename) || isTestOrStoryPath(normalizeFilename(filename))) {
      return {};
    }
    return {
      JSXAttribute(node) {
        if (!node.name || node.name.name !== "className") return;
        const values = extractStringValues(node.value);
        for (const value of values) {
          const violations = findArbitraryTokenViolations(value);
          for (const v of violations) {
            context.report({ node, messageId: "preferBare", data: { full: v.full } });
          }
        }
      },
    };
  },
};

// Focus-ring accessibility floor (BUG-20260714). Every :focus-visible indicator
// must be ≥2px thick and ≥3:1 non-text contrast (GhostFocus is a critical a11y
// failure). The low-contrast hairline colors (`ring-line-strong` / `ring-ring`)
// and the sub-2px width (`ring-1`) are therefore forbidden inside ANY focus
// variant. Callers use the shared focus tokens `shadow-focus-ring` (outset) /
// `shadow-focus-inset` (inset); accent focus rings stay on `ring-2 ring-accent`.
const FOCUS_RING_COLOR_UTILITIES = new Set(["ring-line-strong", "ring-ring"]);

const FOCUS_RING_WIDTH_UTILITIES = new Set(["ring-1"]);

function focusRingUtilityViolation(token) {
  // Strip an optional Tailwind opacity modifier (e.g. `ring-ring/50`).
  const bare = token.replace(/\/\d+$/, "");
  // The utility is the final colon-separated segment; everything before it is
  // the variant chain. A token with no variant is a resting utility, not focus.
  const lastColon = bare.lastIndexOf(":");
  if (lastColon === -1) return null;
  const variants = bare.slice(0, lastColon);
  const utility = bare.slice(lastColon + 1);
  if (!variants.includes("focus")) return null;
  if (FOCUS_RING_COLOR_UTILITIES.has(utility)) return "focusRingColor";
  if (FOCUS_RING_WIDTH_UTILITIES.has(utility)) return "focusRingWidth";
  return null;
}

function findFocusRingViolations(value) {
  const out = [];
  for (const token of splitClassTokens(value)) {
    const messageId = focusRingUtilityViolation(token);
    if (messageId) out.push(messageId);
  }
  return out;
}

const noLowContrastFocusRing = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid low-contrast (ring-line-strong / ring-ring) or sub-2px (ring-1) focus rings in :focus-visible className. Use the shared focus tokens shadow-focus-ring / shadow-focus-inset.",
      recommended: false,
    },
    messages: {
      focusRingColor:
        "Low-contrast focus ring in className. :focus-visible indicators need ≥3:1 contrast (GhostFocus is a critical a11y failure). Use focus-visible:shadow-focus-ring (outset) or focus-visible:shadow-focus-inset (inset); for accent focus use focus-visible:ring-2 focus-visible:ring-accent. ring-line-strong / ring-ring are banned in :focus-visible. See BUG-20260714 and DESIGN.md §10.",
      focusRingWidth:
        "Sub-2px focus ring in className. :focus-visible indicators must be ≥2px thick. Use focus-visible:shadow-focus-ring / focus-visible:shadow-focus-inset, or focus-visible:ring-2 for accent rings. ring-1 is banned in :focus-visible. See BUG-20260714 and DESIGN.md §10.",
    },
    schema: [],
  },
  create(context) {
    const filename = normalizeFilename(context.filename || "");
    // The focus-ring floor (BUG-20260714) applies to every frontend surface —
    // the web app, the `@agh/ui` primitives, and `packages/site`.
    if (!isFrontendSourcePath(filename) || isTestOrStoryPath(filename)) {
      return {};
    }
    return {
      JSXAttribute(node) {
        if (!node.name || node.name.name !== "className") return;
        const seen = new Set();
        for (const value of extractStringValues(node.value)) {
          for (const messageId of findFocusRingViolations(value)) {
            if (seen.has(messageId)) continue;
            seen.add(messageId);
            context.report({ node, messageId });
          }
        }
      },
    };
  },
};

const plugin = {
  meta: {
    name: "compozy-design-system",
  },
  rules: {
    "no-inline-eyebrow": noInlineEyebrow,
    "no-design-glaze-rgba": noDesignGlazeRgba,
    "no-banned-imports": noBannedImports,
    "no-inline-design-tuples": noInlineDesignTuples,
    "prefer-bare-token-utility": preferBareTokenUtility,
    "no-low-contrast-focus-ring": noLowContrastFocusRing,
  },
};

export default plugin;
