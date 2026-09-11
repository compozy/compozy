// Adapted from nexu-io/open-design (Apache-2.0); see ../SOURCES.md.
import {
  PURPLE_HEXES,
  AI_DEFAULT_INDIGO,
  SLOP_EMOJI,
  INVENTED_METRIC_PATTERNS,
  FILLER_PATTERNS,
  DISPLAY_SANS_RE,
  clip,
  escapeRe,
  detectBlueCyanTrustGradient,
} from "./rules";
import { extractCssTokens, hasAdequateUppercaseTracking, stripTokenBlocks } from "./css";

export type LintFinding = {
  id: string;
  severity: "P0" | "P1" | "P2";
  message: string;
  fix: string;
  snippet?: string;
};

export function lintArtifact(rawHtml: unknown): LintFinding[] {
  const out: LintFinding[] = [];
  if (typeof rawHtml !== "string" || rawHtml.length === 0) return out;

  // Strip HTML comments before any pattern matching — comments often contain
  // pedagogical examples ("paste a `<section class="slide">` here") that
  // would otherwise fire false positives for the section / slide checks.
  const html = rawHtml.replace(/<!--[\s\S]*?-->/g, "");

  // ── P0-1: purple gradient backgrounds ─────────────────────────────
  for (const hex of PURPLE_HEXES) {
    const re = new RegExp(`linear-gradient\\([^)]*${escapeRe(hex)}[^)]*\\)`, "i");
    const m = re.exec(html);
    if (m) {
      out.push({
        severity: "P0",
        id: "purple-gradient",
        message: `Found a violet/purple gradient using ${hex} — anti-slop list says no.`,
        fix: "Replace the gradient with a flat surface (var(--bg) or var(--surface)) or use the active accent at a single intensity, not in a gradient.",
        snippet: clip(m[0]),
      });
      break;
    }
  }
  // Also catch the literal "purple"/"violet" keyword in a linear-gradient.
  if (out.find(f => f.id === "purple-gradient") === undefined) {
    const m = /linear-gradient\([^)]*\b(purple|violet)\b[^)]*\)/i.exec(html);
    if (m) {
      out.push({
        severity: "P0",
        id: "purple-gradient",
        message: `Found a "${m[1]}" keyword inside a gradient — anti-slop.`,
        fix: "Remove the gradient or swap to a single solid color from the active design tokens.",
        snippet: clip(m[0]),
      });
    }
  }

  // ── P0-1c: blue→cyan "trust" two-stop gradient ─────────────────────
  // craft/anti-ai-slop.md documents three flavours of the two-stop
  // "trust" gradient — purple→blue, blue→cyan, indigo→pink. The first
  // and third are caught by `purple-gradient` above because the
  // relevant indigo/violet hex appears in PURPLE_HEXES, but a pure
  // blue→cyan gradient has no overlap with that list and slipped
  // past unflagged. Detect a `linear-gradient(...)` whose stop list
  // contains both a blue token (hex or keyword) and a cyan token
  // (hex or keyword). Skip if the purple-gradient rule already fired
  // so we emit a single corrective signal per artifact.
  if (out.find(f => f.id === "purple-gradient") === undefined) {
    const tg = detectBlueCyanTrustGradient(html);
    if (tg) {
      out.push({
        severity: "P0",
        id: "trust-gradient",
        message: `Found a blue→cyan two-stop "trust" gradient — anti-slop list says no.`,
        fix: "Replace the gradient with a flat surface (var(--bg) or var(--surface)) or use a single design-token color. Two-stop blue→cyan trust gradients are a SaaS hero cliché.",
        snippet: clip(tg),
      });
    }
  }

  // ── P0-1b: solid AI-default indigo as accent ──────────────────────
  // Even outside a gradient, a single use of #6366f1 et al. is the
  // textbook LLM tell. We only fire if the existing purple-gradient
  // check didn't already, since they overlap in spirit. Strip
  // token-definition blocks first: a brief whose accent is
  // intentionally indigo declares it as `--accent: #6366f1` inside
  // a selector list containing `:root` (or another known global
  // theme scope like `html` / bare `[data-theme="..."]`) and uses
  // var(--accent) downstream. That is the design system speaking,
  // not the model defaulting, and must not fire. Component-local
  // variables (e.g. `.cta { --cta-bg: #6366f1; }`) stay in scope so
  // the lint still catches indigo laundered through a local var.
  if (out.find(f => f.id === "purple-gradient") === undefined) {
    const htmlForIndigo = stripTokenBlocks(html);
    for (const hex of AI_DEFAULT_INDIGO) {
      const re = new RegExp(escapeRe(hex), "i");
      const m = re.exec(htmlForIndigo);
      if (m) {
        out.push({
          severity: "P0",
          id: "ai-default-indigo",
          message: `Found a default LLM accent color (${hex}) — this is the most-reported AI design tell.`,
          fix: "Replace with var(--accent) from the active DESIGN.md. If the brief truly requires indigo, encode it as the design system's accent so it reads as intentional, not default.",
          snippet: clip(m[0]),
        });
        break;
      }
    }
  }

  // ── P0-2: emoji used as feature/UI icons ──────────────────────────
  for (const e of SLOP_EMOJI) {
    if (html.includes(e)) {
      // Only flag if it appears in a structural context — heading,
      // button, list item — not in body prose.
      const re = new RegExp(
        `<(?:h[1-6]|button|li|span class="[^"]*icon[^"]*")[^>]*>[^<]*${escapeRe(e)}`,
        "i"
      );
      const m = re.exec(html);
      if (m) {
        out.push({
          severity: "P0",
          id: "emoji-icon",
          message: `Emoji "${e}" used as a UI icon — anti-slop list says SVG monoline only.`,
          fix: "Replace with a small inline SVG icon (1.6–1.8px stroke, currentColor) or remove the icon entirely.",
          snippet: clip(m[0]),
        });
        break;
      }
    }
  }

  // ── P0-3: rounded card with left-border accent ────────────────────
  const leftAccentRe =
    /\.[a-z-]+\s*\{[^}]*border-left\s*:\s*\d+px\s+solid\s+[^;]+;[^}]*border-radius\s*:\s*[1-9]/i;
  const lam = leftAccentRe.exec(html);
  if (lam) {
    out.push({
      severity: "P0",
      id: "left-accent-card",
      message: "Rounded card with a coloured left border — the canonical AI-slop card pattern.",
      fix: "Drop either the border-radius (set 0px) or the border-left. Cards in the OD seed use hairline borders all-round, no left accent.",
      snippet: clip(lam[0]),
    });
  }

  // ── P0-4: sans-serif display face ─────────────────────────────────
  // Skill seeds bind --font-display to a serif. Catch the case where a
  // generated artifact reverts this on h1/h2/h3 to system-sans.
  const dm = DISPLAY_SANS_RE.exec(html);
  if (dm) {
    out.push({
      severity: "P0",
      id: "sans-display",
      message:
        "A heading rule uses Inter / Roboto / system-sans as the display face — not the serif the seed binds.",
      fix: 'Use `font-family: var(--font-display)` on h1/h2/h3 and let the active design system pick the serif. Override only if the active direction is "tech / utility" or "modern minimal".',
      snippet: clip(dm[0]),
    });
  }

  // ── P0-5: invented metric phrasing ────────────────────────────────
  for (const re of INVENTED_METRIC_PATTERNS) {
    const m = re.exec(html);
    if (m) {
      out.push({
        severity: "P0",
        id: "invented-metric",
        message: `Suspected invented metric: "${m[0]}". Anti-slop list says: no numbers without a real source.`,
        fix: "Either remove the claim or replace with a placeholder (— or a labelled stub) until the user supplies a real number.",
        snippet: clip(m[0]),
      });
      break;
    }
  }

  // ── P0-6: filler / lorem text ─────────────────────────────────────
  for (const re of FILLER_PATTERNS) {
    const m = re.exec(html);
    if (m) {
      out.push({
        severity: "P0",
        id: "filler-copy",
        message: `Filler copy detected: "${m[0]}". Pages should ship with real, brief-derived copy.`,
        fix: "Replace with copy specific to the brief or delete the section entirely. An empty section is a design problem to solve with composition, not by inventing words.",
        snippet: clip(m[0]),
      });
      break;
    }
  }

  // ── P0-7: scrollIntoView (breaks iframe preview) ──────────────────
  if (/\.scrollIntoView\s*\(/.test(html)) {
    out.push({
      severity: "P0",
      id: "scroll-into-view",
      message:
        "Element.scrollIntoView() detected — yanks the host page when an iframe boundary is crossed.",
      fix: 'Use `scrollTo({ left, top, behavior: "smooth" })` on the actual scroller (see simple-deck seed for the proven pattern).',
    });
  }

  // ── P1-0: ALL-CAPS without letter-spacing ─────────────────────────
  // Refero's typography rules: any `text-transform: uppercase` rule
  // must pair with `letter-spacing: >= 0.06em` (or an absolute px
  // equivalent). Iterate every <style> block (artifacts often emit
  // a reset block followed by a tokens/components block) and scan
  // each CSS body for an uppercase declaration whose selector body
  // is missing letter-spacing or sets it visibly too low.
  // Token-aware tracking: collect per-scope `--name: value` declarations
  // from global theme scopes once, then pass them to the tracking helper
  // so a rule like `letter-spacing: var(--caps-tracking)` is judged by
  // the token's literal value in every applicable theme instead of being
  // treated as missing.
  const tokenScopes = extractCssTokens(html);
  outer: for (const styleBlock of html.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gi)) {
    // Strip CSS comments before structural matching: a `<style>` body
    // such as `/* .eyebrow { text-transform: uppercase; } */` is
    // commented-out by the browser but the rule-shaped regex below
    // would otherwise match it and emit a P1 finding for CSS that has
    // no rendered effect.
    const css = (styleBlock[1] ?? "").replace(/\/\*[\s\S]*?\*\//g, "");
    // Match a CSS rule body containing text-transform: uppercase.
    // Capture the selector + body so we can inspect tracking. The body
    // alternation is `[^{}]*` (not `[^}]*`) so the regex matches only
    // innermost `selector { body }` rules. With `[^}]*`, an outer
    // `@media (...) { .display { font-size: 48px; text-transform:
    // uppercase; … } }` matches as a single rule whose selector is the
    // `@media (...)` wrapper and whose body begins with `.display {
    // font-size: …` — so `parseDeclarations()` sees the first property
    // as `.display { font-size`, not `font-size`, the same-rule
    // font-size is lost, and `hasAdequateUppercaseTracking()` falls
    // back to the lenient inherited-size path that accepts 1px
    // tracking on a 48px heading. Restricting the body to `[^{}]*`
    // makes the regex skip the wrapper and match the inner rule
    // directly.
    const upperRe = /([^{}]*)\{([^{}]*text-transform\s*:\s*uppercase[^{}]*)\}/gi;
    let m;
    while ((m = upperRe.exec(css)) !== null) {
      const selector = (m[1] ?? "").trim();
      const body = m[2] ?? "";
      if (!hasAdequateUppercaseTracking(body, tokenScopes)) {
        out.push({
          severity: "P1",
          id: "all-caps-no-tracking",
          message: `Selector \`${selector.slice(0, 60)}\` sets text-transform: uppercase without sufficient letter-spacing (≥0.06em).`,
          fix: "Add `letter-spacing: 0.08em` (typical) to the same rule. ALL CAPS without tracking looks cramped — Refero's typography rules call this out as a top-tier amateur tell.",
          snippet: clip(`${selector} { ${body.trim()} }`),
        });
        break outer;
      }
    }
  }

  // ── P1-0b: ALL-CAPS in inline style attributes ────────────────────
  // The <style>-block scan above misses inline declarations such as
  // `<span style="text-transform: uppercase">NEW</span>`, which the
  // browser still renders ALL CAPS. craft/typography.md treats the
  // tracking floor as having no exceptions, so the inline form runs
  // through the same `hasAdequateUppercaseTracking` check used by the
  // <style>-block branch — no separate threshold. Only fire if the
  // <style>-block scan above didn't already produce this id, so the
  // agent gets a single corrective signal per artifact.
  if (out.find(f => f.id === "all-caps-no-tracking") === undefined) {
    const inlineStyleRe = /(?:^|\s)style\s*=\s*(["'])([\s\S]*?)\1/gi;
    let im;
    while ((im = inlineStyleRe.exec(html)) !== null) {
      const decl = im[2] ?? "";
      if (!/text-transform\s*:\s*uppercase/i.test(decl)) continue;
      if (!hasAdequateUppercaseTracking(decl, tokenScopes)) {
        out.push({
          severity: "P1",
          id: "all-caps-no-tracking",
          message:
            "Inline style sets text-transform: uppercase without sufficient letter-spacing (≥0.06em).",
          fix: "Add `letter-spacing: 0.08em` (typical) to the same inline style. ALL CAPS without tracking looks cramped — Refero's typography rules call this out as a top-tier amateur tell.",
          snippet: clip(decl.trim()),
        });
        break;
      }
    }
  }

  // ── P1-1: external image URLs (CDN / unsplash / placehold.co) ─────
  // Allow data: urls and same-origin paths.
  const extImg =
    /<img[^>]+src=["']https?:\/\/(?:images\.unsplash\.com|placehold\.co|placekitten\.com|via\.placeholder\.com|picsum\.photos|loremflickr\.com)/i.exec(
      html
    );
  if (extImg) {
    out.push({
      severity: "P1",
      id: "external-image",
      message: "External placeholder image CDN detected — fragile, looks fake when it 404s.",
      fix: "Use the .ph-img placeholder class shipped in the seed templates instead.",
      snippet: clip(extImg[0]),
    });
  }

  // ── P1-2: raw hex outside :root ───────────────────────────────────
  // Heuristic: count `#xxxxxx` occurrences inside the first <style> block,
  // outside the `:root{...}` declaration. Many is suspicious.
  const styleRe = /<style[^>]*>([\s\S]*?)<\/style>/i;
  const styleMatch = styleRe.exec(html);
  if (styleMatch) {
    const css = styleMatch[1] ?? "";
    const rootRe = /:root\s*\{[^}]*\}/g;
    const cssWithoutRoot = css.replace(rootRe, "");
    const hexes = cssWithoutRoot.match(/#[0-9a-fA-F]{3,8}\b/g) ?? [];
    // Allow up to ~12 raw hex values outside :root. Device chrome
    // (mobile-app frame: bezel gradient, side rails, status icons) has
    // legitimate hardware-specific values in the 8–10 range; raise the
    // threshold so seed templates pass without ceremony. More than ~12
    // signals tokens weren't honoured by the agent's generation.
    if (hexes.length > 12) {
      out.push({
        severity: "P1",
        id: "raw-hex",
        message: `${hexes.length} raw hex values found outside :root — design tokens probably not honoured.`,
        fix: "Move every color into the :root token block (--bg / --surface / --fg / --muted / --border / --accent) and reference via var(). Use color-mix() for derived tones.",
        snippet: hexes.slice(0, 6).join(" "),
      });
    }
  }

  // ── P1-3: too many accent uses in the rendered body ───────────────
  // Approximation: count `var(--accent)` references that appear OUTSIDE
  // the <style> block — i.e. inline styles in the rendered DOM, not the
  // class system definitions. The seed's <style> block defines the
  // accent on many class selectors that won't all render on one page;
  // the body is what the user actually sees.
  const styleStripped = html.replace(/<style[\s\S]*?<\/style>/gi, "");
  const accentUsesInBody = (styleStripped.match(/var\(--accent\)/g) ?? []).length;
  if (accentUsesInBody > 6) {
    out.push({
      severity: "P1",
      id: "accent-overuse",
      message: `var(--accent) used ${accentUsesInBody} times inline in the body — likely overused per screen.`,
      fix: "Cap accent usage at 2 visible uses per screen (one eyebrow + one CTA, OR one accent card + one tab). Demote the rest to var(--fg) or var(--muted).",
    });
  }

  // ── P2-1: missing comment-mode anchor on <section> ────────────────
  // Either `data-od-id` (web/mobile prototypes) or `data-screen-label`
  // (decks) counts. Whichever the artifact uses, every <section> should
  // carry one so the chat layer can target it.
  const sections = html.match(/<section\b[^>]*>/gi) ?? [];
  const tagged = sections.filter(
    s => /data-od-id\s*=/.test(s) || /data-screen-label\s*=/.test(s)
  ).length;
  if (sections.length > 0 && tagged < sections.length) {
    out.push({
      severity: "P2",
      id: "missing-section-anchor",
      message: `${sections.length - tagged} of ${sections.length} <section>s lack data-od-id (or data-screen-label).`,
      fix: 'Add data-od-id="kebab-slug" (or data-screen-label="01 Cover" for slides) to every top-level <section> so comment mode can target it.',
    });
  }

  // ── P2-2: missing slide theme classes (deck specifically) ──────────
  // Triggered only if the artifact looks deck-shaped (has .slide).
  if (/class\s*=\s*["'][^"']*\bslide\b/.test(html)) {
    const slideMatches = html.match(/<section\s+class\s*=\s*["'][^"']*\bslide\b[^"']*["']/gi) ?? [];
    const themed = slideMatches.filter(s =>
      /\b(light|dark|hero\s+light|hero\s+dark)\b/.test(s)
    ).length;
    if (slideMatches.length > 0 && themed < slideMatches.length) {
      out.push({
        severity: "P0",
        id: "slide-theme-missing",
        message: `${slideMatches.length - themed} of ${slideMatches.length} slides lack a theme class (light / dark / hero light / hero dark).`,
        fix: 'Every <section class="slide"> must include exactly one theme class. Audit your slide list and add light/dark/hero modifiers.',
      });
    }
    // Theme rhythm: no 3+ same-theme in a row.
    const themeSeq = slideMatches
      .map(s => {
        if (/hero\s+dark/.test(s)) return "HD";
        if (/hero\s+light/.test(s)) return "HL";
        if (/\bdark\b/.test(s)) return "D";
        if (/\blight\b/.test(s)) return "L";
        return "?";
      })
      .filter(t => t !== "?");
    for (let i = 0; i < themeSeq.length - 2; i++) {
      const a = themeSeq[i];
      const isLight = (t: string | undefined) => t === "L" || t === "HL";
      const isDark = (t: string | undefined) => t === "D" || t === "HD";
      if (
        (isLight(a) && isLight(themeSeq[i + 1]) && isLight(themeSeq[i + 2])) ||
        (isDark(a) && isDark(themeSeq[i + 1]) && isDark(themeSeq[i + 2]))
      ) {
        out.push({
          severity: "P1",
          id: "slide-rhythm",
          message: `Three same-theme slides in a row at position ${i + 1}–${i + 3} — visual fatigue.`,
          fix: "Swap the middle slide to the opposite theme (light → dark, or dark → light). For 8+ slides, mix in at least one hero light AND one hero dark.",
        });
        break;
      }
    }
  }

  return out;
}

/**
 * Format findings as a Markdown block ready to splice into a system
 * reminder back to the agent. P0 findings appear first.
 *
 * @param {LintFinding[]} findings
 * @returns {string}
 */
export function renderFindingsForAgent(findings: LintFinding[]): string {
  if (findings.length === 0) return "";
  const sorted = [...findings].sort((a, b) => severity(a) - severity(b));
  const lines = [
    "<artifact-lint>",
    "The artifact you just produced has the following anti-slop / design-token issues.",
    `${findings.filter(f => f.severity === "P0").length} P0 (must fix), ${findings.filter(f => f.severity === "P1").length} P1 (should fix), ${findings.filter(f => f.severity === "P2").length} P2 (nice to have).`,
    "Update the same workspace HTML file to address applicable findings. Preserve user-approved design choices and explain any justified exception.",
    "",
  ];
  for (const f of sorted) {
    lines.push(`**[${f.severity}] ${f.id}** — ${f.message}`);
    lines.push(`  Fix: ${f.fix}`);
    if (f.snippet) lines.push(`  Snippet: \`${f.snippet}\``);
    lines.push("");
  }
  lines.push("</artifact-lint>");
  return lines.join("\n");
}

function severity(f: LintFinding): number {
  return f.severity === "P0" ? 0 : f.severity === "P1" ? 1 : 2;
}
