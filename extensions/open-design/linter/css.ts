// Adapted from nexu-io/open-design (Apache-2.0); see ../SOURCES.md.
import { AI_DEFAULT_INDIGO } from "./rules";

type CssDeclaration = { prop: string; value: string };
type CssTokenScope = {
  selectors: string[];
  tokens: Map<string, string>;
  themeKeys: Set<string>;
};

const ROOT_FONT_PX = 16;
export function hasAdequateUppercaseTracking(body: string, scopes?: CssTokenScope[]): boolean {
  const themes = buildResolvedThemes(scopes ?? []);
  for (const themeMap of themes) {
    const resolved = resolveCssVars(body, themeMap);
    if (!isResolvedTrackingAdequate(resolved)) return false;
  }
  return true;
}

// Single-resolution tracking check. Parses the declaration list with
// exact property names (so token-name declarations such as
// `--letter-spacing: 0.08em` cannot satisfy the rule) and selects the
// LAST matching `letter-spacing` and `font-size` declarations to model
// CSS source-order cascade — `.eyebrow { letter-spacing: 0.08em;
// letter-spacing: 0.02em }` renders the noncompliant `0.02em` value,
// so the lint must judge against the last declaration, not the first.
function isResolvedTrackingAdequate(body: string): boolean {
  const decls = parseDeclarations(body);
  const ls = findLastDecl(decls, "letter-spacing");
  if (!ls) return false;
  const lsMatch = /^(-?\d*\.?\d+)\s*(em|px|rem)\b/i.exec(ls.value);
  if (!lsMatch) return false;
  const valueText = lsMatch[1];
  const unitText = lsMatch[2];
  if (valueText == null || unitText == null) return false;
  const v = parseFloat(valueText);
  const unit = unitText.toLowerCase();
  if (unit === "em") return v >= 0.06;
  const trackingPx = unit === "rem" ? v * ROOT_FONT_PX : v;
  const fsPx = resolveFontSizePx(decls);
  if (fsPx != null) {
    return fsPx > 0 && trackingPx >= fsPx * 0.06;
  }
  if (decls.some(d => d.prop === "font-size")) return false;
  return trackingPx >= 1;
}

// Build per-theme effective token maps from the per-scope records
// produced by `extractCssTokens`. A "theme" is the default rendering
// (no theme attribute set) plus one entry per distinct theme-attribute
// selector seen across scopes. Default-applying scopes (whose selector
// list contains a bare `:root` / `html` / `body`) apply to every theme
// as a baseline; variant scopes apply only to the themes their
// selector targets. Within a single theme, the most specific matching
// selector wins; source order breaks ties.
//
// Returned as an array — one map per theme. The lint passes only when
// every theme map satisfies the rule, so a default-theme value below
// the floor flags even if a variant overrides it above the floor (and
// vice versa). Building per-theme maps preserves the scope-internal
// relationship between tokens, so values declared together in the
// same scope (e.g. `--display-size` and `--caps-tracking` both on
// `:root`) stay paired during evaluation. The previous design merged
// values by token name across scopes and then took an independent
// per-token cartesian product, which generated impossible cross-theme
// pairings such as `(default-size, dark-track)` and emitted false
// positives on legitimate light/dark theme variants.
function buildResolvedThemes(scopes: CssTokenScope[]): Map<string, string>[] {
  const themeKeys = new Set(["default"]);
  for (const scope of scopes) {
    for (const k of scope.themeKeys) themeKeys.add(k);
  }
  return Array.from(themeKeys, themeKey => {
    const tokens = new Map<string, string>();
    const specificity = new Map<string, number>();
    for (const scope of scopes) {
      const matching = scope.selectors.filter(s => isBareGlobalSelector(s) || s === themeKey);
      if (matching.length === 0) continue;
      const rank = matching.reduce((highest, s) => Math.max(highest, globalThemeSpecificity(s)), 0);
      for (const [name, value] of scope.tokens) {
        if (rank >= (specificity.get(name) ?? -1)) {
          tokens.set(name, value);
          specificity.set(name, rank);
        }
      }
    }
    return tokens;
  });
}

// Only the grammar accepted by isGlobalThemeScopeSelector reaches here:
// at most one :root pseudo-class, one theme attribute, and one type.
// Tens therefore preserve class/attribute priority over the type column.
function globalThemeSpecificity(selector: string): number {
  const base = selector.startsWith(":root") ? 10 : /^(html|body)\b/.test(selector) ? 1 : 0;
  return base + (selector.includes("[") ? 10 : 0);
}

function isBareGlobalSelector(s: string): boolean {
  return /^(?::root|html|body)$/.test(s);
}

function findLastDecl(decls: CssDeclaration[], prop: string): CssDeclaration | undefined {
  for (let i = decls.length - 1; i >= 0; i--) {
    const decl = decls[i];
    if (decl && decl.prop === prop) return decl;
  }
  return undefined;
}

// Split a CSS declaration body into `{ prop, value }` entries, lowercasing
// the property name and skipping custom properties (`--name`). Used by
// the uppercase-tracking lint so substring matches on `letter-spacing`
// or `font-size` cannot collide with token-name declarations.
function parseDeclarations(body: string): CssDeclaration[] {
  const out: CssDeclaration[] = [];
  for (const raw of body.split(";")) {
    const idx = raw.indexOf(":");
    if (idx < 0) continue;
    const prop = raw.slice(0, idx).trim().toLowerCase();
    if (!prop || prop.startsWith("--")) continue;
    const value = raw.slice(idx + 1).trim();
    if (!value) continue;
    out.push({ prop, value });
  }
  return out;
}

// Resolve a same-rule `font-size` declaration to absolute px. Returns
// the px value when font-size is declared in `px` or `rem` (rem maps
// via the root font-size assumption shared with tracking); returns
// `null` when font-size is absent OR present in an unresolvable unit
// (`em`, `%`, `calc(...)`, an unresolved `var(--...)`). The caller
// distinguishes those two `null` cases by re-checking the parsed
// declarations for an exact `font-size` property.
//
// Selects the LAST `font-size` declaration in source order so that a
// rule like `.display { font-size: 48px; font-size: 1em }` is judged
// against the noncompliant `1em` the browser actually renders, not the
// stale earlier `48px`. CSS cascade is last-write-wins on conflicting
// declarations within a single rule body.
function resolveFontSizePx(decls: CssDeclaration[]): number | null {
  const fs = findLastDecl(decls, "font-size");
  if (!fs) return null;
  const m = /^(-?\d*\.?\d+)\s*(px|rem)\b/i.exec(fs.value);
  if (!m) return null;
  const valueText = m[1];
  const unitText = m[2];
  if (valueText == null || unitText == null) return null;
  const v = parseFloat(valueText);
  const unit = unitText.toLowerCase();
  return unit === "rem" ? v * ROOT_FONT_PX : v;
}

// Collect CSS custom properties (`--name: value`) declared in global
// theme scopes (`:root`, `html`, theme-attribute selectors) from every
// `<style>` block in the artifact. Tokens declared on component
// selectors are intentionally ignored: the lint must still catch
// indigo / under-tracking laundered through a local var, and the
// tracking helper resolves only the global-scope tokens artifacts use
// to express design intent.
//
// Returns an array of per-scope records:
//   `{ selectors, tokens, themeKeys }`
// where `tokens` is the per-scope last-write-wins map of CSS custom
// properties, `selectors` lists the parsed selectors from the rule,
// `themeKeys` is the set of theme-attribute selector strings the rule
// targets. Per-theme effective maps are derived downstream from these
// records by `buildResolvedThemes`, which preserves the scope-internal
// relationship between values so a paired declaration like
// `:root { --display-size: 16px; --caps-tracking: 1px }` is judged
// as `(16px, 1px)` together, not against the impossible cross-theme
// pairing `(48px, 1px)` that an independent per-token cartesian over
// distinct values would emit.
//
// Within a single rule body, CSS cascade is last-write-wins: a block
// like `:root { --caps-tracking: 0.02em; --caps-tracking: 0.08em; }`
// renders the second value, and the first never reaches any element.
// Per-scope, we keep only the LAST value declared for each token
// name; cross-scope merging happens later in `buildResolvedThemes`,
// where specificity and then source order select the winning declaration
// between scopes that target the same theme.
export function extractCssTokens(html: string): CssTokenScope[] {
  const scopes: CssTokenScope[] = [];
  for (const styleBlock of html.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gi)) {
    const css = (styleBlock[1] ?? "").replace(/\/\*[\s\S]*?\*\//g, "");
    const ruleRe = /([^{}]*)\{([^{}]*)\}/g;
    let m;
    while ((m = ruleRe.exec(css)) !== null) {
      const sel = (m[1] ?? "").trim();
      if (!selectorListIsGlobalThemeScope(sel)) continue;
      const selectors = sel
        .split(",")
        .map(s => s.trim())
        .filter(Boolean);
      const themeKeys = new Set(selectors.filter(s => !isBareGlobalSelector(s)));
      const body = m[2] ?? "";
      const tokens = new Map();
      for (const decl of body
        .split(";")
        .map(d => d.trim())
        .filter(Boolean)) {
        const dm = /^(--[\w-]+)\s*:\s*(.+)$/.exec(decl);
        if (dm) {
          const tokenName = dm[1];
          const tokenValue = dm[2];
          if (tokenName != null && tokenValue != null) tokens.set(tokenName, tokenValue.trim());
        }
      }
      if (tokens.size === 0) continue;
      scopes.push({ selectors, tokens, themeKeys });
    }
  }
  return scopes;
}

// Replace simple `var(--name)` (and `var(--name, fallback)`) references
// in a CSS declaration body with the literal token value. Iterates a
// few times so a token whose value is itself another `var(--...)`
// resolves through one or two hops; bounded depth so a cyclic
// definition (`--a: var(--b); --b: var(--a)`) terminates instead of
// looping forever. Only one-level fallbacks are recognised — enough
// for the typography pattern this lint cares about, and keeps the
// regex linear-time on artifact-sized inputs.
const VAR_RESOLVE_MAX_DEPTH = 4;
function resolveCssVars(body: string, tokens: Map<string, string>): string {
  let out = body;
  for (let i = 0; i < VAR_RESOLVE_MAX_DEPTH; i++) {
    const next = out.replace(
      /var\(\s*(--[\w-]+)\s*(?:,\s*([^()]*))?\)/g,
      (full: string, name: string, fallback: string | undefined) => {
        const v = tokens.get(name);
        if (v != null) return v;
        if (fallback != null) return fallback.trim();
        return full;
      }
    );
    if (next === out) break;
    out = next;
  }
  return out;
}

// Remove CSS rule blocks that look like design-token definitions.
// Operates only on CSS extracted from <style> blocks — running the
// rule-shaped regex against the full HTML string makes the first
// selector capture include leading text like `<style>`, which then
// fails the `:root` selector test.
//
// A rule is treated as a token block only when ALL THREE conditions hold:
//   1. every selector in the list is a global theme-scope selector
//      (`:root`, `:root[data-theme="..."]`, `html`, `body`, or a bare
//      attribute selector for a known global-theme switch —
//      `data-theme`, `data-color-scheme`, `data-mode`). Selector lists
//      that mix in a component selector — e.g.
//      `:root, .cta { --cta-bg: #6366f1 }` — or that target an
//      arbitrary component/state attribute like `[data-variant="primary"]`
//      or `[aria-current="page"]` fail this test, so indigo laundered
//      through a local var or rule still trips the lint.
//   2. its body is token-shaped: only CSS custom properties
//      (`--name: value`), with a small allowlist for global-theme
//      metadata such as `color-scheme` that legitimately accompanies
//      tokens in `:root` and cannot smuggle a visible color.
//      A non-token declaration on `:root` (e.g.
//      `:root { background: #6366f1 }`) keeps the rule in scope so
//      the indigo check fires.
//   3. no token in the body launders an indigo hex through a
//      non-`--accent` name. The craft contract's escape hatch is to
//      encode indigo as the active design system's `--accent` token;
//      anything else (`:root { --primary: #6366f1 }`,
//      `:root { --button-bg: #4f46e5 }`) is still the LLM-default
//      color hidden behind an arbitrary token name and must stay in
//      scope of the indigo scan.
export function stripTokenBlocks(input: string): string {
  return input.replace(
    /(<style[^>]*>)([\s\S]*?)(<\/style>)/gi,
    (_m: string, open: string, css: string, close: string) =>
      `${open}${stripTokenBlocksFromCss(css)}${close}`
  );
}

function stripTokenBlocksFromCss(css: string): string {
  // Strip CSS comments before any structural matching: a block like
  // `:root { /* brand accent */ --accent: #6366f1; }` would otherwise
  // produce a declaration fragment that begins with the comment,
  // fail `isTokenShapedDeclaration`, and leave a legitimate token
  // definition in scope of the indigo scan.
  const cleaned = css.replace(/\/\*[\s\S]*?\*\//g, "");
  // The body alternation is `[^{}]*` (not `[^}]*`) so the regex matches
  // only innermost `selector { body }` rules. That lets us recognize
  // global token blocks nested inside at-rule wrappers — e.g.
  // `@media (prefers-color-scheme: dark) { :root { --accent: #6366f1 } }`
  // — by matching the inner `:root { ... }` directly. The outer
  // `@media` wrapper is preserved with the inner token block stripped,
  // so the indigo scan no longer fires on legitimate responsive theme
  // declarations.
  return cleaned.replace(
    /([^{}]*)\{([^{}]*)\}/g,
    (full: string, selector: string, body: string) => {
      const sel = (selector || "").trim();
      if (!selectorListIsGlobalThemeScope(sel)) return full;
      const decls = (body || "")
        .split(";")
        .map((d: string) => d.trim())
        .filter(Boolean);
      if (decls.length === 0) return full;
      const tokenShaped = decls.every(isTokenShapedDeclaration);
      if (!tokenShaped) return full;
      // The `--accent` escape hatch is for `--accent` only. Any other
      // global token whose value carries an AI-default indigo hex is
      // still laundering the LLM-default color through an arbitrary
      // name (`--primary: #6366f1`, `--button-bg: #4f46e5`, …). Keep
      // the rule in scope so the indigo lint fires on the literal hex.
      if (decls.some(declarationLaundersIndigo)) return full;
      return "";
    }
  );
}

function declarationLaundersIndigo(decl: string): boolean {
  const m = /^(--[\w-]+)\s*:\s*(.+)$/.exec(decl);
  if (!m) return false;
  const tokenName = m[1];
  const tokenValue = m[2];
  if (tokenName == null || tokenValue == null) return false;
  if (tokenName.toLowerCase() === "--accent") return false;
  const value = tokenValue.toLowerCase();
  for (const hex of AI_DEFAULT_INDIGO) {
    if (value.includes(hex.toLowerCase())) return true;
  }
  return false;
}

function isTokenShapedDeclaration(decl: string): boolean {
  // CSS custom property — the canonical token shape.
  if (/^--[\w-]+\s*:/.test(decl)) return true;
  // Global-theme metadata that legitimately accompanies tokens in
  // `:root` / `html` / `[data-theme="..."]` and whose values are
  // keywords, so they cannot smuggle a hardcoded color.
  if (/^color-scheme\s*:/i.test(decl)) return true;
  return false;
}

function selectorListIsGlobalThemeScope(selector: string): boolean {
  const parts = selector
    .split(",")
    .map((s: string) => s.trim())
    .filter(Boolean);
  if (parts.length === 0) return false;
  return parts.every(isGlobalThemeScopeSelector);
}

// Attribute selectors — bare or attached to `:root`/`html`/`body` —
// are exempted only when the attribute is one of the known
// global-theme switches. A broader exemption would also strip
// arbitrary component/state attribute rules
// (e.g. `[data-variant="primary"] { --button-bg: #6366f1; }`,
// `:root[data-variant="primary"] { --button-bg: #6366f1; }`, or
// `html[aria-current="page"] { --nav-accent: #6366f1; }`), which
// is the exact component-local indigo laundering this lint is
// meant to catch.
const GLOBAL_THEME_ATTRIBUTES = new Set(["data-theme", "data-color-scheme", "data-mode"]);

function isGlobalThemeScopeSelector(s: string): boolean {
  // :root / html / body, optionally suffixed with a single attribute
  // selector. The bare form (no attribute) is always a global theme
  // scope; the prefixed form is only a theme scope when the attribute
  // names one of GLOBAL_THEME_ATTRIBUTES. A component/state attribute
  // suffix (`:root[data-variant="primary"]`, `html[aria-current="page"]`)
  // must keep the rule in scope of the indigo lint.
  const tagAttr = /^(?::root|html|body)(?:\[([a-zA-Z-]+)(?:[*^$|~]?=[^\]]*)?\])?$/.exec(s);
  if (tagAttr) {
    const attrName = tagAttr[1];
    if (!attrName) return true;
    return GLOBAL_THEME_ATTRIBUTES.has(attrName.toLowerCase());
  }
  // Bare attribute selector restricted to known global-theme switches.
  const bareAttr = /^\[([a-zA-Z-]+)(?:[*^$|~]?=[^\]]*)?\]$/.exec(s);
  const bareAttrName = bareAttr?.[1];
  if (bareAttrName && GLOBAL_THEME_ATTRIBUTES.has(bareAttrName.toLowerCase())) {
    return true;
  }
  return false;
}
