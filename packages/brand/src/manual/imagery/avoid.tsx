import { lab } from "../cls";
import { GenImage } from "../parts/gen-image";
import { Section } from "../parts/section";
import { avoid, photo } from "./data";

const WRONG: [n: string, text: string][] = [
  ["01", "A body for a process. Agents in Compozy are sessions, not robots."],
  ["02", "Glow as content. The light means nothing, so the image means nothing."],
  ["03", "Off palette: purple, warm highlights, crushed blacks."],
  ["04", "Interchangeable. Swap the logo and it works for any AI company."],
];

export function Avoid() {
  const bad = photo("avoid-stock");
  return (
    <Section
      id="avoid"
      n="06"
      file="avoid.md"
      bg="ink2"
      title={
        <>
          The AI stock image, <em>never</em>
        </>
      }
      intro={
        <p>
          Most AI companies illustrate themselves with the same eight pictures. Each one says
          “artificial intelligence” and nothing else. Compozy shows the work and the person instead,
          so the image makes a claim.
        </p>
      }
    >
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
        <div className="relative">
          <GenImage src={bad.src} alt={bad.alt} caption={bad.caption} ratio="3/2" />
          <span className="absolute top-4 left-4 rounded-full bg-rail/80 px-3 py-1 font-mono text-[12px] text-destructive">
            ✕ counter-example
          </span>
        </div>
        <div className="rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)] bg-(--b-card) p-7">
          <p className={lab}>What it gets wrong</p>
          <ul className="m-0 mt-4 list-none space-y-3 p-0 text-[15.5px]">
            {WRONG.map(([n, t]) => (
              <li key={n}>
                <span className="font-mono text-destructive">{n}</span> {t}
              </li>
            ))}
          </ul>
        </div>
      </div>
      <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {avoid.map(a => (
          <div
            key={a.name}
            className="flex min-w-0 flex-col rounded-card border border-(--b-line) bg-(--b-card) p-6"
          >
            <p className="m-0 mono-label text-destructive">✕ {a.name}</p>
            <p className="m-0 mt-3 flex-1 text-[14.5px] text-(--b-muted)">{a.why}</p>
            <p className="m-0 mt-4 border-t border-(--b-line) pt-3 text-[14.5px]">
              <span className="mono-label text-[11px] text-(--b-accent)">Instead · </span>
              {a.instead}
            </p>
          </div>
        ))}
      </div>
    </Section>
  );
}
