import { Eyebrow } from "@compozy/ui";

import { HOME_AGENTS_ROW_GRID } from "./home-zone-shape";

// Static pieces of a zone that read the same loaded or loading: the live zone and its
// placeholder render these as they are, so the frame around the figures cannot drift.

/** The agents table's column heads. */
export function HomeAgentsHeader() {
  return (
    <div
      aria-hidden="true"
      className={`grid ${HOME_AGENTS_ROW_GRID} items-center gap-3 border-b border-line-soft px-4 py-2 max-[760px]:hidden`}
    >
      <Eyebrow className="text-subtle">Agent</Eyebrow>
      <Eyebrow className="text-right text-subtle">Active</Eyebrow>
      <Eyebrow className="text-right text-subtle">Sessions</Eyebrow>
      <Eyebrow className="text-right text-subtle">Failed</Eyebrow>
      <Eyebrow className="text-subtle">Runtime</Eyebrow>
    </div>
  );
}

/** The pulse heatmap's hour axis under the grid: static labels every six hours. */
export function HomePulseAxis() {
  return (
    <>
      <span aria-hidden="true" />
      {Array.from({ length: 24 }, (_, hour) => (
        <span
          aria-hidden="true"
          className="pt-0.5 text-left font-mono text-micro text-faint"
          key={hour}
        >
          {hour % 6 === 0 ? String(hour).padStart(2, "0") : ""}
        </span>
      ))}
    </>
  );
}
