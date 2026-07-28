import { Eyebrow } from "@compozy/ui";
import { cn } from "@compozy/ui/lib/utils";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type MarketScope = {
  name: string;
  scope: string;
  layer: string;
  /**
   * Source of record for the scope claim (COPY.md §7, Required Evidence). Intentionally not
   * rendered: the landing states the scope, the repo carries the receipt, and a local
   * `.resources/` path does not resolve for a visitor. `lib/__tests__/landing-truth.test.tsx`
   * validates every value against the approved set and asserts it exists on disk.
   */
  sourcePath?: string;
  highlight?: boolean;
};

const MARKET_SCOPES: MarketScope[] = [
  {
    name: "Paperclip",
    scope: "Open-source orchestration for teams of AI agents.",
    layer: "Orchestration",
    sourcePath: ".resources/paperclip/README.md",
  },
  {
    name: "Smithers",
    scope: "A durable coding-agent workflow runtime with replay and fork controls.",
    layer: "Durable workflow runtime",
    sourcePath: ".resources/smithers/README.md",
  },
  {
    name: "OpenClaw",
    scope: "A personal AI assistant that connects channels, tools, and automation on your devices.",
    layer: "Personal assistant",
    sourcePath: ".resources/openclaw/README.md",
  },
  {
    name: "T3 Code",
    scope: "A minimal web GUI for coding agents across several providers.",
    layer: "Coding interface",
    sourcePath: ".resources/t3code/README.md",
  },
  {
    name: "Mastra Factory",
    scope: "An agent-powered software delivery environment built on Mastra.",
    layer: "Software factory",
    sourcePath: ".resources/mastra/mastracode/factory/README.md",
  },
  {
    name: "CompozyOS",
    scope:
      "Runs the work, keeps the memory, sets the permissions, connects agents to each other, and stays open at every seam.",
    layer: "Operating system",
    highlight: true,
  },
];

const COLUMNS = "md:grid-cols-[minmax(0,10rem)_minmax(0,1fr)_minmax(0,13rem)]";

export function Comparison() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="The market is proving the layers"
        title="The layer is not the operating system."
        description="The category already values agent orchestration, durable runs, assistants, and coding interfaces. The newest layer even has a name — the software factory. Factories are the machines; an OS is the floor they run on. Compozy brings the operating responsibilities together: run, memory, permissions, connection, and extensibility."
      />

      <div className="mt-10 overflow-hidden rounded-diagram border border-line bg-canvas-soft">
        <div className={cn("hidden border-b border-line px-5 py-4 md:grid md:gap-4", COLUMNS)}>
          <Eyebrow className="text-subtle">Approach</Eyebrow>
          <Eyebrow className="text-subtle">What it does</Eyebrow>
          <Eyebrow className="text-subtle">Layer</Eyebrow>
        </div>

        {MARKET_SCOPES.map(row => (
          <article
            key={row.name}
            className={cn(
              "grid gap-3 border-t border-line p-5 first:border-t-0 md:items-baseline md:gap-4",
              COLUMNS,
              row.highlight && "border-l-4 border-l-accent bg-accent-tint/40"
            )}
          >
            <h3 className={cn("text-sm font-semibold", row.highlight ? "text-accent" : "text-fg")}>
              {row.name}
            </h3>
            <div>
              <Eyebrow className="text-subtle md:hidden">What it does</Eyebrow>
              <p
                className={cn(
                  "text-small-body leading-6",
                  row.highlight ? "text-fg" : "text-muted"
                )}
              >
                {row.scope}
              </p>
            </div>
            <div>
              <Eyebrow className="text-subtle md:hidden">Layer</Eyebrow>
              <p
                className={cn(
                  "text-small-body leading-6",
                  row.highlight ? "font-medium text-accent" : "text-muted"
                )}
              >
                {row.layer}
              </p>
            </div>
          </article>
        ))}
      </div>
    </SectionFrame>
  );
}
