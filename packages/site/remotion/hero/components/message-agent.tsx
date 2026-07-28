import type { CSSProperties, ReactNode } from "react";
import { interpolate, useCurrentFrame } from "remotion";
import { TOKENS } from "../tokens";
import type { AgentId } from "../data";
import { useTypewriter } from "./hooks/use-typewriter";

interface MessageAgentProps {
  agent: AgentId;
  timestamp: string;
  labelStart: number;
  reply?: {
    text: string;
    start: number;
    duration: number;
  };
  children?: ReactNode;
  staticMode?: boolean;
}

const LABEL_ROW_STYLE: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 8,
  marginLeft: 6,
};
const DOT_STYLE: CSSProperties = {
  width: 6,
  height: 6,
  borderRadius: 999,
  backgroundColor: TOKENS.success,
  boxShadow: `0 0 0 3px ${TOKENS.success}1a`,
};
const NAME_STYLE: CSSProperties = {
  fontFamily: TOKENS.fontMono,
  fontSize: 10,
  fontWeight: 600,
  letterSpacing: TOKENS.trackingBadge,
  textTransform: "uppercase",
  color: TOKENS.textLabel,
};
const TIMESTAMP_STYLE: CSSProperties = {
  fontFamily: TOKENS.fontSans,
  fontSize: 10,
  color: TOKENS.textLabel,
  fontVariantNumeric: "tabular-nums",
};
const BODY_STYLE: CSSProperties = {
  marginTop: 4,
  marginLeft: 6,
  fontSize: 13,
  lineHeight: 1.5,
  color: TOKENS.textSecondary,
  fontFamily: TOKENS.fontSans,
};

export function MessageAgent({
  agent,
  timestamp,
  labelStart,
  reply,
  children,
  staticMode = false,
}: MessageAgentProps) {
  const frame = useCurrentFrame();
  const typed = useTypewriter(reply?.text ?? "", reply?.start ?? labelStart, reply?.duration ?? 1);

  if (!staticMode && frame < labelStart - 4) {
    return null;
  }

  const opacity = staticMode
    ? 1
    : interpolate(frame, [labelStart - 4, labelStart + 6], [0, 1], {
        extrapolateLeft: "clamp",
        extrapolateRight: "clamp",
      });
  const translateY = staticMode
    ? 0
    : interpolate(frame, [labelStart - 4, labelStart + 10], [14, 0], {
        extrapolateLeft: "clamp",
        extrapolateRight: "clamp",
      });
  const visibleText = staticMode ? (reply?.text ?? "") : typed.visible;
  const done = staticMode ? true : typed.done;

  const wrap: CSSProperties = {
    padding: "6px 18px 4px 18px",
    opacity,
    transform: `translateY(${translateY}px)`,
    flexShrink: 0,
  };
  const caret = reply && !done && !staticMode && frame >= reply.start && frame % 14 < 7 ? "▍" : "";

  return (
    <div style={wrap}>
      <div style={LABEL_ROW_STYLE}>
        <span style={DOT_STYLE} />
        <span style={NAME_STYLE}>{agent}</span>
        <span style={TIMESTAMP_STYLE}>{timestamp}</span>
      </div>
      {children}
      {reply && (visibleText.length > 0 || done) && (
        <div style={BODY_STYLE}>
          {visibleText}
          <span style={{ color: TOKENS.accent }}>{caret}</span>
        </div>
      )}
    </div>
  );
}
