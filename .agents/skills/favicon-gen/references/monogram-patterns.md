# Monogram Favicon Patterns

Comprehensive guide for creating text-based favicons when no logo icon exists.

---

## When to Use Monograms

### ✅ Use Monogram Favicons When:
- Business has text-only logo (no icon element)
- Logo is too complex to simplify into 16×16 favicon
- Need temporary favicon before logo finalized
- Brand identity is strongly tied to initials/acronym
- Industry standard uses lettermarks (law firms, consulting)

### ❌ Don't Use Monograms When:
- Logo has extractable icon element (use extraction-methods.md)
- Business name is single generic word ("Solutions", "Services")
- Initials are confusing (e.g., "XYZ Corp" → "XC" not "XYZ")
- Client has strong preference against lettermarks

---

## Letter Selection Rules

Choosing the right letters makes or breaks monogram readability.

### Single-Word Business Names

| Business Name | Options | Best Choice | Reasoning |
|---------------|---------|-------------|-----------|
| **Acme** | A, AC, ACM | **A** | Single letter cleanest |
| **Stratton** | S, ST, STR | **S** | Single letter sufficient |
| **Microsoft** | M, MS, MIC | **M** | Single letter iconic |
| **Amazon** | A, AM, AMZ | **A** | Single letter + arrow better |

**Rule**: For single words, use first letter UNLESS:
- First letter is too common (A, S, M) → add second letter
- Acronym is well-known (FBI, NASA) → use full acronym

### Two-Word Business Names

| Business Name | Options | Best Choice | Reasoning |
|---------------|---------|-------------|-----------|
| **Blue Sky** | B, BS, BSK | **BS** | Both initials balanced |
| **Red Fox** | R, RF, RFX | **RF** | Clean two-letter combo |
| **Digital Ocean** | D, DO, DIG | **DO** | Two-letter balanced |
| **Creative Agency** | C, CA, CRA | **CA** | Standard initials |

**Rule**: Use both initials (first letter of each word) for two-word names.

### Three-Word Business Names

| Business Name | Options | Best Choice | Reasoning |
|---------------|---------|-------------|-----------|
| **Big Red Box** | B, BR, BRB | **BR** | Drop last initial |
| **North Star Digital** | N, NS, NSD | **NS** | First two words |
| **Blue Sky Consulting** | B, BS, BSC | **BS** | First two words |

**Rule**: Use first two initials (drop third) OR use all three if each is critical to identity.

**Exception**: If three-letter acronym is established (FBI, BMW, IBM), use all three.

### Acronym Business Names

| Business Name | Monogram | Rule |
|---------------|----------|------|
| **FBI** | FBI | Use full acronym (established) |
| **NASA** | NASA | Use full acronym (iconic) |
| **HSBC** | HSBC | Use full acronym (brand) |
| **BMW** | BMW | Use full acronym (3 letters) |

**Rule**: If business commonly uses acronym, use full acronym as monogram.

### Special Cases

**Ampersand names** ("Smith & Jones"):
- Use initials only: SJ
- Omit the "&" (too detailed for small sizes)

**Names with "The"** ("The Agency"):
- Omit "The", use remaining initials: A or AG

**Hyphenated names** ("Blue-Green Design"):
- Treat as two words: BG or BGD

**Numbers in name** ("7 Eleven", "22 Designs"):
- Use number + letter: 7E, 22D
- OR omit number, use letters only: E, D

---

## Font Selection

### Web-Safe Font Recommendations

| Font Family | Character | Best For | Weight |
|-------------|-----------|----------|--------|
| **Arial** | Clean, neutral | Corporate, professional | 700 (bold) |
| **Helvetica** | Classic, refined | Design agencies, premium | 700 (bold) |
| **Verdana** | Wide, readable | 1-2 letters max | 700 (bold) |
| **Georgia** | Serif, elegant | Law, finance, traditional | 700 (bold) |
| **Trebuchet MS** | Modern, humanist | Tech, startups | 700 (bold) |

**Rule**: Always use `font-weight="bold"` (700). Regular (400) weight disappears at 16×16.

### Font Size by Letter Count

| Letters | Font Size (32px canvas) | Approximate Fill |
|---------|-------------------------|------------------|
| **1 letter** | 18-20px | ~60% height |
| **2 letters** | 14-16px | ~50% height |
| **3 letters** | 11-13px | ~40% height |
| **4+ letters** | ❌ Too many | Illegible |

**Rule**: Maximum 3 letters. More = illegible at 16×16.

### Letter Spacing (Kerning)

For 2-3 letter monograms, adjust spacing:

```xml
<!-- Too tight (default) -->
<text x="16" y="21" font-size="16" text-anchor="middle">BS</text>

<!-- Better (add letter-spacing) -->
<text x="16" y="21" font-size="16" text-anchor="middle" letter-spacing="1">BS</text>
```

**Guideline**: 0.5-2px letter-spacing for 2-3 letters (test visually)

### Uppercase vs Lowercase

| Style | Use Case | Example |
|-------|----------|---------|
| **Uppercase** | Professional, corporate, finance | AC, BS, RF |
| **Lowercase** | Friendly, modern, tech | ac, bs, rf |
| **Title Case** | Rare, casual | Ac, Bs, Rf |

**Rule**: Uppercase is default for professional contexts. Lowercase for casual/modern brands.

---

## Shape Templates

Monograms need containing shapes for definition.

### Circle (Universal)

**Use for**: Any industry, safe default

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#0066cc"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">A</text>
</svg>
```

**Characteristics**:
- Most versatile shape
- Works on any background
- Universally recognized
- iOS rounds corners anyway

### Rounded Square

**Use for**: Modern, tech, apps

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect x="0" y="0" width="32" height="32" rx="6" fill="#0066cc"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">A</text>
</svg>
```

**Characteristics**:
- Matches iOS icon style
- Modern, app-like
- `rx="6"` = 18.75% corner radius (standard)

### Shield

**Use for**: Security, trust, protection, legal

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M16,2 L28,8 L28,16 C28,24 16,30 16,30 C16,30 4,24 4,16 L4,8 Z"
        fill="#0066cc"/>
  <text x="16" y="20" font-size="16" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">S</text>
</svg>
```

**Characteristics**:
- Implies security/protection
- Good for legal, finance, security industries
- More complex shape (test at 16×16)

### Hexagon

**Use for**: Tech, engineering, data, science

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <polygon points="16,2 28,9 28,23 16,30 4,23 4,9"
           fill="#0066cc"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">H</text>
</svg>
```

**Characteristics**:
- Technical, geometric feel
- Popular in tech/SaaS
- Six-sided symmetry

### Diamond/Rhombus

**Use for**: Luxury, premium, jewelry

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <polygon points="16,2 30,16 16,30 2,16"
           fill="#0066cc"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">D</text>
</svg>
```

**Characteristics**:
- Elegant, premium
- Diamond/gem association
- Good for luxury brands

---

## Color Patterns

### High-Contrast Combinations (Recommended)

| Background | Text | Contrast Ratio | Use Case |
|------------|------|----------------|----------|
| **#0066cc** (Blue) | #ffffff (White) | 6.3:1 ✅ | Corporate, trust |
| **#1a1a1a** (Dark gray) | #ffffff (White) | 17.9:1 ✅ | Modern, minimal |
| **#d62828** (Red) | #ffffff (White) | 5.5:1 ✅ | Bold, urgent |
| **#2a9d8f** (Teal) | #ffffff (White) | 4.5:1 ✅ | Fresh, health |
| **#7209b7** (Purple) | #ffffff (White) | 8.6:1 ✅ | Creative, premium |
| **#ffffff** (White) | #1a1a1a (Dark) | 17.9:1 ✅ | Light mode sites |

**Rule**: Minimum 4.5:1 contrast ratio (WCAG AA). Test at https://webaim.org/resources/contrastchecker/

### Brand-Matched Colors

Match favicon to existing brand palette:

**Primary brand color → Background**
**White or contrasting brand color → Text**

Example:
- Brand: Navy (#003366) + Gold (#c5a778)
- Favicon: Navy background, gold text
- Alternative: Gold background, navy text (test contrast)

### Gradient Backgrounds (Use Sparingly)

Gradients can look muddy at small sizes. If using:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <defs>
    <linearGradient id="grad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#0066cc;stop-opacity:1" />
      <stop offset="100%" style="stop-color:#003366;stop-opacity:1" />
    </linearGradient>
  </defs>
  <circle cx="16" cy="16" r="16" fill="url(#grad)"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">A</text>
</svg>
```

**Guidelines**:
- Use 2-stop gradients only (more = muddy)
- Similar colors (same hue family)
- Test at 16×16 (gradient may be invisible)
- Prefer solid colors when in doubt

---

## Industry-Specific Recommendations

### Technology / SaaS

- **Shape**: Hexagon or rounded square
- **Colors**: Blues (#0066cc, #4a90e2), teals (#00a896)
- **Letters**: 1-2 max
- **Font**: Helvetica, Arial (clean, modern)

### Finance / Legal

- **Shape**: Square or shield
- **Colors**: Navy (#003366), dark blue (#1a237e), gold (#c5a778)
- **Letters**: 2-3 (firm initials)
- **Font**: Georgia (serif, traditional)

### Healthcare / Medical

- **Shape**: Circle or rounded square
- **Colors**: Medical blue (#0077c8), green (#00a651)
- **Letters**: 1-2
- **Font**: Arial, Helvetica (clean, trustworthy)

### Creative / Agency

- **Shape**: Circle, diamond, or custom
- **Colors**: Bold colors (purple #7209b7, pink #f72585, orange #ff6b35)
- **Letters**: 1-2
- **Font**: Trebuchet MS, Helvetica (modern)

### Real Estate / Property

- **Shape**: Square or house outline (custom)
- **Colors**: Earth tones (#8b4513), blues (#4a90e2)
- **Letters**: 1-2
- **Font**: Arial (approachable)

### Food / Restaurant

- **Shape**: Circle or rounded square
- **Colors**: Warm colors (red #d62828, orange #ff6b35, yellow #ffd60a)
- **Letters**: 1-2
- **Font**: Verdana, Arial (friendly, readable)

### Education

- **Shape**: Circle or square
- **Colors**: Blue (#1976d2), green (#388e3c)
- **Letters**: 1-3 (school initials)
- **Font**: Arial, Georgia (traditional)

### Retail / E-commerce

- **Shape**: Rounded square or circle
- **Colors**: Brand-specific (match packaging)
- **Letters**: 1-2
- **Font**: Arial, Trebuchet MS

---

## Advanced Monogram Techniques

### Ligature Monograms

Connecting multiple letters into single shape:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#0066cc"/>
  <!-- Custom paths forming connected AB -->
  <path d="M8,20 L8,12 L12,12 Q14,12 14,14 Q14,16 12,16 L8,16 M12,16 Q14,16 14,18 Q14,20 12,20 L8,20 M14,20 L14,12 Q14,10 18,10 Q22,10 22,14 L22,20" fill="#ffffff" stroke="#ffffff" stroke-width="1.5"/>
</svg>
```

**When to use**:
- Brand has established ligature logo
- Professional designer available
- 2-letter combination only (3+ too complex)

**Complexity warning**: Custom paths harder to maintain than text elements.

### Negative Space Monograms

Letter cut out of solid shape:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <defs>
    <mask id="cutout">
      <rect width="32" height="32" fill="#ffffff"/>
      <text x="16" y="21" font-size="18" font-weight="bold"
            text-anchor="middle" fill="#000000" font-family="Arial, sans-serif">A</text>
    </mask>
  </defs>
  <circle cx="16" cy="16" r="16" fill="#0066cc" mask="url(#cutout)"/>
</svg>
```

**Effect**: Letter appears as transparent/cutout of shape

**Use case**: Modern, minimal designs

### Two-Color Letter Treatment

Split letter into two colors:

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" fill="#0066cc"/>
  <!-- Top half of letter -->
  <text x="16" y="21" font-size="20" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif"
        clip-path="polygon(0 0, 100% 0, 100% 50%, 0 50%)">A</text>
  <!-- Bottom half of letter (different color) -->
  <text x="16" y="21" font-size="20" font-weight="bold"
        text-anchor="middle" fill="#ffcc00" font-family="Arial, sans-serif"
        clip-path="polygon(0 50%, 100% 50%, 100% 100%, 0 100%)">A</text>
</svg>
```

**Effect**: Letter split horizontally with two colors

**Complexity**: High, test at 16×16 carefully

---

## Text Positioning

### Vertical Centering

SVG `<text>` positions by baseline, not center. Adjust `y` coordinate:

```xml
<!-- For 32px viewBox -->
<text x="16" y="21" ...>  <!-- y = 16 (center) + 5 (offset) = 21 -->

<!-- For different font sizes -->
<!-- y ≈ 16 + (fontSize * 0.35) -->
```

**Rule of thumb**: `y = viewBox height / 2 + font size * 0.35`

### Horizontal Centering

Use `text-anchor="middle"` and `x` at center:

```xml
<text x="16" y="21" text-anchor="middle" ...>
```

**Always** use `text-anchor="middle"` for centered monograms (default is `start`).

---

## Testing Monograms

### Visual Tests

1. **16×16 zoom**: Can you read the letters?
2. **32×32 zoom**: Do letters look crisp?
3. **Light background**: Does it have contrast?
4. **Dark background**: Does it stand out?
5. **Colorblind test**: Run through colorblind simulator

### Technical Tests

1. **Font render**: Does font load in all browsers?
2. **File size**: SVG should be < 2KB for monograms
3. **Export test**: Generate ICO/PNGs, check quality

### A/B Test with Colleagues

Show 3-5 options:
- Different shapes (circle vs square vs shield)
- Different letter counts (A vs AC vs ACM)
- Different colors (primary vs secondary brand colors)

**Ask**: Which is most recognizable? Which matches brand?

---

## Common Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| **Too many letters** (4+) | Illegible | Max 3 letters |
| **Regular font weight** | Letters disappear | Use bold (700) |
| **Wrong font size** | Too big or too small | Follow size guidelines (18-20px for 1 letter) |
| **Low contrast** | Hard to read | Minimum 4.5:1 contrast |
| **Lowercase only** | Less readable | Use uppercase for professional contexts |
| **No shape** | Letter floats | Always use containing shape |
| **Complex ligature** | Muddy at small sizes | Keep simple or use separate letters |

---

## Monogram Template Library

Use these copy-paste templates as starting points:

### Single Letter, Circle, Blue/White
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#0066cc"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">A</text>
</svg>
```

### Two Letters, Rounded Square, Dark/White
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#1a1a1a"/>
  <text x="16" y="21" font-size="16" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif"
        letter-spacing="1">AB</text>
</svg>
```

### Three Letters, Shield, Navy/Gold
```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M16,2 L28,8 L28,16 C28,24 16,30 16,30 C16,30 4,24 4,16 L4,8 Z"
        fill="#003366"/>
  <text x="16" y="20" font-size="12" font-weight="bold"
        text-anchor="middle" fill="#c5a778" font-family="Georgia, serif"
        letter-spacing="0.5">ABC</text>
</svg>
```

**Usage**: Copy template, change letters, colors, and shape as needed.

---

## Quick Reference

**Letter Selection**:
- 1 word → 1 letter
- 2 words → 2 letters (both initials)
- 3 words → 2 letters (first two initials)
- Acronym → full acronym (FBI, BMW, IBM)

**Font Sizes**:
- 1 letter → 18-20px
- 2 letters → 14-16px
- 3 letters → 11-13px
- 4+ letters → ❌ Too many

**Shapes**:
- Circle → universal, safe
- Rounded square → modern, apps
- Shield → security, legal, trust
- Hexagon → tech, engineering

**Colors**:
- Minimum 4.5:1 contrast
- Match brand colors
- Default: Blue bg + white text

**Font**: Always `font-weight="bold"` or 700

---

## Official References

- **WCAG Contrast**: https://webaim.org/resources/contrastchecker/
- **SVG Text**: https://developer.mozilla.org/en-US/docs/Web/SVG/Element/text
- **Google Fonts**: https://fonts.google.com/ (for inspiration)
- **Monogram Design Principles**: https://www.smashingmagazine.com/2015/08/understanding-typography-monogram-design/

---

**Last Updated**: 2026-01-14
**Maintained by**: Jezweb (jeremy@jezweb.net)
