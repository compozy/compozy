import { Eyebrow } from "@agh/ui";
import { cn } from "@agh/ui/lib/utils";
import { Check, Minus } from "lucide-react";
import { SUPPORTED_AGENT_COUNT } from "./provider-data";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type Approach = {
  approach: string;
  focus: string;
  agentModel: string;
  coordination: string;
  deployment: string;
  agents: string;
  /** Whether this approach exposes an implemented cross-runtime protocol. */
  crossRuntime: boolean;
  highlight?: boolean;
};

const APPROACHES: Approach[] = [
  {
    approach: "Letta",
    focus: "Memory-first stateful agents",
    agentModel: "Letta agents in cloud or self-host",
    coordination: "None, single agent",
    deployment: "Cloud-hosted or self-host",
    agents: "1 (managed)",
    crossRuntime: false,
  },
  {
    approach: "LangGraph / CrewAI",
    focus: "Multi-agent orchestration framework",
    agentModel: "Agents you author in Python",
    coordination: "In-process graph or crew",
    deployment: "Library you embed",
    agents: "your code",
    crossRuntime: false,
  },
  {
    approach: "OpenAI Assistants / Devin",
    focus: "Hosted agent platform",
    agentModel: "Managed agents behind an API",
    coordination: "Centralized routing",
    deployment: "Cloud-only",
    agents: "managed",
    crossRuntime: false,
  },
  {
    approach: "AGH",
    focus: "Run + connect real agent CLIs",
    agentModel: "Your existing ACP agents",
    coordination: "agh-network/v0, implemented",
    deployment: "Local-first, single binary",
    agents: `${SUPPORTED_AGENT_COUNT} ACP drivers`,
    crossRuntime: true,
    highlight: true,
  },
];

const DIMENSIONS = [
  { key: "focus" as const, label: "Primary focus" },
  { key: "agentModel" as const, label: "Agent model" },
  { key: "agents" as const, label: "Agent support" },
  { key: "coordination" as const, label: "Coordination" },
  { key: "deployment" as const, label: "Deployment" },
];

export function Comparison() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="Positioning"
        title="Other tools stop at the runtime boundary."
        description="AGH is the only approach here with an implemented cross-runtime protocol. The rest centralize coordination or skip it entirely."
      />

      <div className="mt-10 overflow-hidden rounded-diagram border border-line bg-canvas-soft">
        {/* Header row */}
        <div className="hidden border-b border-line px-5 py-4 md:grid md:grid-cols-[160px_repeat(5,minmax(0,1fr))_60px] md:gap-4">
          <Eyebrow className="text-subtle">Approach</Eyebrow>
          {DIMENSIONS.map(d => (
            <Eyebrow key={d.key} className="text-subtle">
              {d.label}
            </Eyebrow>
          ))}
          <Eyebrow className="text-right text-subtle">Cross-runtime</Eyebrow>
        </div>

        {APPROACHES.map(row => (
          <div
            key={row.approach}
            className={cn(
              "grid gap-3 border-t border-line p-5 first:border-t-0 md:grid-cols-[160px_repeat(5,minmax(0,1fr))_60px] md:items-center md:gap-4",
              row.highlight && "border-l-4 border-l-accent bg-accent-tint/40"
            )}
          >
            <div>
              <h3
                className={cn("text-sm font-semibold", row.highlight ? "text-accent" : "text-fg")}
              >
                {row.approach}
              </h3>
            </div>
            {DIMENSIONS.map(d => (
              <div key={d.key}>
                <Eyebrow className="text-subtle md:hidden">{d.label}</Eyebrow>
                <p
                  className={cn(
                    "text-small-body leading-6",
                    row.highlight && d.key === "coordination" ? "font-medium text-fg" : "text-muted"
                  )}
                >
                  {row[d.key]}
                </p>
              </div>
            ))}
            <div className="flex md:justify-end">
              <span
                aria-label={row.crossRuntime ? "Cross-runtime: yes" : "Cross-runtime: no"}
                className={cn(
                  "inline-flex size-6 items-center justify-center rounded-mono-badge",
                  row.crossRuntime ? "bg-success-tint text-success" : "bg-elevated text-subtle"
                )}
              >
                {row.crossRuntime ? (
                  <Check className="size-3" strokeWidth={3} />
                ) : (
                  <Minus className="size-3" strokeWidth={3} />
                )}
              </span>
            </div>
          </div>
        ))}
      </div>
    </SectionFrame>
  );
}
