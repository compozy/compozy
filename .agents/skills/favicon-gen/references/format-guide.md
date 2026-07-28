# Favicon Format Guide

Complete specification for all favicon formats required for modern web applications.

---

## Overview: Required Files

A complete favicon implementation needs **6 files**:

1. `favicon.svg` - Modern browsers (vector, scalable)
2. `favicon.ico` - Legacy browsers (raster, multi-size)
3. `apple-touch-icon.png` - iOS home screen (180x180, solid background)
4. `icon-192.png` - Android home screen (192x192, transparent)
5. `icon-512.png` - PWA/high-res displays (512x512, transparent)
6. `site.webmanifest` - Web app metadata (JSON)

---

## Format 1: favicon.svg

### Purpose
- Modern browsers (Chrome 80+, Firefox 41+, Safari 9+)
- Scalable vector format (crisp at any size)
- Preferred format (browsers try this first)

### Specifications
- **File name**: `favicon.svg` (exact, case-sensitive)
- **ViewBox**: `0 0 32 32` (recommended standard)
- **Namespace**: `xmlns="http://www.w3.org/2000/svg"` (required)
- **Transparency**: Allowed (renders against browser UI)
- **Colors**: RGB or hex colors (#RRGGBB format)
- **File size**: Keep < 5KB (simpler is better)

### Template
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- Simple shapes only -->
  <!-- 3-5 elements maximum -->
  <!-- Brand colors -->
  <circle cx="16" cy="16" r="16" fill="#0066cc"/>
  <path d="M..." fill="#ffffff"/>
</svg>
```

### Best Practices
- Keep design simple (3-5 shapes max)
- Use geometric primitives when possible (`<circle>`, `<rect>`, `<polygon>`)
- Avoid gradients (may not render well at small sizes)
- Test at 16x16 zoom level
- Ensure high contrast (4.5:1 minimum)

### Browser Support
- ✅ Chrome 80+ (2020)
- ✅ Firefox 41+ (2015)
- ✅ Safari 9+ (2015)
- ✅ Edge 79+ (2020)
- ❌ IE 11 (needs ICO fallback)

---

## Format 2: favicon.ico

### Purpose
- Legacy browser support (IE 11, old mobile browsers)
- Fallback when SVG not supported
- Tab icons in Windows taskbar

### Specifications
- **File name**: `favicon.ico` (exact, case-sensitive)
- **Sizes**: MUST include both 16x16 and 32x32
- **Format**: Microsoft ICO container format
- **Color depth**: 32-bit (24-bit color + 8-bit alpha)
- **Transparency**: Supported (alpha channel)
- **File size**: Typically 1-15KB (depends on complexity)

### Multi-Size Requirement
**Critical**: ICO must contain BOTH sizes in one file:
- 16x16 - Browser tab display
- 32x32 - Retina displays, taskbar

**Why both?** Browser selects appropriate size based on context. Single-size ICO appears blurry in some contexts.

### Generation Methods

**Method 1: ImageMagick** (command line)
```bash
convert favicon.svg -define icon:auto-resize=16,32 favicon.ico
```

**Method 2: Online Converters**
- https://favicon.io (simple, free)
- https://realfavicongenerator.net (comprehensive)
- https://cloudconvert.com/svg-to-ico (batch processing)

**Method 3: Figma/Illustrator**
- Export 16x16 PNG
- Export 32x32 PNG
- Combine using online ICO creator

### Validation
Check ICO contains both sizes:
```bash
identify favicon.ico

# Expected output:
# favicon.ico[0] ICO 16x16 16x16+0+0 32-bit sRGB
# favicon.ico[1] ICO 32x32 32x32+0+0 32-bit sRGB
```

### Best Practices
- Generate from SVG (consistent with other formats)
- Test in IE 11 if legacy support needed
- Verify both 16x16 and 32x32 present
- Keep file size under 15KB

---

## Format 3: apple-touch-icon.png

### Purpose
- iOS home screen icon
- iPadOS home screen icon
- macOS Safari pinned tabs
- iOS Shortcuts app

### Specifications
- **File name**: `apple-touch-icon.png` (exact, case-sensitive)
- **Size**: 180x180 pixels (standard since iPhone 6)
- **Format**: PNG (24-bit color, lossless)
- **Transparency**: ❌ **NOT ALLOWED** (use solid background)
- **Corners**: System applies rounded corners (don't pre-round)
- **File size**: 10-50KB typical

### Critical: Solid Background Required
**Why?** iOS renders transparent PNGs with black fill, resulting in black square icon.

**Wrong** (appears as black square):
```bash
convert favicon.svg -resize 180x180 apple-touch-icon.png  # ❌ Transparent
```

**Correct** (solid background):
```bash
convert favicon.svg -resize 180x180 -background "#0066cc" -alpha remove apple-touch-icon.png  # ✅
```

### Design Guidelines
- **Background**: Use primary brand color (solid, opaque)
- **Icon size**: Scale icon to ~120x120, center on 180x180 canvas
- **Padding**: 30px on all sides (avoid edge cropping)
- **Corners**: Don't pre-round (iOS applies 18% corner radius automatically)
- **Text**: Use bold weight (regular weight too thin)

### Size History
- 57x57 - iPhone (2007)
- 72x72 - iPad (2010)
- 114x114 - iPhone 4 Retina (2010)
- 144x144 - iPad 3 Retina (2012)
- **180x180** - iPhone 6+ (2014) ← Current standard

**Note**: Only 180x180 needed now. iOS scales for older devices.

### Validation
- Open PNG in image viewer
- Check dimensions: 180x180
- Verify background is opaque (no transparency)
- Check icon centered with adequate padding
- Test on iOS device (Add to Home Screen)

---

## Format 4: icon-192.png

### Purpose
- Android home screen icon
- Chrome "Add to Home Screen" on Android
- Referenced in web app manifest

### Specifications
- **File name**: `icon-192.png` (common convention)
- **Size**: 192x192 pixels (Android standard)
- **Format**: PNG (24-bit color + alpha)
- **Transparency**: ✅ Allowed and recommended
- **File size**: 5-20KB typical

### Design Guidelines
- **Background**: Transparent OR brand color (transparent preferred for Android)
- **Icon size**: Full 192x192 canvas (Android doesn't apply padding)
- **Shape**: Can be any shape (circle, square, custom)
- **Corners**: Android applies "adaptive icon" mask on Android 8+

### Generation
```bash
# With transparency (recommended for Android)
convert favicon.svg -resize 192x192 icon-192.png

# With solid background (if brand guidelines require)
convert favicon.svg -resize 192x192 -background "#0066cc" -alpha remove icon-192.png
```

### Adaptive Icons (Android 8+)
Modern Android uses "adaptive icons" with separate foreground and background layers. For simplicity, we generate a single 192x192 PNG that works everywhere.

**Advanced**: For better Android 8+ support, provide separate foreground/background in manifest (out of scope for quick favicons).

---

## Format 5: icon-512.png

### Purpose
- Progressive Web App (PWA) splash screens
- High-resolution displays (4K, Retina)
- Android TV, Chrome OS
- "Install App" dialog

### Specifications
- **File name**: `icon-512.png` (common convention)
- **Size**: 512x512 pixels (PWA standard)
- **Format**: PNG (24-bit color + alpha)
- **Transparency**: ✅ Allowed and recommended
- **File size**: 20-80KB typical

### Design Guidelines
- Same as icon-192.png, just higher resolution
- Export from vector source (SVG) for crisp quality
- Test appearance on high-DPI displays

### Generation
```bash
convert favicon.svg -resize 512x512 icon-512.png
```

---

## Format 6: site.webmanifest

### Purpose
- Progressive Web App metadata
- Links to icon files
- Defines app name, colors, display mode
- Required for "Add to Home Screen" on Android

### Specifications
- **File name**: `site.webmanifest` or `manifest.json` (either works)
- **Format**: JSON
- **Location**: Site root (e.g., `/public/site.webmanifest`)
- **MIME type**: `application/manifest+json`

### Template
```json
{
  "name": "Your Business Name",
  "short_name": "Business",
  "description": "Brief description of your web app",
  "start_url": "/",
  "display": "standalone",
  "theme_color": "#0066cc",
  "background_color": "#ffffff",
  "icons": [
    {
      "src": "/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any maskable"
    }
  ]
}
```

### Field Descriptions

| Field | Required? | Description |
|-------|-----------|-------------|
| `name` | ✅ Yes | Full business/app name (45 chars max) |
| `short_name` | ⚠️ Recommended | Short name for home screen (12 chars max) |
| `description` | ❌ Optional | Brief app description |
| `start_url` | ⚠️ Recommended | URL to open when app launched (usually `/`) |
| `display` | ⚠️ Recommended | `standalone`, `fullscreen`, `minimal-ui`, or `browser` |
| `theme_color` | ⚠️ Recommended | Browser UI color (hex code) |
| `background_color` | ⚠️ Recommended | Splash screen background |
| `icons` | ✅ Yes | Array of icon objects (192 and 512 minimum) |

### Icons Array Requirements
Each icon object needs:
- `src` - Path to PNG file
- `sizes` - Dimensions (e.g., "192x192")
- `type` - MIME type ("image/png")
- `purpose` - "any maskable" (supports adaptive icons)

**Minimum**: 192x192 and 512x512

### Validation
Test manifest at: https://manifest-validator.appspot.com/

---

## HTML Integration

### Complete `<head>` Tags

```html
<!-- Modern browsers (SVG, preferred) -->
<link rel="icon" type="image/svg+xml" href="/favicon.svg">

<!-- Legacy fallback (ICO) -->
<link rel="icon" type="image/x-icon" href="/favicon.ico">

<!-- Apple Touch Icon (iOS, iPadOS, macOS) -->
<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">

<!-- Web App Manifest (Android, PWA) -->
<link rel="manifest" href="/site.webmanifest">

<!-- Theme Color (browser UI) -->
<meta name="theme-color" content="#0066cc">
```

### Tag Order Matters
1. SVG first (modern browsers try this)
2. ICO second (fallback for legacy)
3. Apple touch icon (iOS-specific)
4. Manifest (Android/PWA)

### File Locations
All files should be in site root:
- Vite/React: `/public/` directory
- Next.js: `/public/` directory
- Static sites: Root directory
- WordPress: Theme root or upload to `/wp-content/uploads/`

---

## Browser/Platform Support Matrix

| Format | Chrome | Firefox | Safari | Edge | iOS | Android |
|--------|--------|---------|--------|------|-----|---------|
| `favicon.svg` | ✅ 80+ | ✅ 41+ | ✅ 9+ | ✅ 79+ | ✅ 9+ | ✅ 80+ |
| `favicon.ico` | ✅ All | ✅ All | ✅ All | ✅ All | ✅ All | ✅ All |
| `apple-touch-icon.png` | ❌ | ❌ | ✅ Yes | ❌ | ✅ Yes | ❌ |
| `icon-192.png` (manifest) | ✅ Yes | ❌ | ❌ | ✅ Yes | ❌ | ✅ Yes |
| `icon-512.png` (manifest) | ✅ Yes | ❌ | ❌ | ✅ Yes | ❌ | ✅ Yes |
| `site.webmanifest` | ✅ 39+ | ❌ | ❌ | ✅ 79+ | ❌ | ✅ 39+ |

**Key Insight**: You need ALL formats to support all platforms. No single format works everywhere.

---

## File Size Guidelines

| Format | Target Size | Maximum Size | Typical Size |
|--------|-------------|--------------|--------------|
| `favicon.svg` | < 2KB | 5KB | 1-3KB |
| `favicon.ico` | < 10KB | 15KB | 5-10KB |
| `apple-touch-icon.png` | < 30KB | 50KB | 10-30KB |
| `icon-192.png` | < 15KB | 25KB | 5-15KB |
| `icon-512.png` | < 50KB | 100KB | 20-60KB |
| `site.webmanifest` | < 1KB | 2KB | 0.5-1KB |

**Why size matters**: Favicons load on every page, before render. Smaller = faster initial load.

---

## Testing Checklist

### Visual Tests
- [ ] Browser tab at 100% zoom (16x16 display)
- [ ] Browser tab at 200% zoom (32x32 display)
- [ ] Bookmarks bar
- [ ] iOS "Add to Home Screen"
- [ ] Android "Add to Home Screen"
- [ ] PWA install dialog
- [ ] Light mode AND dark mode (if using transparency)

### Technical Tests
- [ ] All 6 files generated
- [ ] Files in correct locations (site root)
- [ ] HTML `<link>` tags present in `<head>`
- [ ] ICO contains both 16x16 and 32x32
- [ ] apple-touch-icon has solid background (no transparency)
- [ ] Manifest JSON is valid
- [ ] Hard refresh browser (Ctrl+Shift+R)
- [ ] Test in incognito window (no cache)

### Validation Tools
- **Favicon test**: https://realfavicongenerator.net/favicon_checker
- **Manifest validator**: https://manifest-validator.appspot.com/
- **ICO inspection**: `identify favicon.ico` (ImageMagick)
- **SVG validation**: https://validator.w3.org/

---

## Common Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| Only SVG, no ICO | No favicon in IE11, old browsers | Generate ICO fallback |
| ICO has only one size | Blurry at some display sizes | Generate 16x16 AND 32x32 |
| Transparent apple-touch-icon | Black square on iOS | Use solid background color |
| Missing manifest | Android "Add to Home Screen" broken | Create site.webmanifest |
| Wrong file names | Favicons don't load | Use exact names (case-sensitive) |
| Files in wrong location | 404 errors | Place in site root |
| Skipped hard refresh | Old favicon cached | Ctrl+Shift+R to test |

---

## Quick Reference Card

**6 Required Files**:
```
favicon.svg         # Modern browsers (vector)
favicon.ico         # Legacy (16+32px raster)
apple-touch-icon.png  # iOS (180x180, solid background)
icon-192.png        # Android (192x192)
icon-512.png        # PWA (512x512)
site.webmanifest    # Metadata (JSON)
```

**HTML Tags**:
```html
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="icon" type="image/x-icon" href="/favicon.ico">
<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png">
<link rel="manifest" href="/site.webmanifest">
<meta name="theme-color" content="#0066cc">
```

**Generation Commands**:
```bash
# ICO with both sizes
convert favicon.svg -define icon:auto-resize=16,32 favicon.ico

# iOS icon (solid background)
convert favicon.svg -resize 180x180 -background "#0066cc" -alpha remove apple-touch-icon.png

# Android/PWA icons (transparent)
convert favicon.svg -resize 192x192 icon-192.png
convert favicon.svg -resize 512x512 icon-512.png
```

---

## Official References

- **MDN Favicon**: https://developer.mozilla.org/en-US/docs/Glossary/Favicon
- **Apple Touch Icon**: https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html
- **Web App Manifest**: https://w3c.github.io/manifest/
- **ICO Format**: https://en.wikipedia.org/wiki/ICO_(file_format)
- **SVG Support**: https://caniuse.com/link-icon-svg
- **PWA Icons**: https://web.dev/add-manifest/#icons

---

**Last Updated**: 2026-01-14
**Maintained by**: Jezweb (jeremy@jezweb.net)
