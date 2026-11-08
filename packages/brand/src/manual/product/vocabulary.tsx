// 08 Vocabulary: product terms and how to depict each one (icon, glyph, product shot, words).
import { cn } from "@compozy/ui/utils";

import { AgentFace } from "../agent-face";
import type { ProductShotName } from "../art/visuals";
import { cap, h3 } from "../cls";
import { Icon } from "../graphics/icon";
import { UiCrop } from "../graphics/ui-crop";
import { Section } from "../parts/section";
import type { AgentState } from "../roster";
import { vocabulary } from "./data";

const glyphTone: Record<string, string> = {
  "●": "text-(--color-brand-cyan)",
  "◐": "text-(--color-attn)",
  "✓": "text-success",
  "→": "text-(--b-muted)",
};

interface Figure {
  t: string;
  v: ProductShotName;
  wide?: boolean;
  d: string;
  z: number;
  oy?: number;
}
const figures: Figure[] = [
  {
    t: "Control plane",
    v: "desktop",
    wide: true,
    d: "The whole desktop: Always-on faces, projects and sessions, the session window. One place to see and command every harness.",
    z: 1,
  },
  {
    t: "Always-on agent",
    v: "agents",
    d: "A face with its state mark and the last check-in note. It checks in on its own and calls you only when needed.",
    z: 1.5,
    oy: 42,
  },
  {
    t: "Needs you",
    v: "approval",
    d: "The permission dock: the agent asks, you allow once, always or deny.",
    z: 1.7,
    oy: 52,
  },
  {
    t: "Orchestration",
    v: "orchestrate",
    d: "One orchestrator session hands work to child sessions in other harnesses and worktrees, ending in pull requests.",
    z: 1,
  },
  {
    t: "Board and sessions",
    v: "board",
    wide: true,
    d: "Sessions by who has the next step: Needs you, Working, Waiting, In review, Done.",
    z: 1,
    oy: 50,
  },
];
const crew: [state: AgentState, word: string, agent: string][] = [
  ["working", "Working", "checkout-lead"],
  ["needs-you", "Needs you", "docs-steward"],
  ["idle", "Idle", "qa-runner"],
  ["paused", "Paused", "platform-engineer"],
  ["failed", "Failed", "test-writer"],
];

function TermTable() {
  return (
    <div className="mt-4 overflow-x-auto rounded-card border border-(--b-line) bg-(--b-card)">
      <table className="w-full min-w-[920px] border-collapse text-[14px]">
        <thead>
          <tr className="font-mono text-[11px] tracking-[0.08em] text-(--b-muted) uppercase">
            {["Term", "Icon", "Glyph", "Visual", "Say", "Never"].map((h, i) => (
              <th key={h} className={cn("py-3 text-left font-medium", i === 0 ? "px-5" : "px-3")}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {vocabulary.map(v => (
            <tr key={v.term} className="border-t border-(--b-line)">
              <td className="px-5 py-2.5 font-medium">{v.term}</td>
              <td className="px-3 py-2.5">
                <span className="flex items-center gap-2">
                  <Icon name={v.icon} size={20} />
                  <span className="font-mono text-[11px] text-(--b-muted)">{v.icon}</span>
                </span>
              </td>
              <td
                className={cn(
                  "px-3 py-2.5 font-mono",
                  glyphTone[v.glyph ?? ""] ?? "text-(--color-disabled)"
                )}
              >
                {v.glyph ?? "—"}
              </td>
              <td className="px-3 py-2.5 text-[13px] text-(--b-muted)">{v.visual}</td>
              <td className="px-3 py-2.5 text-[13.5px]">{v.say}</td>
              <td className="px-3 py-2.5 text-[13.5px] text-(--b-muted) line-through decoration-(--color-destructive)/60">
                {v.avoid}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Vocabulary() {
  return (
    <Section
      id="vocabulary"
      n="08"
      file="glossary.md"
      title={
        <>
          Every noun has a <em>picture</em>
        </>
      }
      status="proposal"
      intro={
        <p>
          Product terms come from the glossary in the Compozy repo and keep its spelling. This table
          fixes how each term is drawn and said, so docs, UI, social posts and diagrams show the
          same thing the same way.
        </p>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        {figures.map(f => (
          <figure key={f.t} className={cn("m-0 min-w-0", f.wide && "sm:col-span-2")}>
            <div className="relative">
              <UiCrop
                name={f.v}
                ratio={f.wide ? "16/9" : "4/3"}
                scale={f.z}
                oy={f.oy ?? 50}
                flat={f.v !== "orchestrate"}
              />
              <span className="absolute top-3 left-3 font-mono text-[11.5px] text-(--color-muted-foreground)">
                product/{f.v}
              </span>
            </div>
            <figcaption className={cap}>
              <b className="font-medium text-(--b-fg)">{f.t}.</b> {f.d}
            </figcaption>
          </figure>
        ))}
        <figure className="m-0 min-w-0">
          <div className="grid aspect-[4/3] content-center gap-3 rounded-card border border-dashed border-(--b-line) bg-(--b-card) p-[8%]">
            <p className="m-0 mono-label text-[11px] text-(--b-muted)">
              Handoff · no product surface yet
            </p>
            <p className="m-0 font-display text-[22px] leading-[1.2] tracking-[-0.02em]">
              Continue with another agent
            </p>
          </div>
          <figcaption className={cap}>
            <b className="font-medium text-(--b-fg)">Handoff.</b> Continue with another agent: the
            picker across harnesses.
          </figcaption>
        </figure>
        <figure className="m-0 min-w-0 sm:col-span-2">
          <div className="flex flex-wrap items-center justify-between gap-x-8 gap-y-6 rounded-card border border-(--b-line) bg-(--color-rail) px-[4%] py-8">
            {crew.map(([st, w, n]) => (
              <p key={st} className="m-0 flex items-center gap-4 text-[14px]">
                <AgentFace name={n} state={st} size={56} />
                <span>
                  <span className="block font-medium">{w}</span>
                  <span className="block font-mono text-[11.5px] text-(--color-muted-foreground)">
                    {st}
                  </span>
                </span>
              </p>
            ))}
          </div>
          <figcaption className={cap}>
            <b className="font-medium text-(--b-fg)">Every state on an agent.</b> The face with its
            state mark and the word. The product has seven session states (Status colours adds Stuck
            and Later); Needs you is always the amber one.
          </figcaption>
        </figure>
      </div>

      <h3 className={cn(h3, "mt-16")}>Term table</h3>
      <TermTable />
      <p className={cap}>
        Source of truth for words: <code>docs/_memory/glossary.md</code> and <code>COPY.md</code> in
        the Compozy repo. When they change, this table follows. The product is “Compozy” in prose;{" "}
        <code>compozy</code> is the command.
      </p>
    </Section>
  );
}
