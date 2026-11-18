// 09 · 05 Video: sting, end card, product demo; story structure, storyboard, specs and captions.
import { cn } from "@compozy/ui/utils";

import { Visual } from "../art/visual";
import { card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { captionRules, storyboard, videoFormats } from "./data";

const beats: [beat: string, time: string, text: string][] = [
  ["Hook", "0–3 s", "The reader’s problem in one line, on screen in the first second."],
  [
    "Mechanism",
    "3–20 s",
    "The screen where it happens: the desktop, the board, the approval in the session. The real app.",
  ],
  ["Proof", "20 s →", "The result: a finished run, a named outcome, a diff."],
  ["End", "last 2.4 s", "One line, the desktop UI, the URL, the lockup."],
];
const SPEC_COLUMNS = ["Channel", "Size", "Length", "File", "Captions"] as const;
const specRows = [
  ["X", "1920×1080 · 1080×1350", "≤ 2:20", "H.264 .mp4", "burned in"],
  ["LinkedIn", "1080×1350 · 1920×1080", "30–90 s", "H.264 .mp4", "burned in + .srt"],
  ["YouTube", "1920×1080 · 3840×2160", "any", "H.264 / ProRes", ".srt sidecar"],
  ["Shorts / Reels", "1080×1920", "≤ 60 s", "H.264 .mp4", "burned in, above 18%"],
  ["Site hero", "1920×1080, 1080×1350 mobile", "6–12 s loop", ".mp4 + .webm", "none, no audio"],
  ["Stage screen", "3840×2160", "sting / loops", "ProRes 422 HQ", "none"],
];
/** beat colours of the format timeline: volt, sky, teal, then the resting hold */
const BEAT_FILL = [
  "bg-(--color-brand-volt)",
  "bg-(--color-brand-sky)",
  "bg-(--color-brand-teal)",
  "bg-disabled",
];
const HATCH =
  "bg-[repeating-linear-gradient(45deg,color-mix(in_oklab,var(--color-destructive)_28%,transparent)_0_6px,transparent_6px_12px)]";
const CAPTION_BOX =
  "rounded-[6px] bg-[color-mix(in_oklab,var(--color-rail)_70%,transparent)] px-[3cqi] py-[1.4cqi]";
const th = "mono-label text-[11.5px] tracking-[0.08em] text-(--b-muted)";

function Formats() {
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      {videoFormats.map(v => (
        <div key={v.id} className={card}>
          <p className={lab}>{v.size}</p>
          <div className="mt-3 flex items-baseline justify-between gap-3">
            <h3 className={cn(h3, "mb-0")}>{v.name}</h3>
            <span className="font-mono text-[15px] text-(--b-accent)">{v.dur}</span>
          </div>
          <p className="m-0 mt-2 text-[14.5px] text-(--b-muted)">{v.use}</p>
          <ol className="m-0 mt-4 grid list-none gap-0 border-t border-(--b-line) p-0 text-[13.5px]">
            {v.beats.map(([t, w]) => (
              <li
                key={t}
                className="grid grid-cols-[86px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-1.5"
              >
                <span className="font-mono text-(--b-accent)">{t}</span>
                <span>{w}</span>
              </li>
            ))}
          </ol>
          {v.id !== "demo" && (
            <div
              className="mt-4 flex h-2 overflow-hidden rounded-full bg-(--b-card-2)"
              title={`${parseFloat(v.dur)} s`}
            >
              {v.beats.map(([t], i) => (
                <span
                  key={t}
                  className={cn("block flex-1 border-r border-(--b-card)", BEAT_FILL[i])}
                />
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function Storyboard() {
  return (
    <>
      <h3 className={cn(h3, "mt-14")}>Storyboard · product demo, 30 s</h3>
      <p className="m-0 max-w-[62ch] text-[15px] text-(--b-muted)">
        Every frame of a storyboard is one visual plus the line on screen: the pain is Atmosphere,
        every product beat is our UI (Live). Draw it before recording: if the beats do not read as
        five frames, the video will not read either.
      </p>
      <div className="mt-5 grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-6">
        {storyboard.map(f => (
          <figure key={f.t} className="m-0 min-w-0">
            <div className="relative aspect-video overflow-hidden rounded-[12px] border border-(--b-line)">
              <Visual
                name={f.visual}
                x={f.visual === "terrain" || f.visual === "horizon" ? f.x : 0.5}
                y={0.5}
                sizes="(min-width: 1280px) 16vw, 50vw"
              />
            </div>
            <figcaption className="mt-2 flex justify-between mono-label text-[11px] tracking-[0.08em] text-(--b-muted)">
              <span className="text-(--b-accent)">{f.t}</span>
              <span>{f.beat}</span>
            </figcaption>
            <p className="m-0 mt-1.5 text-[13px] leading-[1.35]">{f.line}</p>
          </figure>
        ))}
      </div>
    </>
  );
}

function Specs() {
  return (
    <>
      <h3 className={cn(h3, "mt-14")}>Specs per format</h3>
      <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
        <table className="w-full min-w-[760px] border-collapse text-[14px]">
          <thead>
            <tr className="border-b border-(--b-line) text-left">
              {SPEC_COLUMNS.map(h => (
                <th key={h} className={cn(th, "px-5 py-3 font-medium")}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {specRows.map(r => (
              <tr key={r[0]} className="border-b border-(--b-line) last:border-0">
                {SPEC_COLUMNS.map((column, i) => (
                  <td
                    key={column}
                    className={cn(
                      "px-5 py-3",
                      i === 0
                        ? "font-medium"
                        : i === 1
                          ? "font-mono text-[13px]"
                          : "text-(--b-muted)"
                    )}
                  >
                    {r[i]}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function Captions() {
  return (
    <div className="mt-14 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
      <div className="mx-auto w-full max-w-[300px]">
        <div
          className="relative aspect-[9/16] overflow-hidden rounded-[18px] border border-(--b-line)"
          style={{ containerType: "inline-size" }}
        >
          <Visual name="converge" x={0.6} y={0.3} sizes="300px" />
          <div className={cn("absolute inset-x-0 bottom-0 h-[18%]", HATCH)} />
          <div className="absolute inset-x-[7%] bottom-[21%] grid justify-items-center gap-[1.5cqi] text-center">
            <span
              className={cn(CAPTION_BOX, "font-semibold leading-[1.25]")}
              style={{ fontSize: "7.8cqi" }}
            >
              Start the daemon once
            </span>
            <span
              className={cn(CAPTION_BOX, "font-mono text-(--color-brand-cyan)")}
              style={{ fontSize: "6.6cqi" }}
            >
              compozy daemon start
            </span>
          </div>
        </div>
        <p className="m-0 mt-2.5 text-sm text-(--b-muted)">
          9:16 caption specimen. Hatched band: platform UI, 18% of height.
        </p>
      </div>
      <div className={card}>
        <h3 className={h3}>Captions</h3>
        <ul className={cn(rules, "mt-3 text-[15px]")}>
          {captionRules.map(r => (
            <li key={r}>{r}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export function Video() {
  return (
    <Section
      id="video"
      n="05"
      file="motion/video.md"
      title={
        <>
          Hook, mechanism, proof, <em>end</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Every video is the same four beats at a different length. Open on the reader’s problem
          (five terminals), show the desktop where the work is managed, prove it with a result on
          screen, close on the end card. The sting brackets the piece.
        </p>
      }
    >
      <Formats />
      <div className="mt-10 grid gap-px overflow-hidden rounded-card border border-(--b-line) bg-(--b-line) md:grid-cols-4">
        {beats.map(([b, t, d], i) => (
          <div key={b} className="bg-(--b-card) p-6">
            <p className="m-0 font-mono text-[12px] text-(--b-accent)">
              0{i + 1} · {t}
            </p>
            <p className="m-0 mt-3 font-display text-[24px] tracking-[-0.02em]">{b}</p>
            <p className="m-0 mt-2 text-[14px] text-(--b-muted)">{d}</p>
          </div>
        ))}
      </div>
      <Storyboard />
      <Specs />
      <Captions />
    </Section>
  );
}
