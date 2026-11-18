// 03 Diagrams: the technical wireframe language. Two reference diagrams drawn in it.
import { cn } from "@compozy/ui/utils";

import { cap, h3 } from "../cls";
import { DoDont } from "../parts/do-dont";
import { Section } from "../parts/section";
import { diagramRules, diagramSpec, offBrand } from "./data";
import { DelegationDiagram, LoopDiagram } from "./diagram-art";
import { SpecCard } from "../parts/spec-card";
import { RuleCard } from "../parts/rule-card";

const frame =
  "overflow-hidden rounded-card border border-(--b-line) bg-rail p-[clamp(12px,3vw,32px)]";
const muted = "m-0 mt-4 text-[15px] text-(--b-muted)";

function WireframeDoDont() {
  return (
    <DoDont
      className="mt-4"
      doContent={
        <>
          <svg
            viewBox="0 0 480 160"
            className="block w-full rounded-[12px] bg-rail"
            role="img"
            aria-label="Two panels connected by a one pixel line"
          >
            <rect
              x="30"
              y="50"
              width="160"
              height="60"
              rx="10"
              className="fill-canvas stroke-line-strong"
            />
            <text x="48" y="86" className="fill-fg font-mono" fontSize="13">
              trigger · cron
            </text>
            <path d="M190 80H284" className="fill-none stroke-disabled" />
            <path d="M278 76l6 4-6 4" className="fill-none stroke-disabled" />
            <rect
              x="290"
              y="50"
              width="160"
              height="60"
              rx="10"
              className="fill-canvas stroke-brand-cyan"
              strokeWidth="1.5"
            />
            <text x="308" y="86" className="fill-fg font-mono" fontSize="13">
              session · codex
            </text>
          </svg>
          <p className={muted}>Two real things, one line, one highlight.</p>
        </>
      }
      dontContent={
        <>
          <svg
            viewBox="0 0 480 160"
            className="block w-full rounded-[12px] bg-rail"
            role="img"
            aria-label="Counter-example: glowing robot cloud with curved coloured arrows"
          >
            <circle cx="90" cy="80" r="46" fill={offBrand.violet} opacity=".7" />
            <text x="90" y="88" textAnchor="middle" fontSize="28">
              🤖
            </text>
            <path
              d="M140 60C220 0 260 150 340 80"
              stroke={offBrand.orange}
              strokeWidth="4"
              fill="none"
            />
            <path
              d="M140 100C240 160 280 20 340 100"
              stroke={offBrand.green}
              strokeWidth="4"
              fill="none"
              strokeDasharray="8 6"
            />
            <ellipse cx="400" cy="80" rx="56" ry="40" fill={offBrand.pink} opacity=".7" />
            <text
              x="400"
              y="86"
              textAnchor="middle"
              className="fill-fg-strong"
              fontSize="14"
              fontWeight="700"
            >
              AI MAGIC
            </text>
          </svg>
          <p className={muted}>
            Mascots, clouds, curved rainbow arrows, words that name no real thing.
          </p>
        </>
      }
    />
  );
}

export function Diagrams() {
  return (
    <Section
      id="diagrams"
      n="03"
      file="diagrams.svg"
      title={
        <>
          Draw the <em>agents</em>, not boxes
        </>
      }
      status="proposal"
      intro={
        <p>
          Diagrams are wireframes of the control plane: panels for real things, 1 px lines for what
          moves between them, mono labels with real identifiers. Agents appear as their face with
          the harness named; a bare box is for folders and files. One gradient highlight marks the
          point of the drawing.
        </p>
      }
    >
      <figure className="m-0">
        <div className={frame}>
          <DelegationDiagram />
        </div>
        <figcaption className={cap}>
          Orchestrator → child sessions → worktrees, across harnesses. Each agent is its face with
          its harness named; the highlight is the orchestrator, because the diagram is about who
          delegates.
        </figcaption>
      </figure>

      <figure className="m-0 mt-6">
        <div className={frame}>
          <LoopDiagram />
        </div>
        <figcaption className={cap}>
          Loop: goal → act → verify → stop. The highlight is verify and its return path: the loop
          stops on a check, not on a feeling.
        </figcaption>
      </figure>

      <div className="mt-10 grid gap-4 lg:grid-cols-2">
        <SpecCard title="Spec" rows={diagramSpec} listClassName="mt-3" />
        <RuleCard title="Rules">
          {diagramRules.map(r => (
            <li key={r}>{r}</li>
          ))}
        </RuleCard>
      </div>

      <h3 className={cn(h3, "mt-16")}>Wireframe, not illustration</h3>
      <WireframeDoDont />
    </Section>
  );
}
