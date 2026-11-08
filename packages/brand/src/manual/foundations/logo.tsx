import type { CSSProperties } from "react";

import { card, h3, lab, rules } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";

const B = "/brand";
const INK = "bg-rail";
const PAPER = "bg-fg";

const stages = [
  { bg: INK, label: "Primary · on ink", src: `${B}/svg/compozy-lockup-dark.svg` },
  { bg: PAPER, label: "Primary · on paper", src: `${B}/svg/compozy-lockup-light.svg` },
  { bg: INK, label: "With OS chip", src: `${B}/svg/compozy-lockup-os-dark.svg` },
  {
    bg: "bg-(--color-brand-volt)",
    label: "One colour · on volt",
    src: `${B}/svg/compozy-lockup-mono-white.svg`,
  },
  { bg: PAPER, label: "One colour · ink", src: `${B}/svg/compozy-lockup-mono-ink.svg` },
  {
    bg: "[background:var(--gradient-brand)]",
    label: "One colour · on gradient",
    src: `${B}/svg/compozy-lockup-mono-white.svg`,
  },
];

const files: [file: string, desc: string][] = [
  ["compozy-lockup-dark.svg", "Horizontal, white type, gradient symbol"],
  ["compozy-lockup-light.svg", "Horizontal, ink type"],
  ["compozy-lockup-os-dark.svg", "With OS chip, on ink"],
  ["compozy-lockup-os-light.svg", "With OS chip, on paper"],
  ["compozy-lockup-stacked-dark.svg", "Stacked, on ink"],
  ["compozy-lockup-mono-white.svg", "One colour, white"],
  ["compozy-lockup-mono-ink.svg", "One colour, ink"],
  ["compozy-symbol.svg", "Symbol, gradient (raster inside)"],
  ["compozy-symbol-mono-white.svg", "Symbol, one colour, vector"],
  ["compozy-wordmark-white.svg", "Wordmark only, outlined"],
  ["compozy-app-icon.svg", "App icon, 1024"],
];

const misuse: [title: string, style: CSSProperties][] = [
  ["Recolour the symbol", { filter: "hue-rotate(140deg)" }],
  ["Stretch or skew", { transform: "scaleX(1.4)" }],
  ["Rotate to “level” it", { transform: "rotate(50deg)" }],
  [
    "Add effects",
    {
      filter:
        "drop-shadow(0 0 12px var(--color-brand-cyan)) drop-shadow(0 0 24px var(--color-brand-cyan))",
    },
  ],
];

const symbolSizes: [display: number, file: number][] = [
  [96, 256],
  [48, 128],
  [24, 48],
];

function LogoStages() {
  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {stages.map(s => (
        <figure key={s.label} className="m-0">
          <div
            className={`grid aspect-[16/9] place-items-center rounded-card border border-(--b-line) ${s.bg}`}
          >
            <img src={s.src} alt="" className="h-[19%] w-auto" />
          </div>
          <figcaption className="mt-2.5 mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)">
            {s.label}
          </figcaption>
        </figure>
      ))}
    </div>
  );
}

function Construction() {
  return (
    <div className="mt-16 grid gap-4 lg:grid-cols-3">
      <div className={`${card} lg:col-span-2`}>
        <p className={lab}>Construction</p>
        <div className="relative mt-6 grid place-items-center rounded-[14px] bg-rail py-14">
          <div className="relative inline-block p-[3.2%] outline-1 outline-dashed outline-[color-mix(in_oklab,var(--color-brand-cyan)_60%,transparent)]">
            <img src={`${B}/svg/compozy-lockup-dark.svg`} alt="" className="block h-16 w-auto" />
            <span className="absolute -top-6 left-0 font-mono text-[10.5px] text-(--color-brand-cyan)">
              clear space = ½ symbol height
            </span>
          </div>
        </div>
        <ul className={`${rules} mt-6 text-[15px]`}>
          <li>
            <strong>Symbol height</strong> is 1.12× the wordmark’s font size; the gap is 0.32 em.
          </li>
          <li>
            <strong>Clear space</strong> is half the symbol height on every side. Nothing enters it,
            not even the OS chip of another lockup.
          </li>
          <li>
            <strong>Minimum size</strong>: lockup 88 px wide on screen, 22 mm in print. Below that,
            use the symbol alone.
          </li>
          <li>
            <strong>Favicon and app icon</strong> are the symbol only. The wordmark never appears
            under 14 px cap height.
          </li>
          <li>
            <strong>Spelling</strong>: Compozy, capital C. The CLI is <code>compozy</code>. Never
            “CompoZy”, “COMPOZY” or “Compozy.ai”.
          </li>
        </ul>
      </div>
      <div className={`${card} flex flex-col gap-5`}>
        <p className={lab}>Symbol alone</p>
        <div className="grid grid-cols-3 items-end gap-3">
          {symbolSizes.map(([s, file]) => (
            <div key={s} className="grid place-items-center gap-2">
              <img src={`${B}/png/compozy-symbol-${file}.png`} width={s} height={s} alt="" />
              <span className="font-mono text-[11px] text-(--b-muted)">{s}</span>
            </div>
          ))}
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="grid aspect-square place-items-center rounded-[22%] bg-rail shadow-[inset_0_0_0_1px_var(--color-line-strong)]">
            <img
              src={`${B}/png/compozy-app-icon-512.png`}
              alt="App icon"
              className="w-full rounded-[22%]"
            />
          </div>
          <div className="grid aspect-square place-items-center rounded-[14px] bg-fg">
            <img src={`${B}/svg/compozy-symbol-mono-ink.svg`} alt="" className="w-[62%]" />
          </div>
        </div>
        <p className="m-0 text-[14px] text-(--b-muted)">
          The gradient symbol is a raster master (618 px). For sizes under 24 px and for print in
          one colour, use the vector silhouette.
        </p>
      </div>
    </div>
  );
}

function OsChipAndMisuse() {
  return (
    <>
      <h3 className={`${h3} mt-16`}>The OS chip</h3>
      <div className="mt-4 grid gap-4 md:grid-cols-3">
        <div className={card}>
          <img src={`${B}/svg/compozy-lockup-os-dark.svg`} alt="" className="h-9 w-auto" />
          <p className="m-0 mt-5 text-[15px] text-(--b-muted)">
            Use where the operating-system context matters: the desktop app, install pages, the
            runtime docs.
          </p>
        </div>
        <div className={card}>
          <img src={`${B}/svg/compozy-lockup-dark.svg`} alt="" className="h-9 w-auto" />
          <p className="m-0 mt-5 text-[15px] text-(--b-muted)">
            Everywhere else, drop it. Social, merch, ads, stickers and the site header use the plain
            lockup.
          </p>
        </div>
        <div className={card}>
          <p className="m-0 font-mono text-[13px] text-(--color-brand-cyan)">OS</p>
          <p className="m-0 mt-5 text-[15px] text-(--b-muted)">
            Geist Mono 500 at 0.3 em, signal cyan (#0A6F86 on paper), 1 px line-strong border,
            radius 0.3 em. It is never filled and never stands alone.
          </p>
        </div>
      </div>

      <h3 className={`${h3} mt-16`}>Never</h3>
      <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {misuse.map(([t, style]) => (
          <figure key={t} className="m-0">
            <div className="relative grid aspect-[4/3] place-items-center overflow-hidden rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)] bg-rail">
              <img
                src={`${B}/png/compozy-symbol-256.png`}
                alt=""
                className="w-[38%]"
                style={style}
              />
              <span className="absolute top-3 left-3 font-mono text-[12px] text-destructive">
                ✕
              </span>
            </div>
            <figcaption className="mt-2.5 text-[14px] text-(--b-muted)">{t}</figcaption>
          </figure>
        ))}
      </div>
      <DoDont
        className="mt-4"
        doContent={
          <div>
            <img src={`${B}/svg/compozy-lockup-dark.svg`} alt="" className="h-8 w-auto" />
            <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
              Wordmark in Instrument Sans 500, mixed case, −2% tracking, symbol on the left.
            </p>
          </div>
        }
        dontContent={
          <div>
            <p className="m-0 font-sans text-4xl font-extrabold uppercase italic tracking-wide">
              CompoZy OS
            </p>
            <p className="m-0 mt-4 text-[15px] text-(--b-muted)">
              Retyped wordmark, uppercase, italic, heavy, or with “OS” set in the same face.
            </p>
          </div>
        }
      />
    </>
  );
}

export function Logo() {
  return (
    <Section
      id="logo"
      n="04"
      file="logo.svg"
      title={
        <>
          One symbol, one <em>wordmark</em>
        </>
      }
      bg="ink2"
      status="approved"
      intro={
        <p>
          The lockup is the symbol and “Compozy” in Instrument Sans 500. The OS chip is an optional
          modifier: almost everyone says “Compozy”, so it never carries weight.
        </p>
      }
    >
      <LogoStages />
      <Construction />
      <OsChipAndMisuse />
      <h3 className={`${h3} mt-16`}>Files</h3>
      <ul className="mt-4 grid list-none gap-2 p-0 sm:grid-cols-2 lg:grid-cols-3">
        {files.map(([f, d]) => (
          <li key={f}>
            <a
              href={`${B}/svg/${f}`}
              download
              className="flex items-center justify-between gap-3 rounded-[12px] border border-(--b-line) px-4 py-3 no-underline hover:border-(--b-accent)"
            >
              <span>
                <span className="block font-mono text-[12.5px]">{f}</span>
                <span className="block text-[13px] text-(--b-muted)">{d}</span>
              </span>
              <span className="text-(--b-accent)">↓</span>
            </a>
          </li>
        ))}
      </ul>
    </Section>
  );
}
