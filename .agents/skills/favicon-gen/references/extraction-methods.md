# Logo to Favicon Extraction Methods

Detailed techniques for extracting and simplifying logo icons into favicons.

---

## Overview: When to Extract vs Create New

### Extract from Logo ✅
- Logo has standalone icon element (symbol, mark, shape)
- Icon is recognizable without accompanying text
- Icon works at small sizes (simple enough)
- Client has brand guidelines requiring logo consistency

### Create New Icon ❌
- Logo is text-only (no graphic element)
- Logo icon is too complex to simplify
- Icon requires text to make sense
- Temporary favicon needed before logo finalized

---

## Step 1: Identify Extractable Elements

### Icon Types in Logos

**1. Standalone Symbol**
- Geometric shapes (stars, circles, triangles)
- Objects (rocket, leaf, house, gear)
- Abstract marks (swooshes, curves, unique shapes)

**Examples**:
- Twitter bird (without text)
- Apple apple (bitten apple silhouette)
- Nike swoosh (without text)

**2. Lettermark/Monogram**
- Single letter styled uniquely
- Multiple letters forming a mark
- Ligature (connected letters)

**Examples**:
- McDonald's "M" golden arches
- Facebook "f" (old logo)
- HBO interconnected letters

**3. Pictorial Icon**
- Recognizable real-world object
- Simplified illustration
- Industry-specific symbol

**Examples**:
- Starbucks mermaid
- Shell shell
- Target bullseye

**4. Geometric Pattern**
- Repeating shapes
- Grid-based designs
- Sacred geometry

**Examples**:
- Microsoft window grid
- Adidas three stripes
- Olympics rings

### What to Look For

Open your logo file (SVG preferred) and identify:

1. **Bounded element** - Shape/group that's self-contained
2. **Balanced composition** - Works without surrounding text
3. **Clear silhouette** - Recognizable from shape alone
4. **Simple geometry** - Made of basic shapes (circles, paths, polygons)

### Red Flags (Hard to Extract)

- Icon only makes sense with text
- Highly asymmetrical (hard to center)
- Too many fine details (line art, cross-hatching)
- Gradients with many stops
- Photographic elements

---

## Step 2: Extract the Icon Element

### Method 1: SVG Source File (Preferred)

**If you have the logo as SVG**:

1. **Open in code editor** (VS Code, Sublime)
   ```xml
   <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 60">
     <!-- Logo content -->
     <g id="icon-group">
       <!-- These are the icon paths -->
       <circle cx="30" cy="30" r="20" fill="#0066cc"/>
       <path d="M30,15 L35,25 L25,25 Z" fill="#ffffff"/>
     </g>
     <text x="60" y="40">Company Name</text>
   </svg>
   ```

2. **Identify icon group** (look for `<g>` with id, or grouped shapes)

3. **Copy icon paths only**
   ```xml
   <!-- Extract just these -->
   <circle cx="30" cy="30" r="20" fill="#0066cc"/>
   <path d="M30,15 L35,25 L25,25 Z" fill="#ffffff"/>
   ```

4. **Create new SVG** with 32x32 viewBox
   ```xml
   <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
     <!-- Paste icon paths, adjust coordinates -->
     <circle cx="16" cy="16" r="14" fill="#0066cc"/>
     <path d="M16,8 L19,14 L13,14 Z" fill="#ffffff"/>
   </svg>
   ```

5. **Adjust coordinates** to center in 32x32 viewBox

### Method 2: Design Tool (Figma, Illustrator, Inkscape)

**For non-SVG logos** (PNG, JPEG, AI, PDF):

1. **Open logo in design tool**

2. **Select icon element**
   - Figma: Click icon, Cmd+C (copy)
   - Illustrator: Select with Direct Selection tool
   - Inkscape: Select, Edit → Copy

3. **Create new 32x32 artboard**
   - Figma: Frame (F) → 32x32
   - Illustrator: New artboard, 32px × 32px
   - Inkscape: File → Document Properties → 32x32

4. **Paste icon** (Cmd+V)

5. **Center and scale**
   - Use alignment tools (center vertically + horizontally)
   - Scale to fill ~80% of canvas (leave padding)

6. **Export as SVG**
   - Figma: Export → SVG → Export
   - Illustrator: File → Export → Export As → SVG
   - Inkscape: File → Save As → Optimized SVG

### Method 3: Trace from Raster (Last Resort)

**If only have PNG/JPG logo**:

1. **Use image tracing** (Illustrator's Image Trace, Inkscape's Trace Bitmap)

2. **Extract icon portion** after tracing

3. **Simplify paths** (reduce anchor points)

4. **Follow Method 2 steps above**

**Warning**: Tracing quality depends on source image resolution. Prefer vector sources when possible.

---

## Step 3: Simplify for Small Sizes

Icons that work at 200px often fail at 16px. Simplification is critical.

### Simplification Rules

#### Rule 1: Minimum Feature Size
**At 16x16, one pixel = 6% of canvas**

- Features < 2px wide become invisible
- Gaps < 2px merge into solid
- Details < 3×3px are illegible

**Action**: Remove or merge features smaller than 2-3px

#### Rule 2: Reduce Shape Count
**Too many shapes = muddy appearance**

- Aim for 3-5 major shapes
- Merge adjacent shapes where possible
- Remove decorative elements

**Example**:
- Detailed rocket (10 shapes) → Simple rocket (4 shapes: body, window, fins, flame)

#### Rule 3: Increase Stroke Width
**Thin lines vanish at small sizes**

- Minimum stroke: 2px (preferably 3px)
- Consider converting strokes to fills
- Simplify path details

**Before**: 0.5px hairline → **After**: 3px bold stroke

#### Rule 4: Reduce Color Count
**Too many colors = visual noise**

- 2-3 colors maximum
- High contrast between colors (4.5:1 minimum)
- Consider monochrome (1-2 colors)

**Example**: 6-color gradient logo → 2-color favicon (primary + white)

#### Rule 5: Remove Text
**Text is illegible below 8-10px font size**

- Remove all text elements from icon
- If text is core identity, use monogram instead (see monogram-patterns.md)

#### Rule 6: Increase Negative Space
**Crowded icons are unreadable**

- Add 10-20% padding around icon edges
- Increase gaps between elements
- Simplify internal details

### Simplification Checklist

- [ ] All features ≥ 2px (minimum visible size)
- [ ] 3-5 shapes total (reduced from original)
- [ ] Strokes ≥ 2-3px width
- [ ] 2-3 colors only
- [ ] No text elements
- [ ] Adequate negative space (padding)
- [ ] Tested at 16x16 zoom (still recognizable)

---

## Step 4: Center and Scale

### Centering Techniques

**Manual Method** (design tool):
1. Select icon shapes
2. Use align tools: "Align center" + "Align middle"
3. Verify equal padding on all sides

**SVG Method** (code):
1. Calculate bounding box of icon paths
2. Apply transform to center:
   ```xml
   <g transform="translate(dx, dy)">
     <!-- Icon paths -->
   </g>
   ```
3. Where `dx` = (32 - width) / 2, `dy` = (32 - height) / 2

### Scaling Guidelines

**Target fill**: 70-85% of canvas

- Too small: Icon lost in padding, hard to see
- Too large: Touches edges, feels cramped

**Test at extremes**:
- 16x16 (minimum display)
- 512x512 (maximum display)

Should be recognizable at both scales.

### Padding Standards

| Canvas Size | Recommended Padding | Icon Size |
|-------------|---------------------|-----------|
| 32×32 | 3-5px | 22-26px |
| 180×180 (iOS) | 25-35px | 110-130px |
| 192×192 (Android) | 20-30px | 132-152px |
| 512×512 (PWA) | 50-75px | 362-412px |

---

## Step 5: Optimize Paths

### Path Optimization Techniques

**1. Reduce Anchor Points**
- Fewer points = cleaner render, smaller file
- Use tool: Illustrator's Simplify, Figma's Flatten, Inkscape's Simplify

**2. Convert Curves to Straight Lines** (where appropriate)
- Curves with tiny variation → straight line
- Reduces complexity without visual loss

**3. Merge Overlapping Paths**
- Union operation on adjacent shapes
- Reduces layer count

**4. Round Coordinates**
- Round x/y to nearest 0.5 or 1.0
- Cleaner SVG code, better rendering

**5. Remove Unnecessary Attributes**
- Strip metadata, comments
- Remove unused definitions
- Use SVGO tool for automated optimization

### SVGO (Command Line Optimizer)

```bash
# Install SVGO
npm install -g svgo

# Optimize SVG
svgo favicon.svg

# Before: 3,245 bytes, After: 892 bytes (typical savings)
```

**Configuration** (preserve viewBox, colors):
```json
{
  "plugins": [
    { "removeViewBox": false },
    { "convertColors": { "currentColor": false } }
  ]
}
```

---

## Real-World Examples

### Example 1: Extracting Twitter Bird

**Original**: Twitter logo with bird + text

**Extraction Steps**:
1. Identified bird shape (single path element in SVG)
2. Copied bird path
3. Created 32×32 viewBox
4. Centered bird, scaled to 24px height
5. Changed color from #1DA1F2 to solid blue
6. Removed fine feather details (merged into solid shape)

**Result**: Clean bird silhouette, recognizable at 16×16

**Simplifications**:
- 15 anchor points → 8 anchor points
- 3 separate shapes → 1 merged shape
- Gradient → solid color

### Example 2: Extracting Starbucks Mermaid

**Original**: Detailed mermaid illustration in circle

**Extraction Steps**:
1. Identified circular boundary + mermaid figure
2. Removed outer ring text
3. Simplified mermaid (removed hair details, crown details)
4. Increased stroke width on key elements (face, tail)
5. Merged adjacent shapes
6. Converted to 2-color (green circle + white mermaid)

**Result**: Recognizable mermaid silhouette

**Simplifications**:
- 50+ shapes → 12 shapes
- 4 colors → 2 colors
- Complex hair → simple crown + outline

### Example 3: Extracting Nike Swoosh

**Original**: Swoosh + text

**Extraction Steps**:
1. Copied swoosh path (single shape)
2. Centered in 32×32 canvas
3. Scaled to fill 75% (swoosh is horizontal, needs more width)
4. No simplification needed (already simple)

**Result**: Perfect favicon (swoosh is inherently simple)

**Simplifications**: None needed (already minimal)

---

## Testing Extracted Icons

### Visual Tests

1. **Zoom test**: View at 16×16, 32×32, 64×64, 512×512
   - Should be recognizable at all sizes
   - No pixelation or artifacts

2. **Contrast test**: View on light and dark backgrounds
   - If using transparency, test both contexts
   - Ensure adequate contrast (4.5:1 minimum)

3. **Context test**: Place in browser tab bar
   - Compare to other site favicons
   - Should stand out, not blend in

4. **Recognition test**: Show to colleague (A/B test)
   - Can they identify the brand from icon alone?
   - Does it match logo personality?

### Technical Tests

1. **File size**: Should be < 5KB for SVG
2. **Valid XML**: Validate at https://validator.w3.org/
3. **Render test**: Open SVG directly in browser (drag into tab)
4. **Export test**: Generate ICO and PNGs, verify quality

---

## When Extraction Fails

### Signs Extraction Won't Work

- Icon unrecognizable without text
- Too complex to simplify (would lose identity)
- Asymmetrical composition (can't center well)
- Requires color gradients to work
- Client brand guidelines prohibit simplification

### Alternative Approaches

**Option 1: Create Monogram** (see monogram-patterns.md)
- Use initials or abbreviation
- Place in geometric shape
- Match logo colors

**Option 2: Use Primary Brand Element**
- Extract most recognizable shape
- May not be exact logo icon, but brand-consistent

**Option 3: Consult Designer**
- Request icon variation from brand designer
- Provide specifications (32×32, 2-3 colors, simple)

---

## Extraction Workflow Summary

1. **Identify** extractable icon element in logo
2. **Extract** paths/shapes into new 32×32 SVG
3. **Simplify** reduce shapes, increase stroke width, remove details
4. **Center and scale** fill 70-85% of canvas
5. **Optimize** paths and file size
6. **Test** at 16×16 and in browser context
7. **Generate** all required formats (ICO, PNGs)

**Average Time**: 15-30 minutes for simple extraction, 45-60 minutes for complex logos

---

## Tools Reference

| Tool | Use Case | Cost |
|------|----------|------|
| **VS Code** | Edit SVG source code | Free |
| **Figma** | Visual extraction, scaling | Free (web) |
| **Illustrator** | Professional icon work | $20/mo |
| **Inkscape** | Free Illustrator alternative | Free |
| **SVGO** | Automated SVG optimization | Free |
| **ImageMagick** | Generate ICO and PNGs | Free |
| **RealFaviconGenerator** | Online all-in-one | Free |

---

## Official References

- **SVG Specification**: https://www.w3.org/TR/SVG2/
- **SVGO**: https://github.com/svg/svgo
- **Apple Icon Guidelines**: https://developer.apple.com/design/human-interface-guidelines/app-icons
- **Material Design Icons**: https://material.io/design/iconography/ (simplification examples)

---

**Last Updated**: 2026-01-14
**Maintained by**: Jezweb (jeremy@jezweb.net)
