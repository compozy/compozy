// 03 Status colours: session states → colour + glyph + word, AA verified in both themes.
import { cn } from "@compozy/ui/utils";

import { AgentFace } from "../agent-face";
import { cap, card, h3, lab, rules } from "../cls";
import { grade, ratio } from "../contrast";
import { Section } from "../parts/section";
import type { AgentState } from "../roster";
import { macos, sessions, statusChecks, states } from "./data";
import { SessionRow } from "./session-row";

const minRatio = Math.min(
  ...states.flatMap(s =>
    statusChecks.map(([, bg, t]) => ratio(t === "dark" ? s.dark : s.light, bg))
  )
);

const faces: [state: AgentState, word: string, agent: string, note: string][] = [
  ["working", "Working", "checkout-lead", "Thinking, cyan dot"],
  ["needs-you", "Needs you", "docs-steward", "Surprised, amber ▲"],
  ["idle", "Idle", "qa-runner", "Calm, grey dot"],
  ["paused", "Paused", "platform-engineer", "Sleepy, grey dot"],
  ["failed", "Failed", "test-writer", "Sad, red dot"],
];

const TRAY = ["idle", "working", "attention"] as const;

function StateTable() {
  return (
    <div className="overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
      <table className="w-full min-w-[900px] border-collapse text-[14px]">
        <thead>
          <tr className="font-mono text-[11px] tracking-[0.08em] text-(--b-muted) uppercase">
            <th className="px-5 py-3 text-left font-medium">State</th>
            <th className="px-3 py-3 text-left font-medium">Meaning</th>
            {statusChecks.map(([n, , t]) => (
              <th key={n} className="px-3 py-3 text-right font-medium">
                {t} · {n}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {states.map(s => (
            <tr key={s.key} className="border-t border-(--b-line) align-top">
              <td className="px-5 py-3.5">
                <span className="flex items-center gap-3">
                  <span
                    className="grid size-9 place-items-center rounded-[8px] bg-(--color-rail) font-mono text-[16px]"
                    style={{ color: s.dark }}
                  >
                    {s.glyph}
                  </span>
                  <span>
                    <span className="block font-medium">{s.word}</span>
                    {s.alt && (
                      <span className="block font-mono text-[11px] text-(--b-muted)">
                        also “{s.alt}”
                      </span>
                    )}
                    <span className="block font-mono text-[11px] text-(--b-muted)">
                      {s.dark} · {s.light}
                    </span>
                  </span>
                </span>
              </td>
              <td className="max-w-[360px] px-3 py-3.5 text-[13.5px] text-(--b-muted)">
                {s.meaning}
              </td>
              {statusChecks.map(([n, bg, t]) => {
                const c = t === "dark" ? s.dark : s.light;
                const r = ratio(c, bg);
                const g = grade(r);
                return (
                  <td key={n} className="px-3 py-3.5 text-right font-mono text-[12px]">
                    <span
                      className="mr-1.5 inline-block rounded-[5px] px-1.5 shadow-[inset_0_0_0_1px_var(--b-line)]"
                      style={{ background: bg, color: c }}
                    >
                      {s.glyph} {s.word.split(" ")[0]}
                    </span>
                    {r.toFixed(2)}{" "}
                    <span
                      className={
                        g === "fail" || g === "AA large" ? "text-destructive" : "text-(--b-accent)"
                      }
                    >
                      {g}
                    </span>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function WhereStateShows() {
  return (
    <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div className={card}>
        <p className={lab}>Tab</p>
        <div className="mt-4 flex gap-1 rounded-[10px] bg-(--color-rail) p-1 text-[13px]">
          <span className="flex items-center gap-1.5 rounded-[7px] bg-(--color-elevated) px-2.5 py-1.5">
            <span className="text-(--color-attn)">◐</span>migrate-db
          </span>
          <span className="flex items-center gap-1.5 px-2.5 py-1.5 text-(--color-muted-foreground)">
            <span className="text-(--color-brand-cyan) motion-safe:animate-pulse">●</span>review-pr
          </span>
        </div>
        <p className={cap}>
          Face or glyph before the title; the word goes in the tooltip and the accessible name.
        </p>
      </div>
      <div className={card}>
        <p className={lab}>Dock badge</p>
        <div className="relative mt-4 w-fit">
          <img
            src="/brand/png/compozy-app-icon-128h.png"
            alt=""
            className="size-16 rounded-[22%]"
          />
          <span
            className="absolute -top-1.5 -right-2 grid h-5 min-w-5 place-items-center rounded-full px-1.5 text-[11px] font-semibold text-(--color-fg-strong)"
            style={{ background: macos.badge }}
          >
            2
          </span>
        </div>
        <p className={cap}>Counts Needs you only. Working never badges: it is expected.</p>
      </div>
      <div className={card}>
        <p className={lab}>Tray</p>
        <div
          className="mt-4 flex items-center gap-4 rounded-[10px] px-3 py-2"
          style={{ background: macos.darkMenu }}
        >
          {TRAY.map(t => (
            <img
              key={t}
              src={`/brand/product/tray/compozy-tray-${t}Template@2x.png`}
              alt={t}
              className="size-4 invert"
            />
          ))}
        </div>
        <p className={cap}>
          Idle · working · needs you. Template images: macOS tints them for light and dark menu
          bars.
        </p>
      </div>
      <div className={card}>
        <p className={lab}>Notification</p>
        <div className="mt-4 rounded-[12px] border border-line-strong bg-(--color-elevated) p-3 text-[13px]">
          <p className="m-0 flex items-center gap-2 font-medium">
            <span className="text-(--color-attn)">◐</span> migrate-db needs you
          </p>
          <p className="m-0 mt-1 text-(--color-muted-foreground)">
            Approve: run migration on staging
          </p>
        </div>
        <p className={cap}>
          Title = session + state word. Body = the exact question. Never “Action required”.
        </p>
      </div>
    </div>
  );
}

export function StatusColours() {
  return (
    <Section
      id="status"
      n="03"
      file="status.ts"
      title={
        <>
          Seven states, each with a <em>word</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          An agent or session is always in exactly one state. Each state has a glyph, a word and a
          colour, and the three always travel together: colour helps scanning, the word carries the
          meaning, the glyph survives where colour does not. On an agent the face adds a fourth
          carrier: its expression and state mark.
        </p>
      }
    >
      <div className="mb-6 rounded-card border border-(--b-line) bg-(--color-rail) p-6">
        <p className={lab}>Faces carry the state</p>
        <div className="mt-5 grid grid-cols-2 gap-6 sm:grid-cols-5">
          {faces.map(([st, w, n, d]) => (
            <div key={st} className="grid justify-items-start gap-3">
              <AgentFace name={n} state={st} size={64} />
              <span>
                <span className="block text-[14px] font-medium">{w}</span>
                <span className="block text-[12.5px] text-(--color-muted-foreground)">{d}</span>
              </span>
            </div>
          ))}
        </div>
      </div>
      <StateTable />
      <p className={cap}>
        Lowest ratio across all 28 pairs: {minRatio.toFixed(2)}:1. Every state word passes AA as 12
        px text in both themes. Light success is #0A7650: the foundation token #0F8A5F measures
        4.05:1 on paper and is replaced for status text.
      </p>

      <div className="mt-12 grid gap-4 lg:grid-cols-2">
        <div className="rounded-card border border-line bg-(--color-canvas) p-4">
          <p className="m-0 mb-3 px-2 mono-label text-[11px] text-(--color-muted-foreground)">
            Dark · session list
          </p>
          <div className="grid gap-0.5">
            {sessions.map((s, i) => (
              <SessionRow key={s.name} {...s} selected={i === 1} />
            ))}
          </div>
        </div>
        <div className="band-paper rounded-card border border-(--b-line) bg-(--color-fg-strong) p-4">
          <p className="m-0 mb-3 px-2 mono-label text-[11px] text-(--b-muted)">
            Light · session list
          </p>
          <div className="grid gap-0.5">
            {sessions.map((s, i) => (
              <SessionRow key={s.name} {...s} theme="light" selected={i === 1} />
            ))}
          </div>
        </div>
      </div>

      <h3 className={cn(h3, "mt-16")}>Where state shows up</h3>
      <WhereStateShows />
      <ul className={cn(rules, "mt-10 max-w-[76ch] text-[15.5px]")}>
        <li>
          <strong>Sort order</strong> is Needs you, Stuck, Failed, Working, Idle, Done; Later sits
          in its own view. The exception is always at the top.
        </li>
        <li>
          <strong>Words are fixed.</strong> Never “Running”, “Completed”, “Paused” or “Error”: the
          same words as the CLI, docs and API.
        </li>
        <li>
          <strong>Working is the only motion</strong>: a 2.4 s opacity pulse on the dot, off under
          reduced motion.
        </li>
        <li>
          <strong>Failed and Stuck share red</strong> and differ by glyph: ✕ ended, ■ still running
          without progress.
        </li>
      </ul>
    </Section>
  );
}
