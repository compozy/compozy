import { cn } from "@compozy/ui/utils";

import { card, h3, lab, rules, specs } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { thumbRules } from "./data";

const channel: [string, string][] = [
  ["Name", "Compozy"],
  [
    "Description",
    "Walkthroughs of Compozy, the autonomous control plane for your agents. One task per video, on a real machine.",
  ],
  ["Titles", "Outcome first, ≤ 60 characters: “Run five agents from one desktop”."],
  ["Playlists", "Getting started · Always-on agents · Orchestration · Talks"],
  ["End screen", "One video and the subscribe element. No “smash the like button”."],
];

export function Youtube() {
  return (
    <Section
      id="youtube"
      n="05"
      file="youtube.md"
      title={
        <>
          YouTube: shown <em>working</em>
        </>
      }
      bg="ink2"
      intro={
        <p>
          Videos show one task from start to finish on a real machine. The channel art states the
          line once; thumbnails promise the outcome in four words and prove it with the desktop.
        </p>
      }
    >
      <Frame id="youtube-banner" guides />
      <p className="m-0 mt-3 max-w-[70ch] text-[14.5px] text-(--b-muted)">
        TV shows the full 2560×1440. Desktop shows the 2560×423 strip. Phones show the centred
        1546×423: the cyan box. Lockup and line live there; the Converge visual fills the rest.
      </p>

      <h3 className={cn(h3, "mt-16")}>Thumbnails</h3>
      <div className="mt-4 grid items-start gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <Frame id="youtube-thumb" guides />
        <div className={card}>
          <ul className={cn(rules, "text-[15px]")}>
            {thumbRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <p className={cn(lab, "mt-6")}>At feed size</p>
          <div className="mt-3 flex items-end gap-4">
            <div className="w-[168px]">
              <Frame id="youtube-thumb" caption={false} />
            </div>
            <div className="w-[120px]">
              <Frame id="youtube-thumb" caption={false} />
            </div>
          </div>
          <p className="m-0 mt-2 font-mono text-[11.5px] text-(--b-muted)">
            168 px search · 120 px sidebar
          </p>
        </div>
      </div>

      <div className="mt-16 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)]">
        <div className="mx-auto w-full max-w-[300px]">
          <Frame id="story" guides />
        </div>
        <div className="grid gap-4">
          <div className={card}>
            <p className={lab}>Shorts and vertical covers</p>
            <p className="m-0 mt-3 text-[15px]">
              The same template covers YouTube Shorts, vertical video on X and LinkedIn, and Discord
              event images. The app covers the top 250 px and bottom 340 px: nothing important
              there.
            </p>
          </div>
          <div className={card}>
            <p className={lab}>Channel</p>
            <dl className={cn(specs, "mt-3")}>
              {channel.map(([k, v]) => (
                <div key={k}>
                  <dt>{k}</dt>
                  <dd>{v}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      </div>
    </Section>
  );
}
