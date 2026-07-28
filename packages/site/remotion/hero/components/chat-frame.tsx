import type { CSSProperties, ReactNode } from "react";
import { TOKENS } from "../tokens";

interface ChatFrameProps {
  opacity: number;
  children: ReactNode;
}

export function ChatFrame({ opacity, children }: ChatFrameProps) {
  const style: CSSProperties = {
    width: "100%",
    height: "100%",
    display: "flex",
    flexDirection: "column",
    overflow: "hidden",
    backgroundColor: TOKENS.surfacePanel,
    border: `1px solid ${TOKENS.divider}`,
    borderRadius: TOKENS.radiusXl,
    boxShadow: "0 30px 60px -30px rgba(0,0,0,0.65), 0 0 0 1px rgba(255,255,255,0.02) inset",
    opacity,
    fontFamily: TOKENS.fontSans,
    color: TOKENS.textPrimary,
  };
  return <div style={style}>{children}</div>;
}
