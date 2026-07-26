---
name: AGH Warm Dark
description: Operator-first warm-dark theme. Editorial Playfair display, Inter body, JetBrains Mono accents, single ember accent (#E8572A), flat depth, no shadows.
mode: dark
---

# AGH Warm Dark

Editorial warm-dark identity for AGH presentations. Lifts tokens directly from `DESIGN.md` and `packages/ui/src/tokens.css` so slides match the runtime UI, docs, and marketing site. Single accent (`#E8572A`), flat depth (no shadows), Playfair display over Inter body, JetBrains Mono for eyebrow/labels.

## Palette

| Role         | Value       | Notes                                     |
| ------------ | ----------- | ----------------------------------------- |
| bg           | `#141312`   | canvas — page background                  |
| surface      | `#1E1C1B`   | cards, panels (one-step elevated)         |
| surfaceHi    | `#2E2C2B`   | popovers, inputs (two-step elevated)      |
| canvasDeep   | `#0E0E0F`   | code blocks, deep insets                  |
| text         | `#E5E5E7`   | primary copy                              |
| textSoft     | `#8E8E93`   | secondary copy, subtitles                 |
| muted        | `#636366`   | tertiary copy                             |
| label        | `#98989D`   | mono labels, eyebrow when not accented    |
| border       | `#3C3A39`   | hairline dividers (1px) — replace shadows |
| accent       | `#E8572A`   | single CTA / ember highlight              |
| accentInk    | `#17110F`   | text on accent fills                      |
| accentStrong | `#F6874F`   | brighter accent for highlights            |
| accentTint   | `#E8572A26` | 15% accent wash for tinted blocks         |
| selection    | `#E8572A47` | text selection (28% accent)               |

Use the accent **once per slide** as a deliberate emphasis — never as decoration. Signal colors (success/danger/warning/info) are intentionally omitted; add them ad hoc when a specific slide needs status semantics.

## Typography

- Display font: `'"Playfair Display", "Inter Variable", Georgia, serif'` — weight 400 only (editorial, never bold).
- Body font: `'"Inter Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif'` — weight 400; max weight 500 (no bold body, per DESIGN.md).
- Mono font: `'"JetBrains Mono", "SF Mono", ui-monospace, Menlo, monospace'` — weight 600 for eyebrow/badges, uppercase.

Type scale (overrides over `slide-authoring` defaults — pixel values for the 1920×1080 canvas):

- Hero title: **168 px** (Playfair, weight 400, line-height 0.96, letter-spacing −0.035em).
- Section heading: **96 px** (Playfair, weight 400, line-height 1.02, letter-spacing −0.03em).
- Page heading: **64 px** (Inter, weight 600, line-height 1.05, letter-spacing −0.02em).
- Body: **36 px** (Inter, weight 400, line-height 1.5).
- Caption / supporting label: **24 px** (Inter, weight 500, line-height 1.4).
- Eyebrow / masthead: **22 px** (JetBrains Mono, weight 600, uppercase, letter-spacing 0.16em).
- Inline mono badge: **20 px** (JetBrains Mono, weight 600, uppercase, letter-spacing 0.06em).

Font loading. Add this once near the top of the slide's `styles` string so Playfair/Inter/JetBrains Mono actually render outside the operator's OS:

```css
@import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@500;600&family=Playfair+Display:wght@400&display=swap");
```

## Layout

- Canvas: **1920 × 1080**. Absolute pixel sizing only (no `rem`/`vw`/`%` for type).
- Content padding: **120 px** standard, **80 px** for dense covers/data slides, **160 px** for breathing chapter pages.
- Alignment: left-aligned, single column. Avoid centered headlines.
- Vertical rhythm: eyebrow → **80 px** gap → title; title → **40 px** gap → subtitle; subtitle → **64 px** gap → body block.
- Dividers: 1 px hairline `#3C3A39` instead of shadows when separation is needed.
- Radii: `8 px` for cards/CTAs, `12 px` for diagram containers, `5 px` for chips. **Never `rounded-full` for CTAs**.
- No gradients (a single mesh PNG at 20% opacity is the only allowed exception). No glassmorphism. No drop shadows.

## Fixed components

Paste verbatim into a slide that uses this theme.

### Title

```tsx
const Title = ({ children }: { children: React.ReactNode }) => (
  <h1
    style={{
      fontFamily: '"Playfair Display", "Inter Variable", Georgia, serif',
      fontSize: 168,
      fontWeight: 400,
      lineHeight: 0.96,
      letterSpacing: "-0.035em",
      margin: 0,
      color: "#E5E5E7",
    }}
  >
    {children}
  </h1>
);
```

### Footer

```tsx
const Footer = ({ pageNum, total }: { pageNum: number; total: number }) => (
  <div
    style={{
      position: "absolute",
      left: 120,
      right: 120,
      bottom: 60,
      display: "flex",
      justifyContent: "space-between",
      alignItems: "center",
      fontFamily: '"JetBrains Mono", "SF Mono", ui-monospace, Menlo, monospace',
      fontSize: 22,
      fontWeight: 600,
      letterSpacing: "0.16em",
      textTransform: "uppercase",
      color: "#98989D",
    }}
  >
    <span style={{ display: "inline-flex", alignItems: "center", gap: 16 }}>
      <span style={{ width: 8, height: 8, borderRadius: 9999, background: "#E8572A" }} />
      AGH · 2026
    </span>
    <span>
      {String(pageNum).padStart(2, "0")} / {String(total).padStart(2, "0")}
    </span>
  </div>
);
```

### Eyebrow

```tsx
const Eyebrow = ({
  children,
  tone = "accent",
}: {
  children: React.ReactNode;
  tone?: "accent" | "label";
}) => (
  <div
    style={{
      fontFamily: '"JetBrains Mono", "SF Mono", ui-monospace, Menlo, monospace',
      fontSize: 22,
      fontWeight: 600,
      letterSpacing: "0.16em",
      textTransform: "uppercase",
      color: tone === "accent" ? "#E8572A" : "#98989D",
    }}
  >
    {children}
  </div>
);
```

### Hairline (depth without shadow)

```tsx
const Hairline = () => <div style={{ height: 1, width: "100%", background: "#3C3A39" }} />;
```

## Motion

Philosophy: **subtle** — minimal, purposeful. No entrance bounce, no spring, no staggered list reveals. Match the runtime UI's "editorial calm, operator density."

```css
@keyframes fadeUp {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

Apply to title + eyebrow + first paragraph only, with `cubic-bezier(0.2, 0, 0, 1)` and a duration of `200ms`. Hover transitions on any interactive element use `150ms ease-out` (the design-system base duration). Reduced motion must be respected — wrap entrance animations in `@media (prefers-reduced-motion: no-preference)`.

## Aesthetic

Editorial warm-dark, operator-first. Playfair carries display weight at 400 (never bold) over `#141312`; Inter dense at 36 px sets the body rhythm; JetBrains Mono uppercase eyebrows mark structural beats. The single ember accent `#E8572A` is a scarce resource — one mark of emphasis per slide, never decoration. References: modernist editorial print (Pentagram, Apple Pro Dark terminal, Stripe docs warm tone). Avoid: gradients, drop shadows, glassmorphism, pill CTAs, decorative emoji, neon glow, 3D type, bevels, rainbow signal-color use, centered hero compositions. Slides should read like a calm engineering document, not a keynote.

## Example usage

```tsx
import type { DesignSystem, Page } from "@open-slide/core";

export const design: DesignSystem = {
  palette: { bg: "#141312", text: "#E5E5E7", accent: "#E8572A" },
  fonts: {
    display: '"Playfair Display", "Inter Variable", Georgia, serif',
    body: '"Inter Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
  },
  typeScale: { hero: 168, body: 36 },
  radius: 8,
};

const styles = `
  @import url("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=JetBrains+Mono:wght@500;600&family=Playfair+Display:wght@400&display=swap");
  @keyframes fadeUp { from { opacity: 0; transform: translateY(16px) } to { opacity: 1; transform: translateY(0) } }
  .agh-rise { animation: fadeUp 200ms cubic-bezier(0.2, 0, 0, 1) both; }
`;

const Cover: Page = () => (
  <>
    <style>{styles}</style>
    <div
      style={{
        width: "100%",
        height: "100%",
        background: "#141312",
        color: "#E5E5E7",
        padding: 120,
        display: "flex",
        flexDirection: "column",
        justifyContent: "center",
        position: "relative",
      }}
    >
      <div className="agh-rise" style={{ marginBottom: 80 }}>
        <Eyebrow>Chapter 01</Eyebrow>
      </div>
      <div className="agh-rise">
        <Title>The Big Idea</Title>
      </div>
      <p
        className="agh-rise"
        style={{
          fontFamily:
            '"Inter Variable", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
          fontSize: 36,
          lineHeight: 1.5,
          color: "#8E8E93",
          maxWidth: 1280,
          margin: "40px 0 0",
        }}
      >
        A short subtitle that frames the slide in plain operator language — what problem, what
        shipped, what proof.
      </p>
      <Footer pageNum={1} total={5} />
    </div>
  </>
);

export default [Cover];
```
