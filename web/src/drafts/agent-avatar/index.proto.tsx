/** S1, a component sheet: the one surface that stays out of the app shell, because it's a catalog. */
import { Check, X } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Eyebrow, cn } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { AGENT_STATE_LABEL, type AgentState } from "../stays-on-agents-shared/agent-state";
import { RELEASE, SCOUT, TRIAGE } from "../stays-on-agents-shared/fixtures";

export const meta = definePrototype({
  title: "Agent avatar",
  spec: "always-on-agent",
  surface: "S1",
  states: [
    { id: "poses", label: "Generated in each pose" },
    { id: "icon", label: "Icon" },
    { id: "emoji", label: "Emoji" },
    { id: "badge", label: "With and without badge" },
    { id: "needs-you", label: "Needs-you triangle" },
    { id: "tones", label: "Tones per theme" },
    { id: "color", label: "Color sets the tone" },
    { id: "reduced-motion", label: "Reduced motion" },
    { id: "unknown-appearance", label: "Unknown appearance" },
    { id: "sizes", label: "Sizes 16 to 92" },
    { id: "legibility-16", label: "16 px legibility" },
  ],
});

const STATES: readonly AgentState[] = [
  "working",
  "needs-you",
  "failed",
  "paused",
  "idle",
  "starting",
  "unknown",
];
const SIZES = [16, 20, 24, 32, 40, 48, 64, 92];
const SEEDS = ["orchestrator", "triage", "general", "code-engineer", "reviewer", "docs"];

function Cell({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col items-center gap-4 rounded-xl bg-card px-4 py-6 shadow-card">
      <div className="grid h-28 place-items-center">{children}</div>
      <span className="text-center text-small-body text-fg-2">{label}</span>
    </div>
  );
}

function Band({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <Eyebrow>{title}</Eyebrow>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(10.5rem,1fr))] gap-3">{children}</div>
    </section>
  );
}

const TEAM: readonly { seed: string; state: AgentState }[] = [
  { seed: "orchestrator", state: "working" },
  { seed: "triage", state: "needs-you" },
  { seed: "reviewer", state: "idle" },
  { seed: "code-engineer", state: "working" },
  { seed: "docs", state: "paused" },
  { seed: "qa", state: "failed" },
  { seed: "general", state: "idle" },
  { seed: "researcher", state: "idle" },
];

/** A whole team at once: the shapes stay distinct and the faces say who needs a look. */
function Team() {
  return (
    <section className="flex flex-wrap items-center justify-between gap-6 rounded-xl bg-card px-10 py-8 shadow-card">
      {TEAM.map(member => (
        <AgentAvatar
          appearance={{ avatar: "generated", seed: member.seed }}
          key={member.seed}
          label={member.seed}
          provider="claude"
          quietIdle
          size={112}
          state={member.state}
          surfaceClassName="ring-card"
        />
      ))}
    </section>
  );
}

function Poses({ still = false }: { still?: boolean }) {
  return (
    <Band title={still ? "Reduced motion" : "Pose per state"}>
      {STATES.map(state => (
        <Cell key={state} label={AGENT_STATE_LABEL[state]}>
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="claude"
            size={92}
            state={state}
            still={still}
            surfaceClassName="ring-card"
          />
        </Cell>
      ))}
    </Band>
  );
}

function Symbols({ agent }: { agent: typeof RELEASE }) {
  return (
    <>
      <Band title="Keeps its symbol in every state">
        {STATES.map(state => (
          <Cell key={state} label={AGENT_STATE_LABEL[state]}>
            <AgentAvatar
              appearance={agent.appearance}
              label={agent.label}
              provider={agent.provider}
              size={92}
              state={state}
            />
          </Cell>
        ))}
      </Band>
      <SizeBand agent={agent} />
    </>
  );
}

function SizeBand({ agent }: { agent: typeof RELEASE }) {
  return (
    <Band title="Sizes">
      {SIZES.map(size => (
        <Cell key={size} label={`${size} px`}>
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            provider={agent.provider}
            size={size}
            state="idle"
            quietIdle
          />
        </Cell>
      ))}
    </Band>
  );
}

function Badges() {
  return (
    <>
      <Band title="Provider badge">
        {["claude", "codex", "gemini"].map(provider => (
          <Cell key={provider} label={provider}>
            <AgentAvatar
              appearance={TRIAGE.appearance}
              label="Triage Lead"
              provider={provider}
              quietIdle
              size={92}
            />
          </Cell>
        ))}
        <Cell label="Provider unknown">
          <AgentAvatar appearance={TRIAGE.appearance} label="Triage Lead" quietIdle size={92} />
        </Cell>
      </Band>
      <Band title="Badge and signal together">
        <Cell label="Working">
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="claude"
            size={92}
            state="working"
          />
        </Cell>
        <Cell label="Failed">
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="codex"
            size={92}
            state="failed"
          />
        </Cell>
      </Band>
    </>
  );
}

function Ring() {
  return (
    <Band title="Needs you">
      {[TRIAGE, RELEASE, SCOUT].map(agent =>
        [32, 56, 92].map(size => (
          <Cell key={`${agent.name}-${size}`} label={`${agent.label} · ${size}`}>
            <AgentAvatar
              appearance={agent.appearance}
              label={agent.label}
              provider={agent.provider}
              size={size}
              state="needs-you"
            />
          </Cell>
        ))
      )}
    </Band>
  );
}

const TONE_SEEDS = [
  "general",
  "orchestrator",
  "triage",
  "code-engineer",
  "code_implementer",
  "release-captain",
  "reviewer",
  "docs",
  "qa",
  "researcher",
  "planner",
  "designer",
  "ink-test",
  "migration-pilot",
];

const SURFACES = [
  { label: "Canvas", className: "bg-canvas", ring: "ring-canvas" },
  { label: "Card", className: "bg-card", ring: "ring-card" },
  { label: "Sidebar", className: "bg-sunken", ring: "ring-sunken" },
];

function Tones() {
  return (
    <>
      {SURFACES.map(surface => (
        <Band key={surface.label} title={surface.label}>
          <div className={cn("flex flex-wrap gap-4 rounded-lg p-4 shadow-card", surface.className)}>
            {TONE_SEEDS.map(seed => (
              <AgentAvatar
                appearance={{ avatar: "generated", seed }}
                key={seed}
                label={seed}
                provider="claude"
                quietIdle
                size={40}
                surfaceClassName={surface.ring}
              />
            ))}
          </div>
        </Band>
      ))}
    </>
  );
}

const COLORS = ["#4ea7fc", "#2aa8a8", "#4cb782", "#c26ad6", "#e56aa2", "#e8b04a", "#b58e5f"];

function Colors() {
  return (
    <>
      {["triage", "orchestrator", "reviewer"].map(seed => (
        <Band key={seed} title={seed}>
          {COLORS.map(color => (
            <Cell key={color} label={color}>
              <AgentAvatar
                appearance={{ avatar: "generated", seed, color }}
                label={seed}
                quietIdle
                size={92}
              />
            </Cell>
          ))}
        </Band>
      ))}
    </>
  );
}

function Unknown() {
  return (
    <Band title="Unknown appearance falls back to the slug">
      <Cell label="version: 9 in AGENT.md">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "triage" }}
          label="Triage Lead"
          provider="claude"
          quietIdle
          size={92}
        />
      </Cell>
      <Cell label="No saved appearance">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "general" }}
          label="General"
          provider="claude"
          quietIdle
          size={92}
        />
      </Cell>
      <Cell label="Deleted agent">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "release-captain" }}
          label="Release Captain"
          size={92}
          state="unknown"
        />
      </Cell>
    </Band>
  );
}

function Legibility() {
  return (
    <>
      <Band title="Distinct at 16 px">
        {SEEDS.map(seed => (
          <Cell key={seed} label={seed}>
            <AgentAvatar
              appearance={{ avatar: "generated", seed }}
              label={seed}
              quietIdle
              size={16}
            />
          </Cell>
        ))}
      </Band>
      <Band title="State at 16 px">
        {STATES.map(state => (
          <Cell key={state} label={AGENT_STATE_LABEL[state]}>
            <AgentAvatar
              appearance={TRIAGE.appearance}
              label="Triage Lead"
              provider="claude"
              size={16}
              state={state}
            />
          </Cell>
        ))}
      </Band>
      <Band title="Badge fits from 24 px">
        <Cell
          label={
            <span className="inline-flex items-center gap-1">
              <X aria-hidden="true" className="size-3" /> 20 px
            </span>
          }
        >
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="claude"
            quietIdle
            size={20}
          />
        </Cell>
        <Cell
          label={
            <span className="inline-flex items-center gap-1">
              <Check aria-hidden="true" className="size-3" /> 24 px
            </span>
          }
        >
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="claude"
            quietIdle
            size={24}
          />
        </Cell>
      </Band>
    </>
  );
}

function Content({ state }: { state: string }) {
  switch (state) {
    case "icon":
      return <Symbols agent={RELEASE} />;
    case "emoji":
      return <Symbols agent={SCOUT} />;
    case "badge":
      return <Badges />;
    case "needs-you":
      return <Ring />;
    case "tones":
      return <Tones />;
    case "color":
      return <Colors />;
    case "reduced-motion":
      return <Poses still />;
    case "unknown-appearance":
      return <Unknown />;
    case "sizes":
      return (
        <>
          <SizeBand agent={TRIAGE} />
          <SizeBand agent={RELEASE} />
          <SizeBand agent={SCOUT} />
        </>
      );
    case "legibility-16":
      return <Legibility />;
    default:
      return (
        <>
          <Team />
          <Poses />
        </>
      );
  }
}

export default function AgentAvatarDraft({ state }: PrototypeProps) {
  return (
    <main className={cn("flex min-h-dvh flex-col gap-8 bg-canvas p-8 text-fg")}>
      <Content state={state} />
    </main>
  );
}
