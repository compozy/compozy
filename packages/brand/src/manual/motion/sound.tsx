// 09 · 06 Sound: three cues (start, done, needs you), playable via Web Audio, plus mix rules.
import { cn } from "@compozy/ui/utils";
import { useCallback, useEffect, useRef } from "react";

import { btn, btnSecondary, btnSm, card, h3, lab, rules } from "../cls";
import { Section } from "../parts/section";
import { soundRules, sounds } from "./data";

type CueId = (typeof sounds)[number]["id"];

// envelope sketch per cue, 48 bars: start = two short rising tones, done = one long soft decay,
// needs you = two equal taps with a gap
const wave = (id: CueId) =>
  Array.from({ length: 48 }, (_, i) => {
    const t = i / 47;
    const bump = (a: number, b: number, peak = 1) =>
      t < a || t > b
        ? 0
        : Math.sin(((t - a) / (b - a)) * Math.PI) * peak * (1 - ((t - a) / (b - a)) * 0.5);
    const v =
      id === "start"
        ? Math.max(bump(0, 0.42, 0.7), bump(0.38, 0.9, 1))
        : id === "done"
          ? t < 0.04
            ? t / 0.04
            : Math.exp(-(t - 0.04) * 3.2)
          : Math.max(bump(0.02, 0.36), bump(0.62, 0.96));
    return Math.round(v * 100);
  });

const file: Record<CueId, string> = {
  start: "start.wav",
  done: "done.wav",
  needs: "needs-you.wav",
};
const glyphTone: Record<CueId, string> = {
  start: "text-(--color-brand-cyan)",
  done: "text-success",
  needs: "text-warning",
};
const levels: [name: string, value: string, pct: number][] = [
  ["Voice", "-16 to -18 LUFS", 82],
  ["UI cue in video", "voice -3 dB", 70],
  ["Music bed", "-20 to -24 LUFS", 48],
  ["Master", "-14 LUFS · -1 dBTP", 92],
];

/** Synthesised cues: sine tones with a 6 ms attack and exponential release. No files needed. */
function useCuePlayer() {
  const ctxRef = useRef<AudioContext | null>(null);
  useEffect(() => {
    const ctxs = ctxRef;
    return () => {
      void ctxs.current?.close();
      ctxs.current = null;
    };
  }, []);
  return useCallback((id: CueId, [f1, f2]: [number, number]) => {
    ctxRef.current ??= new AudioContext();
    const ctx = ctxRef.current;
    const tone = (f: number, at: number, len: number, gain = 0.12) => {
      const o = ctx.createOscillator();
      const g = ctx.createGain();
      o.type = "sine";
      o.frequency.value = f;
      const t = ctx.currentTime + at;
      g.gain.setValueAtTime(0, t);
      g.gain.linearRampToValueAtTime(gain, t + 0.006);
      g.gain.exponentialRampToValueAtTime(0.0001, t + len);
      o.connect(g).connect(ctx.destination);
      o.start(t);
      o.stop(t + len + 0.02);
    };
    if (id === "start") {
      tone(f1, 0, 0.09);
      tone(f2, 0.08, 0.12);
    } else if (id === "done") {
      tone(f1, 0, 0.32, 0.1);
      tone(f2, 0.03, 0.3, 0.07);
    } else {
      tone(f1, 0, 0.14, 0.14);
      tone(f2, 0.26, 0.14, 0.14);
    }
  }, []);
}

export function Sound() {
  const playCue = useCuePlayer();
  return (
    <Section
      id="sound"
      n="06"
      file="motion/sound/"
      title={
        <>
          Three sounds, all <em>quiet</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Sound marks the same three moments the product cares about: a run starts, a run is done, a
          person is needed. Short sine tones, low in the mix, never a melody. Press a cue to hear
          it; it is synthesised in the page.
        </p>
      }
    >
      <div className="grid gap-4 md:grid-cols-3">
        {sounds.map(s => (
          <div key={s.id} className={card}>
            <div className="flex items-center justify-between gap-3">
              <p className={lab}>cues/{file[s.id]}</p>
              <span className={cn("font-mono text-[18px]", glyphTone[s.id])}>{s.glyph}</span>
            </div>
            <h3 className={cn(h3, "mt-3")}>{s.name}</h3>
            <div className="mt-3 flex h-16 items-center gap-[3px]" aria-hidden="true">
              {wave(s.id).map((v, i) => (
                <span
                  key={i}
                  className="block w-full rounded-full bg-[linear-gradient(45deg,var(--color-brand-cyan),var(--color-brand-volt))]"
                  style={{ height: `${Math.max(4, v * 0.6)}%` }}
                />
              ))}
            </div>
            <button
              type="button"
              onClick={() => playCue(s.id, s.f)}
              className={cn(btn, btnSm, btnSecondary, "mt-4")}
            >
              ▶ Play {s.name.toLowerCase()}
            </button>
            <dl className="m-0 mt-5 grid gap-0 border-t border-(--b-line) text-[13.5px]">
              {[
                ["length", s.len],
                ["pitch", s.pitch],
                ["level", s.level],
                ["when", s.use],
              ].map(([k, v]) => (
                <div
                  key={k}
                  className="grid grid-cols-[70px_minmax(0,1fr)] gap-3 border-b border-(--b-line) py-1.5"
                >
                  <dt className="font-mono text-[12px] text-(--b-muted)">{k}</dt>
                  <dd className="m-0">{v}</dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
        <div className={card}>
          <h3 className={h3}>Mix levels</h3>
          <div className="mt-4 grid gap-4">
            {levels.map(([n, v, p]) => (
              <div key={n}>
                <div className="flex justify-between text-[14px]">
                  <span>{n}</span>
                  <span className="font-mono text-[12.5px] text-(--b-accent)">{v}</span>
                </div>
                <div className="mt-1.5 h-2 rounded-full bg-(--b-card-2)">
                  <span
                    className="block h-full rounded-full bg-[linear-gradient(45deg,var(--color-brand-volt),var(--color-brand-sky),var(--color-brand-cyan))]"
                    style={{ width: `${p}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className={card}>
          <h3 className={h3}>Tone and mix rules</h3>
          <ul className={cn(rules, "mt-3 text-[15px]")}>
            {soundRules.map(r => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
      </div>
    </Section>
  );
}
