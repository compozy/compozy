/** S1, a component sheet: the one surface that stays out of the app shell, because it's a catalog. */
import { Check, X } from "lucide-react";
import type { ReactNode } from "react";

import { Eyebrow, cn } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { AGENT_STATE_LABEL, type AgentState } from "../persistent-agents-shared/agent-state";
import { RELEASE, SCOUT, TRIAGE } from "../persistent-agents-shared/fixtures";

export const meta = definePrototype({
  title: "Agent avatar",
  spec: "always-on-agent",
  surface: "S1",
  states: [
    { id: "poses", label: "Generated in each pose" },
    { id: "icon", label: "Icon" },
    { id: "emoji", label: "Emoji" },
    { id: "badge", label: "With and without badge" },
    { id: "needs-you", label: "Needs-you mark" },
    { id: "reduced-motion", label: "Reduced motion" },
    { id: "unknown-appearance", label: "Unknown appearance" },
    { id: "sizes", label: "Sizes 16 to 64" },
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
const SIZES = [16, 20, 24, 32, 40, 48, 64];
const SEEDS = ["orchestrator", "triage", "general", "code-engineer", "reviewer", "docs"];

function Cell({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-w-24 flex-col items-center gap-3 rounded-lg bg-card p-4 shadow-card">
      <div className="grid h-18 place-items-center">{children}</div>
      <span className="text-center text-eyebrow tracking-eyebrow text-muted">{label}</span>
    </div>
  );
}

function Band({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <Eyebrow>{title}</Eyebrow>
      <div className="flex flex-wrap gap-3">{children}</div>
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
            size={56}
            state={state}
            still={still}
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
              size={56}
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
              size={56}
            />
          </Cell>
        ))}
        <Cell label="Provider unknown">
          <AgentAvatar appearance={TRIAGE.appearance} label="Triage Lead" quietIdle size={56} />
        </Cell>
      </Band>
      <Band title="Badge and signal together">
        <Cell label="Working">
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="claude"
            size={56}
            state="working"
          />
        </Cell>
        <Cell label="Failed">
          <AgentAvatar
            appearance={TRIAGE.appearance}
            label="Triage Lead"
            provider="codex"
            size={56}
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
        [24, 32, 56].map(size => (
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

function Unknown() {
  return (
    <Band title="Unknown appearance falls back to the slug">
      <Cell label="version: 9 in AGENT.md">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "triage" }}
          label="Triage Lead"
          provider="claude"
          quietIdle
          size={56}
        />
      </Cell>
      <Cell label="No saved appearance">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "general" }}
          label="General"
          provider="claude"
          quietIdle
          size={56}
        />
      </Cell>
      <Cell label="Deleted agent">
        <AgentAvatar
          appearance={{ avatar: "generated", seed: "release-captain" }}
          label="Release Captain"
          size={56}
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
      return <Poses />;
  }
}

export default function AgentAvatarDraft({ state }: PrototypeProps) {
  return (
    <main className={cn("flex min-h-dvh flex-col gap-8 bg-canvas p-8 text-fg")}>
      <Content state={state} />
    </main>
  );
}
