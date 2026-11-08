import { card, h3, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { photo, principles } from "./data";

const TESTS: [question: string, answer: string][] = [
  [
    "Would a colleague take it?",
    "If the shot needs a stylist, a ring light or a direction like “look excited”, it is an ad. Reshoot it from the side of the room.",
  ],
  [
    "Does it sit on ink?",
    "Place it on #0a0a0a. If the edge of the photo is visible because its blacks are deeper or warmer, the grade is not applied yet.",
  ],
  [
    "Can you read a screen?",
    "Zoom to 200%. Any legible word, name or number on a screen means a blur pass or a different frame.",
  ],
];

export function Direction() {
  const hero = photo("operator-night");
  const side = [photo("laptop-closed"), photo("hands-detail")];
  return (
    <Section
      id="direction"
      n="01"
      file="direction.md"
      title={
        <>
          Real work, in <em>cold</em> light
        </>
      }
      intro={
        <p>
          Compozy photographs people doing the work agents hand back to them: reviewing, deciding,
          leaving the desk while a run continues. The light is cold, the angle is documentary, and
          screens are light sources, never content.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)]">
        <GenImage src={hero.src} alt={hero.alt} caption={hero.caption} ratio="4/5" />
        <div className="grid content-start gap-4">
          {side.map(p => (
            <GenImage key={p.src} src={p.src} alt={p.alt} caption={p.caption} ratio="3/2" />
          ))}
        </div>
      </div>

      <div className="mt-16 grid gap-px overflow-hidden rounded-card border border-(--b-line) bg-(--b-line)">
        {principles.map(p => (
          <div
            key={p.n}
            className="grid gap-3 bg-(--b-card) p-6 md:grid-cols-[64px_minmax(0,0.9fr)_minmax(0,1.3fr)_minmax(0,1.1fr)] md:gap-6 md:p-7"
          >
            <span className="font-mono text-[13px] text-(--b-accent)">{p.n}</span>
            <p className="m-0 font-display text-[22px] leading-[1.2] tracking-[-0.02em]">
              {p.name}
            </p>
            <p className="m-0 text-[15px]">{p.rule}</p>
            <p className="m-0 text-[14px] text-(--b-muted)">
              <span className="mono-label text-[11px]">Why · </span>
              {p.why}
            </p>
          </div>
        ))}
      </div>

      <h3 className={`${h3} mt-16`}>The test before any photo ships</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        {TESTS.map(([q, a]) => (
          <div key={q} className={card}>
            <p className={lab}>{q}</p>
            <p className="m-0 mt-3 text-[15px]">{a}</p>
          </div>
        ))}
      </div>
      <DoDont
        className="mt-4"
        doContent={
          <p className="m-0 text-[15px]">
            A person at a desk at night, face lit by the monitor, looking at the work. The screen is
            a soft rectangle of light. Cool shadows, open blacks.
          </p>
        }
        dontContent={
          <p className="m-0 text-[15px] text-(--b-muted)">
            A smiling model pointing at a laptop with a readable dashboard, warm office light, a
            plant, a coffee cup turned to show its logo.
          </p>
        }
      />
    </Section>
  );
}
