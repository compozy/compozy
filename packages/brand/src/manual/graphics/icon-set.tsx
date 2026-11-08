// 04 Icons: the 1.5 px stroke set for product concepts, sizes, surfaces and the terminal glyphs.
import { cn } from "@compozy/ui/utils";

import { card, h3, lab } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { iconSizes, iconSpecs } from "./data";
import { Icon } from "./icon";
import { glyphs, iconNames, icons, type IconName } from "./icons";

const grid = Array.from({ length: 25 }, (_, i) => i);
/** glyph colour keys → tokens; "needs you" (◐) wears the attn amber of the product. */
const glyphColour: Record<string, string> = {
  cyan: "text-brand-cyan",
  warning: "text-attn",
  success: "text-success",
  danger: "text-destructive",
  muted: "text-muted-foreground",
};
const surfaces: [string, string][] = [
  ["bg-rail text-fg border border-line-strong", "On ink · #ececec"],
  ["bg-fg text-rail", "On paper · #0a0a0a"],
  ["bg-brand-volt text-brand-ink", "On volt · #FFFFFF"],
  ["bg-canvas text-brand-cyan border border-line-strong", "Selected · signal cyan"],
];
const sizeRow: IconName[] = ["session", "loop", "approval", "worktree", "trigger"];
const strokeAt: Record<number, string> = { 16: "1 px", 20: "1.25 px", 32: "2 px" };
const codePoint = (g: string) =>
  (g.codePointAt(0) ?? 0).toString(16).toUpperCase().padStart(4, "0");

function Construction() {
  return (
    <div className={cn(card, "grid content-start gap-5")}>
      <p className={lab}>Construction · loop</p>
      <svg
        viewBox="-1 -1 26 26"
        className="w-full max-w-[300px] text-fg"
        role="img"
        aria-label="Icon construction grid: 24 units, 20 unit live area, 1.5 stroke"
      >
        {grid.map(i => (
          <g key={i} className="stroke-line" strokeWidth="0.06">
            <line x1={i} x2={i} y1="0" y2="24" />
            <line y1={i} y2={i} x1="0" x2="24" />
          </g>
        ))}
        <rect
          x="2"
          y="2"
          width="20"
          height="20"
          fill="none"
          className="stroke-brand-cyan"
          strokeWidth="0.1"
          strokeDasharray="0.5 0.5"
        />
        <g
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          // Static markup from the icon set data (icons.ts).
          dangerouslySetInnerHTML={{ __html: icons.loop.path }}
        />
      </svg>
      <dl className="m-0 grid gap-0 text-[14px] [&_dd]:m-0 [&_dt]:font-mono [&_dt]:text-[12px] [&_dt]:text-(--b-muted) [&>div]:grid [&>div]:grid-cols-[72px_1fr] [&>div]:gap-3 [&>div]:border-b [&>div]:border-(--b-line) [&>div]:py-2">
        {iconSpecs.map(([k, v]) => (
          <div key={k}>
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function TheSet() {
  return (
    <div className={card}>
      <p className={lab}>The set · 24 px</p>
      <ul className="m-0 mt-4 grid list-none grid-cols-2 gap-3 p-0 sm:grid-cols-4">
        {iconNames.map(k => (
          <li
            key={k}
            className="grid content-start justify-items-center gap-1.5 rounded-[14px] border border-(--b-line) bg-rail px-2 py-4 text-center"
          >
            <Icon name={k} size={28} className="text-fg" />
            <span className="mt-1 text-[14px] font-medium">{icons[k].label}</span>
            <span className="font-mono text-[10.5px] text-muted-foreground">{k}.svg</span>
            <span className="text-[12px] leading-[1.35] text-muted-foreground">
              {icons[k].meaning}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function SizesAndSurfaces() {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <div className={card}>
        <p className={lab}>Sizes</p>
        <div className="mt-5 grid gap-4">
          {iconSizes.map(s => (
            <div key={s} className="flex items-center gap-4">
              <span className="w-12 font-mono text-[12px] text-muted-foreground">{s} px</span>
              {sizeRow.map(n => (
                <Icon key={n} name={n} size={s} className="text-fg" />
              ))}
              <span className="ml-auto font-mono text-[11px] text-faint">
                {strokeAt[s] ?? "1.5 px"}
              </span>
            </div>
          ))}
        </div>
        <p className="m-0 mt-5 text-[14px] text-(--b-muted)">
          16 in dense rows and menus, 20 in the sidebar, 24 in toolbars, 32 in empty states. Never
          larger: past 32 it becomes an illustration.
        </p>
      </div>
      <div className={card}>
        <p className={lab}>Surfaces</p>
        <div className="mt-5 grid grid-cols-2 gap-3">
          {surfaces.map(([c, t]) => (
            <div key={t} className={cn("grid gap-3 rounded-[12px] p-4", c)}>
              <span className="flex gap-3">
                <Icon name="session" size={22} />
                <Icon name="loop" size={22} />
                <Icon name="vault" size={22} />
              </span>
              <span className="font-mono text-[11px] opacity-80">{t}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function Glyphs() {
  return (
    <>
      <h3 className={cn(h3, "mt-16")}>Terminal glyphs</h3>
      <p className="m-0 max-w-[62ch] text-[15.5px] text-(--b-muted)">
        Six Unicode characters do the work of icons wherever text runs: the CLI, logs, product rows,
        social posts. Each has one meaning. They render in Geist Mono and keep its width.
      </p>
      <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {glyphs.map(([g, n, m, c]) => (
          <div
            key={n}
            className="flex items-center gap-4 rounded-[14px] border border-(--b-line) bg-(--b-card) p-4"
          >
            <span
              className={cn(
                "grid size-14 shrink-0 place-items-center rounded-[12px] bg-rail font-mono text-[26px]",
                glyphColour[c]
              )}
            >
              {g}
            </span>
            <span>
              <span className="block mono-label tracking-[0.08em] text-muted-foreground">
                {n} · U+{codePoint(g)}
              </span>
              <span className="block text-[15px]">{m}</span>
            </span>
          </div>
        ))}
      </div>
    </>
  );
}

function IconDoDont() {
  return (
    <DoDont
      className="mt-10"
      doContent={
        <>
          <div className="flex flex-wrap items-center gap-3 font-mono text-[13px]">
            <span className="flex items-center gap-2 rounded-[10px] border border-line-strong px-3 py-2">
              <Icon name="approval" size={16} className="text-attn" /> 2 approvals
            </span>
            <span className="flex items-center gap-2 rounded-[10px] border border-line-strong px-3 py-2">
              <Icon name="worktree" size={16} /> wt/review-pr
            </span>
          </div>
          <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
            Icon beside the word it labels, same colour as the text or the state.
          </p>
        </>
      }
      dontContent={
        <>
          <div className="flex flex-wrap items-center gap-3 text-[13px]">
            <span className="rounded-[10px] px-3 py-2 text-[20px] [background:var(--gradient-brand)]">
              🤖
            </span>
            <svg viewBox="0 0 24 24" width="40" height="40" aria-hidden="true">
              <defs>
                <linearGradient id="ig" x1="0" y1="1" x2="1" y2="0">
                  <stop offset="0" style={{ stopColor: "var(--color-brand-sky)" }} />
                  <stop offset="1" style={{ stopColor: "var(--color-brand-cyan)" }} />
                </linearGradient>
              </defs>
              <path d="M12 2l3 7h7l-5.5 4.5 2 7.5L12 16.5 5.5 21l2-7.5L2 9h7z" fill="url(#ig)" />
            </svg>
            <span className="text-[22px]">✨</span>
          </div>
          <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
            Emoji, filled gradient icons, sparkles for “AI”. The gradient never fills an icon.
          </p>
        </>
      }
    />
  );
}

export function IconSet() {
  return (
    <Section
      id="icons"
      n="04"
      file="icons.svg"
      title={
        <>
          Sixteen icons, one <em>stroke</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          A small set for the product’s own nouns. 24 px grid, 1.5 px stroke, round ends, one
          colour. Icons label things that exist in Compozy; they never decorate a headline or
          replace a word.
        </p>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[minmax(0,340px)_minmax(0,1fr)]">
        <Construction />
        <div className="grid min-w-0 content-start gap-4">
          <TheSet />
          <SizesAndSurfaces />
        </div>
      </div>
      <Glyphs />
      <IconDoDont />
    </Section>
  );
}
