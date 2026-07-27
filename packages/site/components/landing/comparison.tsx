import { Eyebrow } from "@compozy/ui";

import { SectionFrame } from "./primitives/section-frame";

const MARKET_SCOPES = [
  {
    name: "Paperclip",
    scope: "Open-source orchestration for teams of AI agents.",
    sourcePath: ".resources/paperclip/README.md",
  },
  {
    name: "Smithers",
    scope: "A durable coding-agent workflow runtime with replay and fork controls.",
    sourcePath: ".resources/smithers/README.md",
  },
  {
    name: "OpenClaw",
    scope: "A personal AI assistant that connects channels, tools, and automation on your devices.",
    sourcePath: ".resources/openclaw/README.md",
  },
  {
    name: "T3 Code",
    scope: "A minimal web GUI for coding agents across several providers.",
    sourcePath: ".resources/t3code/README.md",
  },
] as const;

export function Comparison() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <div className="grid gap-10 lg:grid-cols-[minmax(0,0.72fr)_minmax(0,1.28fr)] lg:gap-18">
        <div>
          <Eyebrow className="text-accent">The market is proving the layers</Eyebrow>
          <h2 className="mt-5 max-w-[13ch] text-site-hero-section leading-none font-normal tracking-tight text-fg">
            The layer is not the operating system.
          </h2>
          <p className="mt-6 max-w-[47ch] text-base leading-relaxed text-muted">
            The category already values agent orchestration, durable runs, assistants, and coding
            interfaces. Compozy brings the operating responsibilities together: run, memory,
            permissions, connection, and extensibility.
          </p>
        </div>

        <div className="border-t border-line">
          {MARKET_SCOPES.map(entry => (
            <article
              key={entry.name}
              className="grid gap-3 border-b border-line py-5 sm:grid-cols-[10rem_1fr]"
            >
              <h3 className="text-sm font-medium text-fg">{entry.name}</h3>
              <div>
                <p className="text-small-body leading-relaxed text-muted">{entry.scope}</p>
                <p className="mt-2 font-mono text-micro text-subtle">Source: {entry.sourcePath}</p>
              </div>
            </article>
          ))}
        </div>
      </div>
    </SectionFrame>
  );
}
