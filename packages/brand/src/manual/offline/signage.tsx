// 11 · 05 Signage: poster, roll-up, backdrop, directional signs and the reading-distance table.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { card, h3, lab, rules } from "../cls";
import { Frame } from "../kit-frame";
import { Section } from "../parts/section";
import { distances, signRules } from "./data";

const signs: [label: string, arrow: string][] = [
  ["Talks", "→"],
  ["Coffee", "←"],
  ["Demo room", "↑"],
];
const th = "px-5 py-3 font-medium";

function Backdrop() {
  return (
    <div>
      <p className={lab}>backdrop · 3000 × 2400 mm</p>
      <div
        className="relative mt-3 aspect-[5/4] overflow-hidden rounded-card border border-(--b-line)"
        style={{ containerType: "inline-size" }}
      >
        <Visual name="horizon" x={0.5} y={0.7} />
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_50%_42%,color-mix(in_oklab,var(--color-rail)_80%,transparent)_0%,color-mix(in_oklab,var(--color-rail)_55%,transparent)_40%,transparent_75%)]" />
        <div className="absolute inset-x-0 top-[28%] grid justify-items-center text-center">
          <img
            src="/brand/svg/compozy-lockup-dark.svg"
            alt="Compozy"
            style={{ height: "5.4cqi", width: "auto" }}
          />
          <p
            className="m-0 mt-[3cqi] font-display"
            style={{ fontSize: "5.6cqi", letterSpacing: "-.03em", fontWeight: 400 }}
          >
            Keep every agent running
          </p>
        </div>
        <div className="absolute inset-x-0 bottom-0 h-[33%] bg-[repeating-linear-gradient(45deg,color-mix(in_oklab,var(--color-destructive)_22%,transparent)_0_6px,transparent_6px_12px)]" />
        <p className="absolute right-3 bottom-2 m-0 mono-label text-[10.5px] tracking-[0.08em] text-muted-foreground">
          below 800 mm: hidden by people
        </p>
      </div>
    </div>
  );
}

function Directional() {
  return (
    <div>
      <p className={lab}>directional · A2 portrait</p>
      <div className="mt-3 grid gap-3">
        {signs.map(([t, a]) => (
          <div
            key={t}
            className="flex items-center justify-between rounded-[14px] bg-rail px-6 py-5 shadow-[0_0_0_1px_var(--b-line)]"
          >
            <span className="font-display text-[34px] tracking-[-0.025em]">{t}</span>
            <span className="font-mono text-[34px] text-(--color-brand-cyan)">{a}</span>
          </div>
        ))}
        <p className="m-0 text-[14px] text-(--b-muted)">
          Ink board, Instrument Sans, arrow in mono cyan at text height. Visuals stay off
          directional signs: they must read in one glance.
        </p>
      </div>
    </div>
  );
}

export function Signage() {
  return (
    <Section
      id="signage"
      n="05"
      file="offline/signage.md"
      title={
        <>
          Read from across the <em>room</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Signs are read in two seconds while walking. One line, sized for the distance, the art
          large and calm behind it. The poster and roll-up are in the kit; backdrop and directional
          signs follow the same rules.
        </p>
      }
    >
      <div className="grid gap-6 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <Frame id="poster-a3" guides />
        <Frame id="rollup" guides />
      </div>
      <p className="m-0 mt-3 text-sm text-(--b-muted)">
        Roll-up: hatched band is the 150 mm that slides into the cassette. Keep the headline above
        1,600 mm.
      </p>

      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <Backdrop />
        <Directional />
      </div>

      <div className="mt-10 grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div className="overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
          <table className="w-full min-w-[560px] border-collapse text-[14px]">
            <thead>
              <tr className="border-b border-(--b-line) text-left mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
                <th className={th}>Piece</th>
                <th className={th}>Read from</th>
                <th className={th}>Minimum main line</th>
                <th className={th}>Format</th>
              </tr>
            </thead>
            <tbody>
              {distances.map(([a, b, c, d]) => (
                <tr key={a} className="border-b border-(--b-line) last:border-0">
                  <td className="px-5 py-3 font-medium">{a}</td>
                  <td className="px-5 py-3 font-mono text-(--b-accent)">{b}</td>
                  <td className="px-5 py-3 font-mono text-[12.5px]">{c}</td>
                  <td className="px-5 py-3 text-(--b-muted)">{d}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className={card}>
          <h3 className={h3}>Rules</h3>
          <ul className={cn(rules, "mt-3 text-[15px]")}>
            {signRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
      </div>
    </Section>
  );
}
