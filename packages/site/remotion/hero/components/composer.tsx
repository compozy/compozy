import type { CSSProperties } from "react";
import { useCurrentFrame } from "remotion";
import { TOKENS } from "../tokens";

interface ComposerProps {
  staticMode?: boolean;
}

const WRAP_STYLE: CSSProperties = {
  padding: "10px 14px 14px 14px",
  backgroundColor: TOKENS.surfacePanel,
  borderTop: `1px solid ${TOKENS.divider}`,
  flexShrink: 0,
};
const BOX_STYLE: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 10,
  padding: "7px 8px 7px 14px",
  border: `1px solid ${TOKENS.divider}`,
  backgroundColor: TOKENS.surface,
  borderRadius: TOKENS.radiusLg,
};
const INPUT_STYLE: CSSProperties = {
  flex: 1,
  fontSize: 12,
  color: TOKENS.textLabel,
  fontFamily: TOKENS.fontSans,
  display: "flex",
  alignItems: "center",
  gap: 2,
};
const SEND_STYLE: CSSProperties = {
  width: 30,
  height: 30,
  borderRadius: 999,
  backgroundColor: TOKENS.accent,
  color: "#fff",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  flexShrink: 0,
};

export function Composer({ staticMode = false }: ComposerProps) {
  const frame = useCurrentFrame();
  const caretOn = !staticMode && frame % 30 < 15;

  const caret: CSSProperties = {
    display: "inline-block",
    width: 1,
    height: 12,
    backgroundColor: caretOn ? TOKENS.accent : "transparent",
    marginLeft: 2,
  };

  return (
    <div style={WRAP_STYLE}>
      <div style={BOX_STYLE}>
        <div style={INPUT_STYLE}>
          <span>Send a message…</span>
          <span style={caret} />
        </div>
        <span style={SEND_STYLE} aria-label="send">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M4 12h14M14 6l6 6-6 6"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </span>
      </div>
    </div>
  );
}
