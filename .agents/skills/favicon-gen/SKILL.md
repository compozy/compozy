---
name: favicon-gen
description: >
  Generate custom favicons from logos, text, or brand colours. Produces all required
  formats: favicon.svg, favicon.ico, apple-touch-icon.png, icon-192/512.png, and web
  manifest. Use when initialising websites, replacing CMS default favicons, converting
  logos to favicons, creating branded icons from initials, or troubleshooting favicon
  not displaying, iOS black square, or missing manifest.
compatibility: claude-code-only
---

# Favicon Generator

Generate a complete favicon package from a logo, initials, or brand colours. Produces all required formats and HTML integration code.

## Workflow

### Step 1: Choose Your Approach

```
Have a logo with an icon element?
 YES -> Extract icon from logo
 NO  -> Have text/initials?
         YES -> Create monogram favicon
         NO  -> Use branded shape
```

### Step 2: Create Source SVG

**Extracted icon** — copy icon paths from logo, centre in 32x32 viewBox, simplify for small sizes.

**Monogram** — use a template from `assets/`:
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#0066cc"/>
  <text x="16" y="21" font-size="16" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="sans-serif">AC</text>
</svg>
```

**Branded shape** — circle (universal), rounded square (modern), shield (security), hexagon (tech).

SVG templates available in `assets/` directory.

### Step 3: Generate All Formats

Requires ImageMagick (`convert` command). Install if needed: `brew install imagemagick` (macOS) or `apt install imagemagick` (Linux).

```bash
# ICO (16x16 + 32x32)
convert favicon.svg -define icon:auto-resize=16,32 favicon.ico

# Apple Touch Icon (180x180, SOLID background — transparent = black on iOS)
convert favicon.svg -resize 180x180 -background "#0066cc" -alpha remove apple-touch-icon.png

# Android/PWA icons
convert favicon.svg -resize 192x192 icon-192.png
convert favicon.svg -resize 512x512 icon-512.png
```

**No ImageMagick?** Use https://favicon.io to convert from the SVG instead.

### Step 4: Create Web Manifest

Copy and customise `assets/manifest.webmanifest`:
```json
{
  "name": "Your Business Name",
  "short_name": "Business",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ],
  "theme_color": "#0066cc",
  "background_color": "#ffffff",
  "display": "standalone"
}
```

### Step 5: Add HTML Tags

```html
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="icon" type="image/x-icon" href="/favicon.ico">
<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
<link rel="manifest" href="/site.webmanifest">
<meta name="theme-color" content="#0066cc">
```

Place all files in site root (`/public/` in Vite/React).

---

## Critical Rules

- **Always generate ALL formats** — SVG, ICO, apple-touch-icon, 192, 512, manifest
- **iOS icons MUST have solid backgrounds** — transparent = black square
- **Always use bold font weight** for monogram text (regular disappears at 16x16)
- **Test at 16x16** — if it's not legible, simplify
- **Never launch with CMS defaults** (WordPress "W", etc.)

## Format Quick Reference

| Format | Size | Transparency | Purpose |
|--------|------|-------------|---------|
| `favicon.svg` | Vector | Yes | Modern browsers |
| `favicon.ico` | 16+32 | Yes | Legacy browsers |
| `apple-touch-icon.png` | 180x180 | **No** | iOS home screen |
| `icon-192.png` | 192x192 | Yes | Android |
| `icon-512.png` | 512x512 | Yes | PWA |

## Asset Files

- `assets/favicon-svg-circle.svg` — Circle monogram template
- `assets/favicon-svg-square.svg` — Rounded square template
- `assets/favicon-svg-shield.svg` — Shield template
- `assets/manifest.webmanifest` — Web manifest template

## Reference Files

- `references/format-guide.md` — Complete format specifications
- `references/extraction-methods.md` — Logo icon extraction steps
- `references/monogram-patterns.md` — Advanced monogram design
- `references/shape-templates.md` — Industry-specific shapes with SVG code
