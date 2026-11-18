import { card } from "../cls";
import { Section } from "../parts/section";

const groups: [string, string[]][] = [
  [
    "Before you open a file",
    [
      "Start from a kit template, not a blank canvas.",
      "Pick one surface: ink or paper. Volt is an accent, never the ground.",
      "Pick one visual from the system, or none: Atmosphere for mood, Live for product.",
      "Know the one thing the piece says.",
    ],
  ],
  [
    "Composition",
    [
      "Margins follow the grid; nothing touches the edge except the visual.",
      "One visual, one headline, one line of mono metadata.",
      "The logo sits in a corner with clear space, at the kit size.",
      "At most one gradient word in the headline. Text never sits on the bright part of a visual without a scrim.",
    ],
  ],
  [
    "Text",
    [
      "Sentence case. No emoji, no exclamation marks.",
      "No banned words: wrapper, seamless, AI-powered, simple without a metric.",
      "“Always-on”, never “Stays on”. “Compozy”, CLI as `compozy`.",
      "Numbers are dated. Design partners are never customers. No prices.",
    ],
  ],
  [
    "Before you publish",
    [
      "Contrast checked: body ≥ 4.5:1, large ≥ 3:1.",
      "Safe areas checked for the network’s UI.",
      "Alt text written: what the image shows, not “image of”.",
      "Generated images carry a provenance note and never stand for real people.",
    ],
  ],
];

export function Checklist() {
  return (
    <Section
      id="checklist"
      n="03"
      file="checklist.md"
      title={
        <>
          Before anything <em>ships</em>
        </>
      }
      status="approved"
      intro={
        <p>Sixteen checks, four moments. If a piece fails one, fix the piece, not the checklist.</p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {groups.map(([g, items], gi) => (
          <div key={g} className={card}>
            <p className="m-0 font-mono text-[12px] text-(--b-accent)">0{gi + 1}</p>
            <p className="m-0 mt-3 font-display text-[22px] tracking-[-0.025em]">{g}</p>
            <ul className="m-0 mt-5 grid list-none gap-3 p-0">
              {items.map(it => (
                <li key={it} className="flex gap-3 text-[14.5px]">
                  <span className="mt-[3px] inline-block h-4 w-4 flex-none rounded-[4px] border border-line-strong" />
                  <span>{it}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </Section>
  );
}
