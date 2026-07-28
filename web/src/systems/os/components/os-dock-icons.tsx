/**
 * Dock glyph set copied from OpenDesign `os-v2.js` ICONS — kept as presentational
 * SVG markup so Visual Contract dock anatomy matches the prototype strokes, not
 * Lucide substitutes.
 */
import type { SVGProps } from "react";

type GlyphProps = SVGProps<SVGSVGElement>;

function DockGlyph({ children, className, ...props }: GlyphProps & { children: React.ReactNode }) {
  return (
    <svg
      viewBox="0 0 20 20"
      aria-hidden="true"
      className={className ?? "size-dock-icon"}
      fill="none"
      {...props}
    >
      {children}
    </svg>
  );
}

export const DockIcons = {
  sessions: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="M4 4.5h12a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5H9l-3.4 2.6a.5.5 0 0 1-.8-.4v-2.2H4A1.5 1.5 0 0 1 2.5 13V6A1.5 1.5 0 0 1 4 4.5Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  dashboard: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <rect x="3" y="3" width="6" height="6" rx="1.6" stroke="currentColor" strokeWidth="1.5" />
      <rect x="11" y="3" width="6" height="6" rx="1.6" stroke="currentColor" strokeWidth="1.5" />
      <rect x="3" y="11" width="6" height="6" rx="1.6" stroke="currentColor" strokeWidth="1.5" />
      <rect x="11" y="11" width="6" height="6" rx="1.6" stroke="currentColor" strokeWidth="1.5" />
    </DockGlyph>
  ),
  agents: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <rect x="4" y="6" width="12" height="9" rx="2.4" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M10 6V3.2M7.5 10.4h.01M12.5 10.4h.01"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
      />
      <path d="M7 13h6" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
    </DockGlyph>
  ),
  network: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <circle cx="10" cy="10" r="6.6" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M3.4 10h13.2M10 3.4c-3.6 3.8-3.6 9.4 0 13.2 3.6-3.8 3.6-9.4 0-13.2Z"
        stroke="currentColor"
        strokeWidth="1.3"
      />
    </DockGlyph>
  ),
  tasks: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="m3.5 6 1.6 1.6L8 4.7M3.5 13l1.6 1.6L8 11.7M10.5 6.5H17M10.5 13.5H17"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  loops: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="M13.5 4.5H8a4 4 0 0 0-4 4v.5M6.5 15.5H12a4 4 0 0 0 4-4V11M11 2l2.5 2.5L11 7M9 18l-2.5-2.5L9 13"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  jobs: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <circle cx="10" cy="10" r="6.8" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M10 6.2V10l3 1.8"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  triggers: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="M11 2.5 4.5 11h4l-.9 6.5L14.5 9h-4z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  marketplace: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="M4 7.5 5 4h10l1 3.5M4 7.5h12M4 7.5V15a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7.5M8 10.5h4"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  bridges: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <circle cx="4.5" cy="15.5" r="2" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="15.5" cy="4.5" r="2" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="15.5" cy="15.5" r="2" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M6.5 15.5h7M15.5 6.5v7M6 14 14 6"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    </DockGlyph>
  ),
  knowledge: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <path
        d="M4 4.5A1.5 1.5 0 0 1 5.5 3H16v13H5.5A1.5 1.5 0 0 0 4 17.5zM4 4.5v13M16 13H5.5A1.5 1.5 0 0 0 4 14.5"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </DockGlyph>
  ),
  sandbox: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <rect
        x="2.5"
        y="11"
        width="6.5"
        height="6.5"
        rx="1"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <rect x="11" y="11" width="6.5" height="6.5" rx="1" stroke="currentColor" strokeWidth="1.5" />
      <rect
        x="6.75"
        y="2.5"
        width="6.5"
        height="6.5"
        rx="1"
        stroke="currentColor"
        strokeWidth="1.5"
      />
    </DockGlyph>
  ),
  vault: (props: GlyphProps) => (
    <DockGlyph {...props}>
      <circle cx="7.5" cy="8" r="3.8" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="m10.4 10.6 5.6 5.6M13.5 13.5l1.8-1.8M15.5 15.5l1.6-1.6"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </DockGlyph>
  ),
} as const;

export type DockIconId = keyof typeof DockIcons;
