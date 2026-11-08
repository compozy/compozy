import { card, h3, lab } from "../cls";
import { Section } from "../parts/section";

const steps: [title: string, text: string][] = [
  ["The loop", "A tilted ring: work that goes around. Goal, act, verify, stop, and again."],
  [
    "The trace",
    "A second band behind the first, cut by it. The run leaves a trace: history you can read.",
  ],
  [
    "The fold",
    "Light catches the band where it turns. The moment the agent hands work back to a person.",
  ],
  [
    "The gradient",
    "Deep blue in the shadow, cyan where the light hits. Energy moving through a quiet system.",
  ],
];

const marker = "absolute mono-label text-[11px] text-muted-foreground";

export function Story() {
  return (
    <Section
      id="story"
      n="03"
      file="symbol.md"
      title={
        <>
          A cycle that runs on its own and <em>leaves a trace</em>
        </>
      }
      status="approved"
      intro={
        <p>
          The symbol is a loop and its trace. It replaces the four-lobed knot and the orange of the
          previous identity.
        </p>
      }
    >
      <div className="grid items-center gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div className="relative grid aspect-square place-items-center overflow-hidden rounded-hero bg-[radial-gradient(circle_at_55%_55%,color-mix(in_oklab,var(--color-brand-volt)_28%,transparent),transparent_60%)]">
          <img
            src="/brand/png/compozy-symbol-618.png"
            alt="Compozy symbol"
            className="w-[72%]"
            width="618"
            height="618"
          />
          <span className={`${marker} top-[26%] left-[12%]`}>← trace</span>
          <span className={`${marker} right-[8%] bottom-[18%]`}>loop →</span>
          <span className={`${marker} top-[47%] left-[20%] text-(--color-brand-glint)`}>fold</span>
        </div>
        <ol className="m-0 grid list-none gap-4 p-0">
          {steps.map(([t, d], i) => (
            <li key={t} className={card}>
              <p className={lab}>0{i + 1}</p>
              <p className="m-0 mt-2 font-display text-[22px] tracking-[-0.015em]">{t}</p>
              <p className="m-0 mt-1.5 text-[15px] text-(--b-muted)">{d}</p>
            </li>
          ))}
        </ol>
      </div>
      <div className={`${card} mt-10 border-dashed`}>
        <p className={h3}>Open item before registration</p>
        <p className="m-0 text-[15px] text-(--b-muted)">
          The approved symbol is a raster render. Its silhouette still sits close to a reference
          used during exploration, so it needs its own vector drawing before the trademark filing.
          Until then the PNG in <code>brand/png</code> is the official file and the traced
          one-colour version is for small and print use.
        </p>
      </div>
    </Section>
  );
}
