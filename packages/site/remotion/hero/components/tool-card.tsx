import type { CSSProperties } from "react";
import { interpolate, useCurrentFrame } from "remotion";
import { TOKENS } from "../tokens";

import type { ToolIcon as ToolIconName } from "../data";

interface ToolCardProps {
  icon: ToolIconName;
  label: string;
  summary: string;
  start: number;
  doneAt: number;
  staticMode?: boolean;
}

const TOOL_ICON_PROPS = { width: 13, height: 13, viewBox: "0 0 24 24", fill: "none" } as const;
const TOOL_LABEL_STYLE: CSSProperties = {
  color: TOKENS.textPrimary,
  fontFamily: TOKENS.fontSans,
  fontSize: 12,
  fontWeight: 500,
  flexShrink: 0,
};
const TOOL_SUMMARY_STYLE: CSSProperties = {
  color: TOKENS.textSecondary,
  fontFamily: TOKENS.fontMono,
  fontSize: 11,
  letterSpacing: TOKENS.trackingMono,
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
  minWidth: 0,
  flex: 1,
};

function ToolIcon({ name }: { name: ToolIconName }) {
  if (name === "read") {
    return (
      <svg {...TOOL_ICON_PROPS} aria-hidden="true">
        <path
          d="M6 3h9l4 4v14a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinejoin="round"
        />
        <path d="M15 3v4h4" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
        <path d="M8 13h8M8 17h6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
    );
  }
  if (name === "diff") {
    return (
      <svg {...TOOL_ICON_PROPS} aria-hidden="true">
        <path
          d="M7 3v13a3 3 0 0 0 3 3h7M7 3l-3 3M7 3l3 3"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <path d="M17 21v-4M17 13v4" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
    );
  }
  if (name === "test") {
    return (
      <svg {...TOOL_ICON_PROPS} aria-hidden="true">
        <path
          d="M9 3h6M10 3v7l-5 9a2 2 0 0 0 1.7 3h10.6a2 2 0 0 0 1.7-3l-5-9V3"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  if (name === "net") {
    return (
      <svg {...TOOL_ICON_PROPS} aria-hidden="true">
        <circle cx="12" cy="12" r="8.5" stroke="currentColor" strokeWidth="1.7" />
        <path
          d="M3.5 12h17M12 3.5c2.5 2.5 3.8 5.6 3.8 8.5s-1.3 6-3.8 8.5c-2.5-2.5-3.8-5.6-3.8-8.5s1.3-6 3.8-8.5z"
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
        />
      </svg>
    );
  }
  return (
    <svg {...TOOL_ICON_PROPS} aria-hidden="true">
      <path
        d="M5 12l7-8 7 8-7 8-7-8z"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinejoin="round"
      />
      <path d="M12 8v8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

export function ToolCard({
  icon,
  label,
  summary,
  start,
  doneAt,
  staticMode = false,
}: ToolCardProps) {
  const frame = useCurrentFrame();

  const opacity = staticMode
    ? 1
    : interpolate(frame, [start - 2, start + 8], [0, 1], {
        extrapolateLeft: "clamp",
        extrapolateRight: "clamp",
      });
  const translateY = staticMode
    ? 0
    : interpolate(frame, [start - 2, start + 10], [6, 0], {
        extrapolateLeft: "clamp",
        extrapolateRight: "clamp",
      });

  const done = staticMode || frame >= doneAt;

  const wrap: CSSProperties = {
    display: "flex",
    alignItems: "center",
    gap: 10,
    padding: "6px 10px",
    margin: "4px 0 2px 10px",
    border: `1px solid ${TOKENS.divider}`,
    backgroundColor: TOKENS.surface,
    borderRadius: TOKENS.radiusMd,
    opacity,
    transform: `translateY(${translateY}px)`,
  };
  const iconBox: CSSProperties = {
    color: done ? TOKENS.success : TOKENS.accentStrong,
    display: "inline-flex",
    alignItems: "center",
    flexShrink: 0,
  };
  const badge: CSSProperties = {
    marginLeft: "auto",
    fontFamily: TOKENS.fontMono,
    fontSize: 9,
    fontWeight: 600,
    textTransform: "uppercase",
    letterSpacing: TOKENS.trackingBadge,
    padding: "3px 7px",
    borderRadius: 999,
    backgroundColor: done ? TOKENS.successTint : TOKENS.accentTint,
    color: done ? TOKENS.success : TOKENS.accentStrong,
    whiteSpace: "nowrap",
    flexShrink: 0,
  };

  return (
    <div style={wrap}>
      <span style={iconBox}>
        <ToolIcon name={icon} />
      </span>
      <span style={TOOL_LABEL_STYLE}>{label}</span>
      <span style={TOOL_SUMMARY_STYLE}>{summary}</span>
      <span style={badge}>{done ? "DONE" : "RUNNING"}</span>
    </div>
  );
}
