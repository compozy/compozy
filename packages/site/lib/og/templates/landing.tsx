import { ImageResponse } from "next/og";
import type { CSSProperties } from "react";
import { siteConfig } from "@/lib/site-config";
import { loadOGFonts } from "../fonts";
import { LogoLockup, SymbolGlyph } from "../logo";
import { COLORS, FONTS, SIZE } from "../tokens";

const HEADLINE = "An open workplace for AI agents.";
const EYEBROW = "ARTIFICIAL GENERAL HIVEMIND";

function deriveSubhead(description: string): string {
  const firstStop = description.indexOf(".");
  if (firstStop < 0) return description;
  return description.slice(firstStop + 1).trim();
}

const FOOTER_RAIL = [
  { label: "AGH NETWORK / V0", color: COLORS.accent, lowercase: false },
  { label: "agh.network", color: COLORS.textSecondary, lowercase: true },
  { label: "LOCAL-FIRST RUNTIME", color: COLORS.textSecondary, lowercase: false },
] as const;

const canvasStyle: CSSProperties = {
  width: "100%",
  height: "100%",
  display: "flex",
  flexDirection: "column",
  justifyContent: "space-between",
  background: COLORS.canvas,
  color: COLORS.textPrimary,
  fontFamily: FONTS.inter,
  padding: "72px",
};

const eyebrowStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: "20px",
  fontFamily: FONTS.mono,
  fontSize: "18px",
  letterSpacing: "0.06em",
  color: COLORS.textSecondary,
  fontWeight: 500,
};

const symbolTileStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  width: "120px",
  height: "120px",
  background: COLORS.surface,
  border: `1px solid ${COLORS.border}`,
  borderRadius: "28px",
};

const footerRailItemStyle: CSSProperties = {
  display: "flex",
  alignItems: "center",
  fontFamily: FONTS.mono,
  fontSize: "16px",
  letterSpacing: "0.06em",
  fontWeight: 500,
};

export async function renderLandingOG(): Promise<ImageResponse> {
  const fonts = await loadOGFonts();
  const subhead = deriveSubhead(siteConfig.description);

  return new ImageResponse(
    <div style={canvasStyle}>
      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: "28px" }}>
          <LogoLockup height={64} letteringFill={COLORS.textPrimary} />
          <div style={eyebrowStyle}>
            <span style={{ width: "96px", height: "1px", background: COLORS.border }} />
            <span>{EYEBROW}</span>
          </div>
        </div>
        <div style={symbolTileStyle}>
          <SymbolGlyph size={72} radius={20} />
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "32px",
          maxWidth: "1000px",
        }}
      >
        <div
          style={{
            fontFamily: FONTS.display,
            fontSize: "92px",
            lineHeight: 0.96,
            letterSpacing: "-0.025em",
            color: COLORS.textPrimary,
            fontWeight: 400,
          }}
        >
          {HEADLINE}
        </div>
        <div
          style={{
            fontFamily: FONTS.inter,
            fontSize: "26px",
            lineHeight: 1.45,
            color: COLORS.textSecondary,
            maxWidth: "880px",
            fontWeight: 400,
          }}
        >
          {subhead}
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "stretch",
          borderTop: `1px solid ${COLORS.border}`,
          paddingTop: "28px",
        }}
      >
        {FOOTER_RAIL.map((entry, idx) => (
          <div
            key={entry.label}
            style={{
              ...footerRailItemStyle,
              paddingLeft: idx === 0 ? "0" : "32px",
              paddingRight: idx === FOOTER_RAIL.length - 1 ? "0" : "32px",
              borderLeft: idx === 0 ? "none" : `1px solid ${COLORS.border}`,
              color: entry.color,
              textTransform: entry.lowercase ? "lowercase" : "uppercase",
            }}
          >
            {entry.label}
          </div>
        ))}
      </div>
    </div>,
    {
      ...SIZE,
      fonts: fonts.map(font => ({
        name: font.name,
        data: font.data,
        weight: font.weight,
        style: font.style,
      })),
    }
  );
}
