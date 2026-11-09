import { cn } from "@compozy/ui/utils";

import { cap, card, h3, lab } from "../cls";
import { Section } from "../parts/section";
import { crops, cssRecipe, lightroom, photo, recipe } from "./data";
import { Graded } from "./graded";
import { SpecCard, SpecList } from "../parts/spec-card";

const NOTES: [label: string, text: string][] = [
  [
    "Radius",
    "14 px in the manual and on the site at 1x; 20 px when the photo is the card. Never a circle, except avatars.",
  ],
  [
    "Text on photos",
    "Only over the darkest third, with an ink gradient from 0% to 85% behind it. Contrast at least 4.5:1, measured on the lightest pixel under the text.",
  ],
  [
    "Retouch",
    "Remove temporary marks, stray cables and logos on hardware. Never reshape faces or bodies, never smooth skin, never add people.",
  ],
];

function BeforeAfter() {
  const base = photo("portrait-window");
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <figure className="m-0">
        <Graded src={base.src} alt={base.alt} grade={false} ratio="4/5" />
        <figcaption className={cap}>
          <span className="mono-label tracking-[0.08em]">Before</span> · straight from the camera or
          the generator
        </figcaption>
      </figure>
      <figure className="m-0">
        <Graded src={base.src} alt={base.alt} ratio="4/5" />
        <figcaption className={cap}>
          <span className="mono-label tracking-[0.08em] text-(--b-accent)">After</span> · the CSS
          recipe below, applied live
        </figcaption>
      </figure>
    </div>
  );
}

function Recipes() {
  return (
    <div className="mt-4 grid gap-4 lg:grid-cols-3">
      <SpecCard title="Recipe" rows={recipe} />
      <div className={card}>
        <p className={lab}>Lightroom · starting point</p>
        <SpecList rows={lightroom} valueClassName="font-mono" />
        <p className={cn(cap, "mb-0")}>
          Saved as the preset <code>compozy-cool-v1</code> in the shared library.
        </p>
      </div>
      <div className="min-w-0 overflow-hidden rounded-card bg-rail text-fg">
        <div className="flex items-center justify-between gap-3 border-b border-line px-5 py-3">
          <span className="font-mono text-[12.5px] text-muted-foreground">photo.css · web</span>
          <button
            type="button"
            data-copy-from="cz-css-recipe"
            className="cursor-pointer rounded-lg border border-line bg-transparent px-3 py-1 font-mono text-[12px] text-fg hover:border-brand-cyan"
          >
            copy
          </button>
        </div>
        <pre
          id="cz-css-recipe"
          className="m-0 p-5 font-mono text-[12px] leading-[1.7] break-words whitespace-pre-wrap"
        >
          <code>{cssRecipe}</code>
        </pre>
      </div>
    </div>
  );
}

function InkPoint() {
  const night = photo("operator-night");
  return (
    <>
      <h3 className={`${h3} mt-16`}>The ink point</h3>
      <p className="m-0 max-w-[62ch] text-[16px] text-(--b-muted)">
        Lifted blacks are what make a photo belong to the page. Left: crushed to #000, the frame
        shows as a hole in the ink. Right: lifted to #0a0a0a, the photo melts into the band.
      </p>
      <div className="mt-5 grid gap-4 rounded-card bg-rail p-[clamp(16px,3vw,32px)] md:grid-cols-2">
        <figure className="m-0">
          <div className="overflow-hidden rounded-[14px]" style={{ aspectRatio: "3/2" }}>
            <img
              src={night.src}
              alt={night.alt}
              loading="lazy"
              className="block h-full w-full object-cover"
              style={{ filter: "contrast(1.35) brightness(.8) sepia(.35) saturate(1.3)" }}
            />
          </div>
          <figcaption className="mt-2.5 mono-label text-[11.5px] tracking-[0.08em] text-destructive">
            ✕ crushed, warm, saturated
          </figcaption>
        </figure>
        <figure className="m-0">
          <Graded src={night.src} alt={night.alt} ratio="3/2" className="!shadow-none" />
          <figcaption className="mt-2.5 mono-label text-[11.5px] tracking-[0.08em] text-brand-cyan">
            ✓ lifted to ink, cool, −20%
          </figcaption>
        </figure>
      </div>
    </>
  );
}

function Crops() {
  return (
    <>
      <h3 className={`${h3} mt-16`}>Crops per ratio</h3>
      <p className="m-0 max-w-[62ch] text-[16px] text-(--b-muted)">
        Five ratios cover every channel. Eyes sit on the top third line; leave a head of air above
        the head. Faces never cross a template’s margin.
      </p>
      <div className="mt-6 grid grid-cols-2 items-end gap-4 md:grid-cols-3 lg:grid-cols-5">
        {crops.map(c => {
          const p = photo(c.id);
          return (
            <figure key={c.label} className="m-0 min-w-0">
              <Graded src={p.src} alt={p.alt} ratio={c.ratio} pos={c.pos} />
              <figcaption className={cap}>
                <span className="font-mono font-medium text-(--b-fg)">{c.label}</span> · {c.use}
              </figcaption>
            </figure>
          );
        })}
      </div>
      <div className="mt-6 grid gap-4 md:grid-cols-3">
        {NOTES.map(([k, v]) => (
          <div key={k} className={card}>
            <p className={lab}>{k}</p>
            <p className="m-0 mt-3 text-[15px]">{v}</p>
          </div>
        ))}
      </div>
    </>
  );
}

export function Treatment() {
  return (
    <Section
      id="treatment"
      n="04"
      file="treatment.css"
      bg="paper"
      title={
        <>
          One grade for <em>every</em> photo
        </>
      }
      intro={
        <p>
          A single recipe makes a portrait, a meetup and a desk detail look like one family, and
          lets each one sit on ink without a visible edge. Values are starting points; judge the
          result on #0a0a0a.
        </p>
      }
    >
      <BeforeAfter />
      <Recipes />
      <InkPoint />
      <Crops />
    </Section>
  );
}
