import { createElement, type SVGProps } from "react";

import { DockIcon, type DockIconId } from "./os-dock-icons";

type GlyphProps = SVGProps<SVGSVGElement>;

function glyph(name: DockIconId) {
  return (props: GlyphProps) => createElement(DockIcon, { ...props, name });
}

export const DockIcons = {
  sessions: glyph("sessions"),
  dashboard: glyph("dashboard"),
  agents: glyph("agents"),
  network: glyph("network"),
  tasks: glyph("tasks"),
  loops: glyph("loops"),
  jobs: glyph("jobs"),
  triggers: glyph("triggers"),
  marketplace: glyph("marketplace"),
  bridges: glyph("bridges"),
  knowledge: glyph("knowledge"),
  sandbox: glyph("sandbox"),
  vault: glyph("vault"),
} as const;

export type { DockIconId };
