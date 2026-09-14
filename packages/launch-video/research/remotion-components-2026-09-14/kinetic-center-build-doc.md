# Kinetic Center Build
URL: https://remocn.dev/docs/typography/kinetic-center-build

Words enter from the right and push the line until the full phrase locks centered

- Install: `npx shadcn@latest add @remocn/kinetic-center-build`
- Vibe: premium
- Natural length: 60f @ 30fps

## Installation

```bash
npx shadcn@latest add @remocn/kinetic-center-build
```

## Usage

```tsx
// src/Root.tsx
import { Composition } from "remotion";
import { KineticCenterBuild } from "@/components/remocn/kinetic-center-build";

const KineticCenterBuildScene = () => (
  <KineticCenterBuild text="Words push left." entryOffset={88} />
);

export const RemotionRoot = () => (
  <Composition
    id="KineticCenterBuild"
    component={KineticCenterBuildScene}
    durationInFrames={60}
    fps={30}
    width={1280}
    height={720}
  />
);
```

Each word lands centered, then the next word slides in from the right and pushes the line left so the growing phrase stays centered at every step — reads as one kinetic line build, best for short three-word phrases.

This is a layout-aware effect — it measures word widths to keep the line centered, so it works best with short headings.

### With Backdrop

The component renders transparent — supply the background via `Backdrop`. It ships single-theme; edit the copied file to re-theme colors.

Pair with `Backdrop` to place the text inside a full-frame fill with a rounded, shadowed content frame:

```tsx
import { Composition } from "remotion";
import { KineticCenterBuild } from "@/components/remocn/kinetic-center-build";
import { Backdrop } from "@/components/remocn/backdrop";

const KineticCenterBuildScene = () => (
  <Backdrop fill={{ type: "color", value: "#ffffff" }}>
    <KineticCenterBuild text="Words push left." entryOffset={88} />
  </Backdrop>
);

export const RemotionRoot = () => (
  <Composition
    id="KineticCenterBuild"
    component={KineticCenterBuildScene}
    durationInFrames={60}
    fps={30}
    width={1280}
    height={720}
  />
);
```

## Props

| Prop | Type | Default | Description |
|---|---|---|---|
| `text` (required) | `string` | — | The phrase to build, word by word. Best with short three-word phrases. |
| `entryOffset` | `number` | `88` | How far in pixels each incoming word starts to the right before pushing the line. |
| `fontSize` | `number` | `72` | Font size in pixels |
| `fontWeight` | `number` | `600` | CSS font-weight |
| `color` | `string` | `"#171717"` | Text color (any valid CSS color) |
| `className` | `string` | — | Optional className passed to the phrase container |

## Use when
- A short headline should build word by word with satisfying kinetic momentum, locking centered at the end.
- The reveal itself is the hero moment — the growing centered line carries visual weight through motion.
- Premium product or brand video needs a high-energy entrance for a 3–6 word phrase.

## Don't use when
- The text is long (7+ words) — the centering re-balance on each word becomes distracting; use `line-by-line-slide` for multi-word or multi-line phrases.
- The entrance should feel calm and neutral rather than kinetic — use `micro-scale-fade` or `mask-reveal-up` for composed, quiet reveals.
- Words should fade in sequentially with a vertical drift rather than push from the right — use `per-word-crossfade` for that calm keynote rhythm.
