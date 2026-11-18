// Formats, composition: the four parts of a piece, word limits and logo corners, drawn live.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { cap, card, h3 } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { compositionParts, logoCorners, wordLimits } from "./data";

const pos = [
  "top-[7%] left-[6.25%]",
  "bottom-[7%] left-[6.25%]",
  "top-[7%] right-[6.25%]",
  "bottom-[7%] right-[6.25%]",
];
const markers: [n: string, place: string][] = [
  ["4", "top-[8%] left-[2.4%]"],
  ["3", "bottom-[51%] left-[2.4%]"],
  ["2", "bottom-[31%] left-[2.4%]"],
  ["3", "bottom-[10.5%] left-[2.4%]"],
  ["1", "top-[40%] right-[22%]"],
];
const scrim = (stop: number, end: number) =>
  `linear-gradient(90deg, var(--color-rail) 0%, color-mix(in oklab, var(--color-rail) ${stop === 42 ? 80 : 85}%, transparent) ${stop}%, transparent ${end}%)`;

function Anatomy() {
  return (
    <figure className="m-0 min-w-0">
      <div className="relative aspect-[16/9] overflow-hidden rounded-card shadow-[0_0_0_1px_var(--b-line)] [container-type:inline-size]">
        <Visual name="swarm" x={0.86} y={0.5} />
        <div className="absolute inset-0" style={{ background: scrim(42, 72) }} />
        <img
          src="/brand/svg/compozy-lockup-dark.svg"
          alt=""
          className="absolute top-[11.1%] left-[6.25%] h-[2.6cqi] w-auto"
        />
        <div className="absolute bottom-[19%] left-[6.25%]">
          <p
            className="m-0 mono-label text-brand-cyan"
            style={{ fontSize: "1.25cqi", letterSpacing: "0.12em" }}
          >
            Release 0.6 · Loops
          </p>
          <p className="m-0 mt-[1.6cqi] max-w-[11em] font-display text-[5.4cqi] leading-[1.05] font-normal tracking-[-0.03em]">
            Agents that keep running after the tab closes.
          </p>
        </div>
        <p
          className="absolute bottom-[11.1%] left-[6.25%] m-0 font-mono text-muted-foreground"
          style={{ fontSize: "1.1cqi", letterSpacing: "0.06em" }}
        >
          compozy.com/changelog
        </p>
        {markers.map(([n, p]) => (
          <span
            key={p}
            className={cn(
              "absolute grid h-[2.6cqi] min-h-5 w-[2.6cqi] min-w-5 place-items-center rounded-full bg-brand-cyan font-mono text-[max(11px,1.3cqi)] font-medium text-rail",
              p
            )}
          >
            {n}
          </span>
        ))}
      </div>
      <figcaption className={cap}>
        1 visual · 2 headline · 3 mono metadata (kicker and footer) · 4 logo. Same anatomy as the X
        post template.
      </figcaption>
    </figure>
  );
}

function Parts() {
  return (
    <div className={card}>
      <p className={h3}>The four parts</p>
      <ol className="m-0 grid list-none gap-3 p-0">
        {compositionParts.map(([t, d, limit], i) => (
          <li
            key={t}
            className="grid grid-cols-[28px_minmax(0,1fr)] gap-3 border-b border-(--b-line) pb-3 last:border-0"
          >
            <span className="grid h-6 w-6 place-items-center rounded-full bg-brand-cyan font-mono text-[12px] text-rail">
              {i + 1}
            </span>
            <div>
              <p className="m-0 font-medium">
                {t} <span className="ml-1 font-mono text-[12px] text-(--b-accent)">{limit}</span>
              </p>
              <p className="m-0 mt-0.5 text-[14px] text-(--b-muted)">{d}</p>
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

function WordLimits() {
  return (
    <div className={card}>
      <p className={h3}>Word limits</p>
      <dl className="m-0 grid">
        {wordLimits.map(([k, v, d]) => (
          <div
            key={k}
            className="grid grid-cols-[minmax(0,7rem)_minmax(0,5.5rem)_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-2.5 last:border-0"
          >
            <dt className="text-[15px] font-medium">{k}</dt>
            <dd className="m-0 font-mono text-[13px] text-(--b-accent)">{v}</dd>
            <dd className="m-0 text-[13.5px] text-(--b-muted)">{d}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function LogoCorners() {
  return (
    <div className={card}>
      <p className={h3}>Logo corners</p>
      <div className="mt-3 grid grid-cols-2 gap-3">
        {logoCorners.map(([t, d], i) => (
          <div key={t}>
            <div className="relative aspect-[16/10] overflow-hidden rounded-[10px] bg-rail shadow-[0_0_0_1px_var(--b-line)]">
              <Visual name="swarm" x={i === 2 ? 0.15 : 0.85} y={0.5} className="opacity-80" />
              <span
                className={cn(
                  "absolute h-[9%] w-[26%] rounded-[3px]",
                  pos[i],
                  i === 3 ? "border border-dashed border-destructive bg-destructive/25" : "bg-fg"
                )}
              />
            </div>
            <p className="m-0 mt-2 text-[14px] font-medium">
              {i === 3 ? "✕ " : ""}
              {t}
            </p>
            <p className="m-0 text-[13px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function DoPiece() {
  return (
    <div className="relative aspect-[16/9] overflow-hidden rounded-[14px] [container-type:inline-size]">
      <Visual name="stack" x={0.88} y={0.5} />
      <div className="absolute inset-0" style={{ background: scrim(38, 68) }} />
      <p
        className="absolute top-[9%] left-[6%] m-0 mono-label text-brand-cyan"
        style={{ fontSize: "2cqi", letterSpacing: "0.12em" }}
      >
        Triggers
      </p>
      <p className="absolute bottom-[10%] left-[6%] m-0 max-w-[9em] font-display text-[5.6cqi] leading-[1.05] font-normal tracking-[-0.03em]">
        One event starts every run that listens.
      </p>
    </div>
  );
}

function DontPiece() {
  return (
    <div className="relative aspect-[16/9] overflow-hidden rounded-[14px] [container-type:inline-size]">
      <Visual name="swarm" x={0.85} y={0.3} />
      <div className="absolute inset-0 mix-blend-screen [mask-image:linear-gradient(0deg,black_0%,transparent_65%)]">
        <Visual name="terrain" x={0.6} y={0.8} />
      </div>
      <p className="absolute top-[7%] left-[5%] m-0 max-w-[13em] font-display text-[4.6cqi] leading-[1.05] font-normal">
        The control plane for your agents.
      </p>
      <p className="absolute top-[30%] left-[5%] m-0 max-w-[16em] font-display text-[3.4cqi] leading-[1.05] font-normal text-brand-cyan">
        Loops, triggers, memory, approvals and more
      </p>
      <ul className="absolute bottom-[6%] left-[5%] m-0 pl-[3cqi] text-[2.3cqi] leading-[1.35]">
        <li>Sessions</li>
        <li>Loops</li>
        <li>Triggers</li>
        <li>Vault</li>
      </ul>
    </div>
  );
}

export function Composition() {
  return (
    <Section
      id="composition"
      n="04"
      file="composition.md"
      title={
        <>
          One visual, one line, one <em>label</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          A Compozy piece has four parts and never more: a visual, a headline, a line of mono
          metadata and the logo. Each has a place and a word limit. If the message needs more, it
          needs a second piece.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
        <Anatomy />
        <Parts />
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <WordLimits />
        <LogoCorners />
      </div>
      <DoDont
        className="mt-4"
        doLabel="Eight words, one visual"
        dontLabel="Two headlines, a list and two visuals"
        doContent={<DoPiece />}
        dontContent={<DontPiece />}
      />
    </Section>
  );
}
