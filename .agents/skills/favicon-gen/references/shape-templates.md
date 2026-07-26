# Shape Templates for Favicons

Industry-specific shapes and custom favicon templates beyond basic circles and squares.

---

## Overview

While circles and rounded squares work universally, industry-specific shapes can reinforce brand identity and improve recognition.

**When to use custom shapes**:
- Industry has visual conventions (house for real estate, shield for security)
- Brand personality calls for unique shape
- Client requests specific shape
- Differentiation needed in crowded market

**When to use standard shapes**:
- Unclear industry fit
- Risk of complexity at small sizes
- Budget/time constraints
- Client has no preference

---

## Shape Selection by Industry

| Industry | Primary Shape | Alternative | Rationale |
|----------|---------------|-------------|-----------|
| **Technology** | Hexagon | Circuit | Precision, structure |
| **Security** | Shield | Lock | Protection, safety |
| **Real Estate** | House | Location pin | Property, home |
| **Healthcare** | Cross | Heart | Medical, care |
| **Finance** | Dollar sign | Bar chart | Money, growth |
| **Legal** | Scales | Gavel | Justice, law |
| **Education** | Book | Graduation cap | Learning, knowledge |
| **Food** | Fork/knife | Chef hat | Dining, culinary |
| **Travel** | Airplane | Compass | Journey, adventure |
| **Retail** | Shopping bag | Tag | Commerce, products |
| **Construction** | Hard hat | Wrench | Building, tools |
| **Creative** | Brush | Lightbulb | Art, ideas |

---

## Template 1: Shield (Security, Legal, Protection)

### Standard Shield

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- Shield outline -->
  <path d="M16,2 L28,8 L28,16 C28,24 16,30 16,30 C16,30 4,24 4,16 L4,8 Z"
        fill="#0066cc"/>
  <!-- Optional: Add letter/icon inside -->
  <text x="16" y="20" font-size="16" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">S</text>
</svg>
```

**Characteristics**:
- Top-heavy design (wider at top)
- Point at bottom
- Symmetrical

**Use for**: Security companies, legal firms, insurance, protection services

### Badge Shield (Police/Authority Style)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M16,2 L20,6 L26,6 L28,12 L28,18 L24,26 L16,30 L8,26 L4,18 L4,12 L6,6 L12,6 Z"
        fill="#1a237e" stroke="#ffd700" stroke-width="1"/>
  <text x="16" y="20" font-size="14" font-weight="bold"
        text-anchor="middle" fill="#ffd700" font-family="Arial, sans-serif">P</text>
</svg>
```

**Use for**: Law enforcement, security agencies, authority brands

---

## Template 2: House (Real Estate, Home Services)

### Simple House

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- Roof -->
  <polygon points="16,4 30,16 26,16 26,28 6,28 6,16 2,16"
           fill="#4a90e2"/>
  <!-- Door -->
  <rect x="13" y="20" width="6" height="8" fill="#ffffff"/>
</svg>
```

**Characteristics**:
- Triangular roof
- Rectangular body
- Optional door/window details

**Use for**: Real estate agencies, home builders, property management, home services

### Modern House (Minimalist)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M2,16 L16,4 L30,16 L30,28 L2,28 Z"
        fill="#00a896"/>
  <circle cx="16" cy="20" r="3" fill="#ffffff"/>
</svg>
```

**Simpler variant** for cleaner look at small sizes.

---

## Template 3: Medical Cross

### Standard Cross

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#0077c8"/>
  <path d="M16,8 L16,24 M8,16 L24,16"
        stroke="#ffffff" stroke-width="4" stroke-linecap="round"/>
</svg>
```

**Characteristics**:
- Equal-length arms
- Centered
- Round or square ends

**Use for**: Healthcare, medical clinics, hospitals, first aid

### Plus Icon (Modern Medical)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#00a651"/>
  <path d="M12,16 L20,16 M16,12 L16,20"
        stroke="#ffffff" stroke-width="3" stroke-linecap="round"/>
</svg>
```

**Simpler, more modern** variant.

---

## Template 4: Hexagon (Tech, Engineering, Data)

### Regular Hexagon

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <polygon points="16,2 28,9 28,23 16,30 4,23 4,9"
           fill="#7209b7"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">H</text>
</svg>
```

**Characteristics**:
- Six equal sides
- Technical/geometric feel
- Popular in tech/SaaS

**Use for**: Software companies, data analytics, engineering firms, tech startups

### Flat-Top Hexagon

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <polygon points="8,2 24,2 32,16 24,30 8,30 0,16"
           fill="#4a90e2"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif">T</text>
</svg>
```

**Orientation**: Flat top/bottom (vs pointy top in regular)

---

## Template 5: Diamond (Luxury, Premium, Jewelry)

### Standard Diamond

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <polygon points="16,2 30,16 16,30 2,16"
           fill="#c5a778"/>
  <text x="16" y="21" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#1a1a1a" font-family="Georgia, serif">D</text>
</svg>
```

**Characteristics**:
- 45-degree rotated square
- Elegant, premium feel
- Point at top/bottom

**Use for**: Jewelry stores, luxury brands, premium services, high-end retail

### Gem Diamond (Faceted)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- Diamond outline -->
  <polygon points="16,2 30,16 16,30 2,16" fill="#e0e0e0"/>
  <!-- Facet lines -->
  <path d="M16,2 L16,30 M2,16 L30,16 M6,8 L26,24 M26,8 L6,24"
        stroke="#ffffff" stroke-width="1" opacity="0.3"/>
</svg>
```

**More complex**: Test at 16×16 carefully.

---

## Template 6: Location Pin (Travel, Local Services)

### Standard Pin

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#2a9d8f"/>
  <path d="M16,8 C12,8 10,10 10,13 C10,16 16,22 16,22 C16,22 22,16 22,13 C22,10 20,8 16,8 Z"
        fill="#ffffff"/>
  <circle cx="16" cy="13" r="2" fill="#2a9d8f"/>
</svg>
```

**Characteristics**:
- Teardrop shape with circle at top
- Point at bottom
- Iconic "map marker" design

**Use for**: Travel agencies, local services, mapping apps, delivery services

### Simple Pin

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <path d="M16,4 C11,4 8,7 8,11 C8,16 16,28 16,28 C16,28 24,16 24,11 C24,7 21,4 16,4 Z"
        fill="#e76f51"/>
  <circle cx="16" cy="11" r="3" fill="#ffffff"/>
</svg>
```

**Simpler variant** with no background circle.

---

## Template 7: Lightbulb (Ideas, Innovation, Consulting)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#ffd60a"/>
  <!-- Bulb -->
  <circle cx="16" cy="14" r="5" fill="#1a1a1a"/>
  <!-- Base -->
  <rect x="14" y="19" width="4" height="4" rx="1" fill="#1a1a1a"/>
  <!-- Rays (optional, test at 16x16) -->
  <path d="M16,5 L16,8 M11,8 L12,10 M8,11 L10,12 M24,11 L22,12 M21,8 L20,10"
        stroke="#1a1a1a" stroke-width="1.5" stroke-linecap="round"/>
</svg>
```

**Use for**: Consulting, creative agencies, innovation labs, idea platforms

**Complexity warning**: Rays may be too detailed at 16×16. Test and simplify if needed.

---

## Template 8: Gear/Cog (Manufacturing, Engineering, Services)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#003366"/>
  <!-- Simplified gear (8 teeth) -->
  <path d="M16,6 L18,10 L22,10 L18,14 L20,18 L16,16 L12,18 L14,14 L10,10 L14,10 Z"
        fill="#ffffff"/>
  <circle cx="16" cy="16" r="3" fill="#003366"/>
</svg>
```

**Use for**: Manufacturing, engineering firms, maintenance services, automation

**Simplification critical**: Real gears have 12-20 teeth; use 6-8 for favicons.

---

## Template 9: Book (Education, Publishing, Libraries)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#1976d2"/>
  <!-- Book cover -->
  <rect x="10" y="8" width="12" height="16" rx="1" fill="#ffffff"/>
  <!-- Pages -->
  <path d="M12,10 L12,22 M14,10 L14,22"
        stroke="#1976d2" stroke-width="0.5"/>
</svg>
```

**Use for**: Schools, libraries, publishers, educational platforms, online courses

---

## Template 10: Shopping Bag (Retail, E-commerce)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#d62828"/>
  <!-- Bag body -->
  <rect x="9" y="12" width="14" height="12" rx="1" fill="#ffffff"/>
  <!-- Handles -->
  <path d="M12,12 C12,9 14,8 16,8 C18,8 20,9 20,12"
        stroke="#ffffff" stroke-width="2" fill="none"/>
</svg>
```

**Use for**: Retail stores, e-commerce sites, shopping platforms, marketplaces

---

## Template 11: Envelope (Email Services, Communication)

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <circle cx="16" cy="16" r="16" fill="#4a90e2"/>
  <!-- Envelope -->
  <rect x="6" y="11" width="20" height="14" rx="1" fill="#ffffff"/>
  <!-- Flap -->
  <path d="M6,11 L16,18 L26,11" stroke="#4a90e2" stroke-width="2" fill="none"/>
</svg>
```

**Use for**: Email services, communication platforms, messaging apps, mail services

---

## Template 12: Chart/Graph (Analytics, Finance, Data)

### Bar Chart

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#00a896"/>
  <!-- 3 bars of different heights -->
  <rect x="8" y="18" width="4" height="8" fill="#ffffff" rx="1"/>
  <rect x="14" y="12" width="4" height="14" fill="#ffffff" rx="1"/>
  <rect x="20" y="15" width="4" height="11" fill="#ffffff" rx="1"/>
</svg>
```

**Use for**: Analytics platforms, finance apps, data visualization, business intelligence

### Line Chart

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <rect width="32" height="32" rx="6" fill="#7209b7"/>
  <!-- Simplified line graph -->
  <path d="M6,22 L12,18 L18,20 L24,12"
        stroke="#ffffff" stroke-width="3" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
  <!-- Data points -->
  <circle cx="6" cy="22" r="2" fill="#ffffff"/>
  <circle cx="12" cy="18" r="2" fill="#ffffff"/>
  <circle cx="18" cy="20" r="2" fill="#ffffff"/>
  <circle cx="24" cy="12" r="2" fill="#ffffff"/>
</svg>
```

---

## Custom Shape Creation Guidelines

### When Creating New Shapes

1. **Start with basic geometric primitives**
   - Circles, rectangles, triangles, polygons
   - Combine 2-4 shapes maximum

2. **Test at 16×16 early and often**
   - Details disappear at small sizes
   - Simplify aggressively

3. **Ensure symmetry when possible**
   - Symmetrical shapes are easier to recognize
   - Better visual balance

4. **High contrast required**
   - Foreground vs background: 4.5:1 minimum
   - Test on both light and dark browser UIs

5. **Keep paths simple**
   - Minimize anchor points
   - Use straight lines over curves when possible
   - Avoid complex bezier curves

### Shape Complexity Guidelines

| Complexity Level | Description | Use Case |
|------------------|-------------|----------|
| **Simple** (3-5 shapes) | Circle + icon | Universal |
| **Medium** (6-10 shapes) | House, shield, hexagon | Industry-specific |
| **Complex** (11+ shapes) | Gear, lightbulb with rays | ⚠️ Use cautiously |

**Rule**: Simpler is always better at favicon sizes. If unsure, simplify more.

---

## Combining Shapes with Text

### Icon + Letter

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- Shield background -->
  <path d="M16,2 L28,8 L28,16 C28,24 16,30 16,30 C16,30 4,24 4,16 L4,8 Z"
        fill="#003366"/>
  <!-- Letter inside -->
  <text x="16" y="20" font-size="16" font-weight="bold"
        text-anchor="middle" fill="#c5a778" font-family="Arial, sans-serif">L</text>
</svg>
```

**Use for**: Professional services that need letter + industry shape (e.g., "Smith Legal" → "S" in shield)

### Icon + Monogram

```xml
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
  <!-- House shape -->
  <polygon points="16,4 30,16 26,16 26,28 6,28 6,16 2,16" fill="#4a90e2"/>
  <!-- Initials -->
  <text x="16" y="24" font-size="10" font-weight="bold"
        text-anchor="middle" fill="#ffffff" font-family="Arial, sans-serif"
        letter-spacing="0.5">RG</text>
</svg>
```

**Use for**: "Realtor Group" → "RG" initials in house

**Font size critical**: 10-12px max when combining with detailed shape.

---

## Testing Custom Shapes

### Visual Test Checklist

- [ ] Recognizable at 16×16 pixels
- [ ] Still recognizable at 512×512 (scales well)
- [ ] Works on light browser UI (Chrome light mode)
- [ ] Works on dark browser UI (dark mode)
- [ ] High contrast (4.5:1 minimum)
- [ ] Symmetrical or balanced composition
- [ ] No fine details lost at small size
- [ ] Matches industry conventions (if applicable)

### A/B Testing

Show colleagues 3 options:
1. Custom shape (e.g., house for real estate)
2. Standard circle with letter
3. Standard rounded square with letter

**Ask**: Which is most recognizable? Which do you prefer?

**Decision**: If custom shape isn't significantly better, use standard.

---

## When to Avoid Custom Shapes

### Use Standard Shapes If:
- Custom shape is too complex to simplify
- Industry doesn't have clear visual convention
- Testing shows standard shapes perform better
- Budget/time is limited
- Client has no strong preference

**Remember**: A simple circle with good color and letter is better than a complex custom shape that's illegible.

---

## Quick Reference: Shape Selection

**Circle** → Universal, safe default
**Rounded Square** → Modern, apps, iOS-style
**Shield** → Security, legal, protection
**House** → Real estate, home services
**Hexagon** → Tech, engineering, data
**Diamond** → Luxury, premium, jewelry
**Cross** → Medical, healthcare, first aid
**Location Pin** → Travel, local services, maps
**Lightbulb** → Innovation, consulting, ideas
**Gear** → Manufacturing, engineering, services
**Book** → Education, publishing, libraries
**Shopping Bag** → Retail, e-commerce
**Envelope** → Email, communication
**Chart** → Analytics, finance, data

---

## Official References

- **MDN SVG Shapes**: https://developer.mozilla.org/en-US/docs/Web/SVG/Tutorial/Basic_Shapes
- **Material Design Icons**: https://material.io/design/iconography/system-icons.html (simplification examples)
- **Heroicons**: https://heroicons.com/ (shape inspiration)
- **Lucide Icons**: https://lucide.dev/ (shape inspiration)

---

**Last Updated**: 2026-01-14
**Maintained by**: Jezweb (jeremy@jezweb.net)
