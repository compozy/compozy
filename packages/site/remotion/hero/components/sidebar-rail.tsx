import type { CSSProperties } from "react";
import { Logo } from "@agh/ui";
import { TOKENS } from "../tokens";
import type { AgentId } from "../data";

interface SidebarRailProps {
  activeAgent: AgentId;
}

const AGENTS: { id: AgentId; letter: string }[] = [
  { id: "CODER", letter: "C" },
  { id: "REVIEWER", letter: "R" },
  { id: "OPS", letter: "O" },
  { id: "QA", letter: "Q" },
  { id: "DEPLOYER", letter: "D" },
];

const WRAP_STYLE: CSSProperties = {
  width: 60,
  flexShrink: 0,
  backgroundColor: TOKENS.canvas,
  borderRight: `1px solid ${TOKENS.divider}`,
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  padding: "14px 0",
  gap: 10,
};
const LOGO_BOX_STYLE: CSSProperties = {
  width: 32,
  height: 32,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  marginBottom: 8,
};
const DIVIDER_STYLE: CSSProperties = {
  width: 24,
  height: 1,
  backgroundColor: TOKENS.divider,
  marginBottom: 4,
};

export function SidebarRail({ activeAgent }: SidebarRailProps) {
  return (
    <div style={WRAP_STYLE}>
      <div style={LOGO_BOX_STYLE}>
        <Logo variant="symbol" decorative style={{ width: 32, height: 32, display: "block" }} />
      </div>
      <div style={DIVIDER_STYLE} />
      {AGENTS.map(a => {
        const active = a.id === activeAgent;
        const circle: CSSProperties = {
          position: "relative",
          width: 34,
          height: 34,
          borderRadius: 999,
          backgroundColor: TOKENS.surfaceElevated,
          color: active ? TOKENS.textPrimary : TOKENS.textLabel,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontFamily: TOKENS.fontMono,
          fontSize: 12,
          fontWeight: 600,
          border: `2px solid ${active ? TOKENS.accent : "transparent"}`,
          transition: "none",
        };
        const pulse: CSSProperties = {
          position: "absolute",
          right: -1,
          bottom: -1,
          width: 8,
          height: 8,
          borderRadius: 999,
          backgroundColor: active ? TOKENS.success : TOKENS.textLabel,
          border: `2px solid ${TOKENS.canvas}`,
        };
        return (
          <div key={a.id} style={circle}>
            {a.letter}
            <span style={pulse} />
          </div>
        );
      })}
    </div>
  );
}
