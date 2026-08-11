import { Eyebrow } from "@compozy/ui";
import { cn } from "@compozy/ui/lib/utils";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type StackRow = {
  name: string;
  diy: string;
  builtIn: string;
};

const STACK_ROWS: StackRow[] = [
  {
    name: "Loops",
    diy: "Retry scripts, watch loops, and ad-hoc guardrails rewritten per project.",
    builtIn: "Loops are core objects with typed guardrails and durable runs.",
  },
  {
    name: "Triggers and schedules",
    diy: "Cron jobs, webhook endpoints, and the glue that connects them to agents.",
    builtIn: "Automation the daemon owns: cron, webhooks, and event triggers.",
  },
  {
    name: "Memory",
    diy: "Memory files and recall scripts that drift per machine.",
    builtIn: "Scoped, file-backed memory per workspace and agent, inspectable from every surface.",
  },
  {
    name: "Permissions and approvals",
    diy: "Permission wrappers and approval prompts enforced only by convention.",
    builtIn: "Permissions and approvals enforced in the runtime, not in the prompt.",
  },
  {
    name: "Observability",
    diy: "Log scraping and ad-hoc dashboards bolted around each run.",
    builtIn: "Durable events and run history from the records the runtime already writes.",
  },
  {
    name: "Agent integration",
    diy: "Per-provider adapters and startup scripts for every CLI.",
    builtIn: "Runs the agents you already use: Claude Code, OpenClaw, and Hermes among them.",
  },
];

const COLUMNS = "md:grid-cols-[minmax(0,11rem)_minmax(0,1fr)_minmax(0,1fr)]";

export function Comparison() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="The DIY agent stack"
        title="Every piece you would otherwise assemble."
        description="Getting continuous work out of agents means wiring loops, triggers, cron, memory, permissions, approvals, and observability, then maintaining the glue between them. In CompozyOS those are core objects in one runtime."
      />

      <div className="mt-10 overflow-hidden rounded-diagram border border-line bg-canvas-soft">
        <div className={cn("hidden border-b border-line px-5 py-4 md:grid md:gap-4", COLUMNS)}>
          <Eyebrow className="text-subtle">Piece</Eyebrow>
          <Eyebrow className="text-subtle">What you assemble yourself</Eyebrow>
          <Eyebrow className="text-accent">What comes built in</Eyebrow>
        </div>

        {STACK_ROWS.map(row => (
          <article
            key={row.name}
            className={cn(
              "grid gap-3 border-t border-line p-5 first:border-t-0 md:items-baseline md:gap-4",
              COLUMNS
            )}
          >
            <h3 className="text-sm font-semibold text-fg">{row.name}</h3>
            <div>
              <Eyebrow className="text-subtle md:hidden">What you assemble yourself</Eyebrow>
              <p className="text-small-body leading-6 text-muted">{row.diy}</p>
            </div>
            <div>
              <Eyebrow className="text-accent md:hidden">What comes built in</Eyebrow>
              <p className="text-small-body leading-6 text-fg">{row.builtIn}</p>
            </div>
          </article>
        ))}
      </div>
    </SectionFrame>
  );
}
