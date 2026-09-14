import type { CSSProperties } from "react";
import { Sequence } from "remotion";

import { FONT_SANS } from "@/brand/fonts";
import { color, weight } from "@/brand/tokens";
import { MaskRevealUp } from "@/components/remocn/mask-reveal-up";
import { FEATURE_LINE_HEIGHT } from "@/compositions/release-announce/layout";

/** MaskRevealUp resolves its type through this custom property; point it at the loaded Geist. */
const FONT_VAR = { "--font-geist-sans": FONT_SANS } as CSSProperties;

interface FeatureLinesProps {
  features: readonly string[];
  fontSize: number;
  /** Composition frame the reveal starts on. */
  from: number;
  /** Band the lines are centered in — MaskRevealUp fills its positioned parent. */
  top: number;
  height: number;
}

export function FeatureLines({ features, fontSize, from, top, height }: FeatureLinesProps) {
  return (
    <div style={{ position: "absolute", left: 0, right: 0, top, height, ...FONT_VAR }}>
      <Sequence from={from} layout="none">
        <MaskRevealUp
          text={features.join("\n")}
          fontSize={fontSize}
          fontWeight={weight.medium}
          lineHeight={FEATURE_LINE_HEIGHT}
          color={color.fg}
          distance={Math.round(fontSize * 0.34)}
          enterStagger={10}
          exit={false}
        />
      </Sequence>
    </div>
  );
}
