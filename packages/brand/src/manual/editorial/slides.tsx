import { card, lab } from "../cls";
import { imageById, imgSrc } from "../images";
import { Frame } from "../kit-frame";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { slideRules } from "./data";
import { Rows } from "./rows";

const talk: [n: string, template: string, count: string][] = [
  ["01", "slide-title", "1"],
  ["02", "slide-content", "3 – 5 per part"],
  ["03", "slide-section", "1 per part, 3 parts"],
  ["04", "slide-code", "1 – 2 per part"],
  ["05", "slide-diagram", "1 per part"],
  ["06", "slide-title", "closing: one link, one CTA"],
];

export function Slides() {
  const stage = imageById("slide-talk");
  return (
    <Section
      id="slides"
      n="06"
      file="deck.key"
      title={
        <>
          Five slides, <em>every</em> talk
        </>
      }
      status="proposal"
      intro={
        <p>
          Talks, meetups and demos all use the same 16:9 system: a title, section dividers over the
          horizon visual, content, code and diagrams. Five templates are enough for a 40-minute talk
          and keep every deck recognisable.
        </p>
      }
    >
      <div className="grid gap-6 md:grid-cols-2">
        <Frame id="slide-title" guides />
        <Frame id="slide-section" guides />
        <Frame id="slide-content" guides />
        <Frame id="slide-code" guides />
      </div>
      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
        <Frame id="slide-diagram" guides />
        <div className={card}>
          <p className={lab}>Deck rules</p>
          <Rows rows={slideRules} className="mt-3" />
        </div>
      </div>
      <div className="mt-6 grid gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <GenImage
          src={imgSrc(stage)}
          alt={stage.alt}
          caption={stage.caption}
          ratio={stage.ratio.replace("/", " / ")}
        />
        <div className={card}>
          <p className={lab}>A 40-minute talk</p>
          <ol className="m-0 mt-4 grid list-none gap-0 p-0 font-mono text-[13px]">
            {talk.map(([n, t, c]) => (
              <li
                key={n}
                className="grid grid-cols-[36px_minmax(0,1fr)_auto] gap-3 border-b border-(--b-line) py-2.5 last:border-0"
              >
                <span className="text-(--b-accent)">{n}</span>
                <span>{t}</span>
                <span className="text-(--b-muted)">{c}</span>
              </li>
            ))}
          </ol>
          <p className="m-0 mt-4 text-[14px] text-(--b-muted)">
            About 30 slides. More than one slide a minute means the talk is reading its slides.
          </p>
        </div>
      </div>
    </Section>
  );
}
