// Source: https://remocn.dev — copied from the registry; MIT.
"use client";

import { type RemocnTheme, useRemocnTheme } from "@/lib/remocn-ui";

export type CursorVariant = "arrow" | "pointer";

export interface CursorStyle {
  x: number;
  y: number;
  scale: number;
  rippleOpacity: number;
  rippleScale: number;
  pressScale?: number;
}

export interface CursorProps {
  style?: CursorStyle;
  variant?: CursorVariant;
  size?: number;
  theme?: Partial<RemocnTheme>;
  rippleColor?: string;
  className?: string;
}

const REST: CursorStyle = {
  x: 0,
  y: 0,
  scale: 1,
  rippleOpacity: 0,
  rippleScale: 0,
};

const ARROW_PATH = "M1 1 L1 18.5 L5.6 14.4 L8.7 21.6 L11.6 20.4 L8.5 13.2 L14.5 13.2 Z";
const POINTER_PATH =
  "M8 2.2 C8 1.3 8.7 0.7 9.5 0.7 C10.3 0.7 11 1.3 11 2.2 L11 9 " +
  "C11 9 11.3 8.4 12.2 8.4 C13 8.4 13.4 9 13.4 9.6 " +
  "C13.4 9.6 13.9 9.1 14.7 9.1 C15.5 9.1 15.9 9.7 15.9 10.3 " +
  "C15.9 10.3 16.4 9.9 17.1 9.9 C17.9 9.9 18.3 10.5 18.3 11.2 " +
  "L18.3 16.8 C18.3 19.8 16.3 22.3 12.8 22.3 L11.4 22.3 " +
  "C9 22.3 7.9 21.2 6.6 19.2 L4 15.2 C3.5 14.4 3.7 13.4 4.5 12.9 " +
  "C5.1 12.5 5.9 12.6 6.4 13.2 L8 15 Z";

export function Cursor({
  style,
  variant = "arrow",
  size = 28,
  theme: themeOverride,
  rippleColor,
  className,
}: CursorProps) {
  const theme = useRemocnTheme(themeOverride, "light");
  const v = style ?? REST;

  const press = v.pressScale ?? 1;
  const pressedScale = v.scale * (0.9 + 0.1 * press);

  const ring = rippleColor ?? theme.primary;
  const fill = theme.foreground;
  const stroke = theme.background;
  const rippleSize = size * 1.8;

  return (
    <div
      className={className}
      style={{
        position: "absolute",
        left: 0,
        top: 0,
        transform: `translate(${v.x}px, ${v.y}px)`,
        pointerEvents: "none",
      }}
    >
      {}
      <div
        style={{
          position: "absolute",
          left: -rippleSize / 2,
          top: -rippleSize / 2,
          width: rippleSize,
          height: rippleSize,
          borderRadius: "50%",
          border: `2px solid ${ring}`,
          opacity: v.rippleOpacity,
          transform: `scale(${v.rippleScale})`,
          transformOrigin: "center",
        }}
      />
      {}
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          transform: `scale(${pressedScale})`,
          transformOrigin: "0 0",
          filter: "drop-shadow(0 1px 2px rgba(0,0,0,0.3))",
          overflow: "visible",
        }}
      >
        <path
          d={variant === "pointer" ? POINTER_PATH : ARROW_PATH}
          fill={fill}
          stroke={stroke}
          strokeWidth={1.4}
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}
