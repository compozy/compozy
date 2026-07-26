import type { CSSProperties } from "react";

import { COLORS, FONTS } from "../tokens";

const chipBaseStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  padding: "8px 14px",
  borderRadius: "5px",
  fontFamily: FONTS.mono,
  fontSize: "16px",
  letterSpacing: "0.06em",
  textTransform: "uppercase",
  fontWeight: 500,
};

interface ChipProps {
  children: string;
  accent?: boolean;
}

export function Chip({ children, accent = false }: ChipProps) {
  return (
    <div
      style={{
        ...chipBaseStyle,
        border: `1px solid ${accent ? COLORS.accent : COLORS.border}`,
        color: accent ? COLORS.accent : COLORS.textSecondary,
      }}
    >
      {children}
    </div>
  );
}
