"use client";

import { Easing, interpolate, useCurrentFrame, useVideoConfig } from "remotion";

export interface MaskRevealUpProps {
  text: string;
  distance?: number;
  fontSize?: number;
  color?: string;
  fontWeight?: number;
  /** Added locally: the stock 1.1 reads as one paragraph when the lines are independent items. */
  lineHeight?: number;
  /**
   * Added locally. The stock component always schedules a blur-out exit against the end of its
   * duration, which a bumper cannot have — it has to hold to the last frame so an editor can trim
   * into it. Budgeting a longer `<Sequence>` does not help: Remotion clamps the duration reported
   * inside a sequence to whatever is left of the composition.
   */
  exit?: boolean;
  /**
   * Added locally. At the stock 3 frames a short stack lands as one block; independent items want
   * enough delay that the eye reads the first before the second arrives.
   */
  enterStagger?: number;
  speed?: number;
  className?: string;
}

export function MaskRevealUp({
  text,
  distance = 30,
  fontSize = 72,
  color = "#171717",
  fontWeight = 600,
  lineHeight = 1.1,
  exit = true,
  enterStagger = 3,
  speed = 1,
  className,
}: MaskRevealUpProps) {
  const frame = useCurrentFrame() * speed;
  const { durationInFrames } = useVideoConfig();

  const lines = text.split("\n");

  const enterDur = 23;
  const enterTravel = 11;
  const exitDur = 16;
  const exitTravelFrom = 8;
  const exitStagger = 2;

  const enterEasing = Easing.bezier(0.22, 1, 0.36, 1);
  const exitEasing = Easing.bezier(0.64, 0, 0.78, 0);

  const enterEnd = enterDur + (lines.length - 1) * enterStagger;
  const exitStart = Math.max(
    enterEnd,
    durationInFrames - exitDur - (lines.length - 1) * exitStagger
  );

  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "transparent",
      }}
    >
      <span
        className={className}
        style={{
          fontSize,
          fontWeight,
          color,
          letterSpacing: "-0.03em",
          lineHeight,
          textAlign: "center",
          fontFamily: "var(--font-geist-sans), -apple-system, BlinkMacSystemFont, sans-serif",
        }}
      >
        {lines.map((line, i) => {
          const enterLocal = frame - i * enterStagger;
          // A negative local frame clamps every exit track to its start value, i.e. no exit at all.
          const exitLocal = exit ? frame - exitStart - i * exitStagger : -1;

          const enterP = interpolate(enterLocal, [0, enterDur], [0, 1], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: enterEasing,
          });

          const exitP = interpolate(exitLocal, [0, exitDur], [0, 1], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: exitEasing,
          });

          const opacity = enterP * (1 - exitP);

          const yEnter = interpolate(enterLocal, [0, enterTravel], [distance, 0], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: enterEasing,
          });

          const yExit = interpolate(exitLocal, [exitTravelFrom, exitDur], [0, -22], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: exitEasing,
          });

          const y = yEnter + yExit;

          const blurEnter = interpolate(enterLocal, [0, enterDur], [6, 0], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: enterEasing,
          });

          const blurExit = interpolate(exitLocal, [0, exitDur], [0, 6], {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: exitEasing,
          });

          const blur = blurEnter + blurExit;

          return (
            <span
              key={i}
              style={{
                display: "block",
                transformOrigin: "50% 50%",
                opacity,
                translate: `0 ${y}px`,
                filter: `blur(${blur}px)`,
              }}
            >
              {line}
            </span>
          );
        })}
      </span>
    </div>
  );
}
