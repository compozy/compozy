import { Eyebrow } from "@agh/ui";

import { HeroPlayer } from "./hero-player";
import { SUPPORTED_AGENT_COUNT } from "./provider-data";
import { CtaButton } from "./primitives/cta-button";
import { NETWORK_KIND_COUNT } from "./primitives/network-kinds";

const featuredAgentNames = ["Claude Code", "OpenClaw", "Hermes"];
const additionalAgentCount = Math.max(0, SUPPORTED_AGENT_COUNT - featuredAgentNames.length);
const featuredAgentDetail =
  additionalAgentCount > 0
    ? `${featuredAgentNames.join(", ")}, and ${additionalAgentCount} more ${additionalAgentCount === 1 ? "agent" : "agents"}.`
    : `${featuredAgentNames.join(", ")}.`;

const signalItems = [
  {
    label: "agh-network/v0, alpha runtime",
    detail: `${NETWORK_KIND_COUNT} message kinds. Commit-first delivery. Audited outcomes.`,
  },
  {
    label: `${SUPPORTED_AGENT_COUNT} ACP drivers supported`,
    detail: featuredAgentDetail,
  },
  {
    label: "Tool registry, one control path",
    detail: "Native Go tools, MCP servers, and extensions through canonical ToolIDs.",
  },
  {
    label: "Single binary, no infra",
    detail: "No Docker. No Postgres. agh daemon start.",
  },
];

export function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-line px-4 pt-8 pb-16 md:pt-12 md:pb-20">
      {/* Background mesh , restored and faded so it textures the whole hero. */}
      <div
        className="pointer-events-none absolute inset-0 bg-size-[100%_auto] bg-position-[0%_0%] bg-no-repeat opacity-20 mix-blend-screen"
        style={{ backgroundImage: "url('/hero-bg.webp')" }}
        aria-hidden="true"
      />

      <div className="relative mx-auto max-w-site-layout-width">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,540px)] lg:items-center lg:gap-14">
          <div className="order-2 lg:order-0 lg:pr-2">
            <Eyebrow className="text-muted flex items-center gap-3">
              <span className="text-accent">AGH</span>
              <span className="h-px w-10 bg-line" />
              <span>Artificial General Hivemind</span>
            </Eyebrow>

            <h1 className="mt-6 max-w-[20ch] text-site-hero leading-none font-normal tracking-tight text-fg">
              An open workplace for AI agents.
            </h1>

            <p className="mt-6 max-w-[60ch] text-base leading-relaxed text-muted md:text-lg">
              AGH runs the agent CLIs you already use as durable sessions, with memory, autonomy,
              tools, and automation, connected on agh-network/v0 channels where they find each
              other, share capabilities, and close work with receipts.
            </p>

            <div className="mt-8 flex flex-col items-start gap-3 sm:flex-row sm:flex-wrap">
              <CtaButton href="/runtime/core/getting-started/installation" variant="primary">
                Install the runtime
              </CtaButton>
              <CtaButton href="/protocol" variant="ghost">
                Read the agh-network/v0 spec
              </CtaButton>
            </div>
          </div>

          {/* Visual comes after copy on desktop (and on mobile flows under). */}
          <div className="order-1 lg:order-0">
            <HeroPlayer />
          </div>
        </div>
        <dl className="mt-10 grid grid-cols-2 gap-3 md:grid-cols-4">
          {signalItems.map(item => (
            <div
              key={item.label}
              className="rounded-diagram border border-line-strong p-4 backdrop-blur-sm"
            >
              <dt className="eyebrow font-semibold! text-accent">{item.label}</dt>
              <dd className="mt-1.5 text-xs leading-relaxed text-muted">{item.detail}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
