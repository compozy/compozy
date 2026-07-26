import { ImageResponse } from "next/og";
import type { CSSProperties } from "react";
import { loadOGFonts } from "../fonts";
import { SymbolGlyph } from "../logo";
import { COLORS, FONTS, formatBlogDate, SIZE, truncate } from "../tokens";

export interface RenderBlogOGInput {
  title: string;
  description?: string;
  slug: string;
  date?: string;
  author?: string;
}

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

const metaStyle: CSSProperties = {
  display: "flex",
  flexDirection: "row",
  alignItems: "center",
  gap: "20px",
  fontFamily: FONTS.mono,
  fontSize: "18px",
  letterSpacing: "0.06em",
  fontWeight: 500,
};

export async function renderBlogOG({
  title,
  description,
  slug,
  date,
  author,
}: RenderBlogOGInput): Promise<ImageResponse> {
  const fonts = await loadOGFonts();
  const safeTitle = truncate(title, 140);
  const safeDescription = truncate(description, 160);
  const formattedDate = formatBlogDate(date);
  const trimmedSlug = slug.replace(/^posts\//, "");

  return new ImageResponse(
    <div style={canvasStyle}>
      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <div
          style={{
            display: "flex",
            flexDirection: "row",
            alignItems: "center",
            gap: "16px",
          }}
        >
          <SymbolGlyph size={48} radius={12} />
          <div
            style={{
              display: "flex",
              fontFamily: FONTS.mono,
              fontSize: "32px",
              color: COLORS.textPrimary,
              fontWeight: 500,
              letterSpacing: "-0.01em",
            }}
          >
            agh
          </div>
        </div>
        <div style={metaStyle}>
          <span style={{ color: COLORS.accent, textTransform: "uppercase" }}>AGH BLOG</span>
          {formattedDate ? (
            <>
              <span style={{ width: "48px", height: "1px", background: COLORS.border }} />
              <span style={{ color: COLORS.textSecondary }}>{formattedDate}</span>
            </>
          ) : null}
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "stretch",
          gap: "32px",
          maxWidth: "1056px",
        }}
      >
        <div
          style={{
            display: "flex",
            width: "3px",
            background: COLORS.accent,
            borderRadius: "2px",
          }}
        />
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "28px",
            flexGrow: 1,
          }}
        >
          <div
            style={{
              fontFamily: FONTS.display,
              fontSize: "76px",
              lineHeight: 1.0,
              letterSpacing: "-0.02em",
              color: COLORS.textPrimary,
              fontWeight: 400,
              maxWidth: "940px",
            }}
          >
            {safeTitle}
          </div>
          {safeDescription ? (
            <div
              style={{
                fontFamily: FONTS.inter,
                fontSize: "22px",
                lineHeight: 1.5,
                color: COLORS.textSecondary,
                maxWidth: "880px",
                fontWeight: 400,
              }}
            >
              {safeDescription}
            </div>
          ) : null}
        </div>
      </div>

      <div
        style={{
          display: "flex",
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
          borderTop: `1px solid ${COLORS.border}`,
          paddingTop: "24px",
        }}
      >
        <div
          style={{
            display: "flex",
            fontFamily: FONTS.mono,
            fontSize: "18px",
            color: COLORS.textSecondary,
            fontWeight: 500,
            letterSpacing: "0.04em",
          }}
        >
          {`agh.network/blog/${trimmedSlug}`}
        </div>
        {author ? (
          <div
            style={{
              display: "flex",
              fontFamily: FONTS.mono,
              fontSize: "16px",
              color: COLORS.textTertiary,
              fontWeight: 500,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
            }}
          >
            {`BY ${author}`}
          </div>
        ) : null}
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
