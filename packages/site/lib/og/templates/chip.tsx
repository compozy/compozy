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
  brand?: boolean;
}

export function Chip({ children, brand = false }: ChipProps) {
  return (
    <div
      style={{
        ...chipBaseStyle,
        border: `1px solid ${brand ? COLORS.brand : COLORS.border}`,
        color: brand ? COLORS.brand : COLORS.textSecondary,
      }}
    >
      {children}
    </div>
  );
}
