export const DESIGN_MD_BASE = "https://github.com/compozy/agh/blob/main/DESIGN.md";

export interface ShowcaseSection {
  id: string;
  label: string;
  anchor: string;
}

export const SECTIONS: ShowcaseSection[] = [
  { id: "foundations", label: "Foundations: Tokens", anchor: "#2-color-palette--roles" },
  { id: "typography", label: "Foundations: Typography", anchor: "#3-typography-rules" },
  { id: "buttons", label: "Buttons & Pills", anchor: "#buttons" },
  { id: "inputs", label: "Inputs & Search", anchor: "#inputs" },
  {
    id: "status",
    label: "Status, Metric, MonoBadge, KindChip",
    anchor: "#status-indicators",
  },
  { id: "feedback", label: "Feedback", anchor: "#empty-state" },
  { id: "overlays", label: "Dialog, Sheet, Popover, Tooltip", anchor: "#4-component-stylings" },
  { id: "code-chat", label: "Code & Chat", anchor: "#chat-components" },
  { id: "layout", label: "Sidebar & SplitPane", anchor: "#sidebar-operator-ui" },
  { id: "listing-row", label: "ListingRow", anchor: "#listing-row" },
];

/** Resolve a showcase section by stable identity so ordering remains presentation-only. */
export function sectionById(id: string): ShowcaseSection {
  const section = SECTIONS.find(entry => entry.id === id);
  if (!section) {
    throw new Error(`Unknown showcase section id: ${id}`);
  }
  return section;
}
