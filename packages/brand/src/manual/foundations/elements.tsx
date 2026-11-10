import { Visual } from "../art/visual";
import { card, lab } from "../cls";
import { Section } from "../parts/section";

/** run name, harness, state label, state tone, glyph */
const runs: [string, string, string, string, string][] = [
  ["review-pr", "claude-code", "Working", "text-(--color-brand-cyan)", "●"],
  ["migrate-db", "codex", "Waiting for you", "text-warning", "◐"],
  ["triage-issues", "cursor", "Done", "text-success", "✓"],
  ["release-notes", "gemini", "Stuck", "text-destructive", "■"],
];

const metrics: [value: string, label: string][] = [
  ["12", "running"],
  ["3", "need you"],
  ["41", "done today"],
];

const CELLS = Array.from({ length: 18 }, (_, i) => i);
const LIT_CELL = 8;
const note = "m-0 mt-6 text-[14px] text-(--b-muted)";

export function Elements() {
  return (
    <Section
      id="elements"
      n="07"
      file="elements.astro"
      title={
        <>
          The parts every piece is <em>built</em> from
        </>
      }
      status="proposal"
      intro={
        <p>
          Six elements carry the identity beyond the logo. They come from the product itself, so
          marketing and interface look like the same thing.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        <div className={card}>
          <p className={lab}>01 · Mono label</p>
          <div className="mt-6 grid gap-3 mono-label tracking-[0.12em]">
            <span className="text-(--color-brand-cyan)">release 0.6 · beta</span>
            <span className="text-muted-foreground">sessions / loops / triggers</span>
            <span className="text-muted-foreground">02 · symbol.md</span>
          </div>
          <p className={note}>
            Metadata, eyebrows, file names. It says where you are, never what to feel.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>02 · Chip and pill</p>
          <div className="mt-6 flex flex-wrap gap-2">
            <span className="rounded-full border border-line-strong px-3 py-1.5 font-mono text-[12px] text-(--color-brand-cyan)">
              ● 12 running
            </span>
            <span className="rounded-full border border-line-strong px-3 py-1.5 font-mono text-[12px] text-muted-foreground">
              always-on
            </span>
            <span className="rounded-md border border-line-strong px-2 py-1 font-mono text-[11px] text-(--color-brand-cyan)">
              OS
            </span>
            <span className="rounded-full bg-elevated px-3 py-1.5 text-[13px] font-medium">
              Needs you
            </span>
          </div>
          <p className={note}>
            Outline by default, filled only when selected. Pills hold state, chips hold identifiers.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>03 · Status dot</p>
          <ul className="m-0 mt-5 grid list-none gap-2 p-0 text-[14px]">
            {runs.map(([n, h, s, tone, g]) => (
              <li
                key={n}
                className="flex items-center justify-between rounded-[10px] border border-(--b-line) px-3 py-2 font-mono text-[12.5px]"
              >
                <span>
                  {n} <span className="text-muted-foreground">· {h}</span>
                </span>
                <span className={tone}>
                  {g} {s}
                </span>
              </li>
            ))}
          </ul>
        </div>
        <div className={card}>
          <p className={lab}>04 · Metric</p>
          <div className="mt-5 grid grid-cols-3 gap-3">
            {metrics.map(([v, l]) => (
              <div key={l}>
                <p className="m-0 font-display text-[44px] leading-none tracking-[-0.04em]">{v}</p>
                <p className="m-0 mt-2 mono-label text-[11px] text-muted-foreground">{l}</p>
              </div>
            ))}
          </div>
          <p className={note}>
            Number in Instrument Sans, label in mono below. Real or clearly labelled sample.
          </p>
        </div>
        <div className={card}>
          <p className={lab}>05 · Hairline grid</p>
          <div className="mt-5 grid aspect-[16/7] grid-cols-6 grid-rows-3 overflow-hidden rounded-[10px] border border-line-strong">
            {CELLS.map(i => (
              <span
                key={i}
                className={
                  i === LIT_CELL
                    ? "border-r border-b border-line [background:var(--gradient-brand)]"
                    : "border-r border-b border-line"
                }
              />
            ))}
          </div>
          <p className={note}>
            1 px lines in line colour. One cell may carry light; the rest stays quiet.
          </p>
        </div>
        <div className={`${card} relative overflow-hidden`}>
          <p className={lab}>06 · Visuals</p>
          <div className="relative mt-5 aspect-[16/7] overflow-hidden rounded-[10px]">
            <Visual name="converge" x={0.7} sizes="(min-width: 1024px) 33vw, 100vw" />
          </div>
          <p className={note}>
            Atmosphere for mood, Live for the product. Its own chapter:{" "}
            <a href="/visuals" className="text-(--b-accent)">
              Visual system →
            </a>
          </p>
        </div>
      </div>
    </Section>
  );
}
