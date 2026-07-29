import { Eyebrow } from "@compozy/ui";

import { HeroPlayer } from "./hero-player";
import { BUILTIN_PROVIDER_COUNT } from "./provider-data";
import { CtaButton } from "./primitives/cta-button";
import { NETWORK_KIND_COUNT } from "./primitives/network-kinds";

const featuredAgentNames = ["Claude Code", "OpenClaw", "Hermes"];
const additionalProviderCount = Math.max(0, BUILTIN_PROVIDER_COUNT - featuredAgentNames.length);
const featuredAgentDetail =
  additionalProviderCount > 0
    ? `${featuredAgentNames.join(", ")}, and ${additionalProviderCount} more built-in integrations.`
    : `${featuredAgentNames.join(", ")}.`;

// Locked launch hero (COPY.md §2, packages/site/CLAUDE.md): the headline and this definition
// ship together, verbatim. `lib/og/templates/landing.tsx` carries the same pair for the OG image.
const DEFINITION =
  "A window on top of an agent isn't an OS. An OS runs the work, keeps the memory, sets the permissions, connects agents to each other — and lets you build on it. That's the test, and Compozy is the only one built to pass it.";

const signalItems = [
  {
    label: "Durable sessions, one state",
    detail: "Agent work stays visible and inspectable beyond one terminal interaction.",
  },
  {
    label: `${BUILTIN_PROVIDER_COUNT} built-in providers`,
    detail: featuredAgentDetail,
  },
  {
    label: "Tool registry, one control path",
    detail: "Native tools, MCP servers, and extensions on one control path.",
  },
  {
    label: "Local by default, Live by choice",
    detail: `${NETWORK_KIND_COUNT} typed message kinds, commit-first delivery, and explicit external boundaries.`,
  },
];

export function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-line px-4 pt-8 pb-16 md:pt-12 md:pb-20">
      {/* Background mesh — faded so it textures the whole hero without competing with the copy. */}
      <div
        className="pointer-events-none absolute inset-0 bg-size-[100%_auto] bg-position-[0%_0%] bg-no-repeat opacity-20 mix-blend-screen"
        style={{ backgroundImage: "url('/hero-bg.webp')" }}
        aria-hidden="true"
      />

      <div className="relative mx-auto max-w-site-layout-width">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,540px)] lg:items-center lg:gap-14">
          <div className="order-2 lg:order-0 lg:pr-2">
            <Eyebrow className="text-muted flex items-center gap-3">
              <span className="text-accent">CompozyOS</span>
              <span className="h-px w-10 bg-line" />
              <span>Agent operating system · beta</span>
            </Eyebrow>

            <h1 className="mt-6 max-w-[16ch] text-site-hero leading-none font-normal tracking-tight text-fg">
              The only true OS for AI agents.
            </h1>

            <p className="mt-6 max-w-[60ch] text-base leading-relaxed text-muted md:text-lg">
              {DEFINITION}
            </p>

            <div className="mt-8 flex flex-col items-start gap-3 sm:flex-row sm:flex-wrap">
              <CtaButton href="/runtime/core/getting-started/installation" variant="primary">
                Install the beta
              </CtaButton>
              <CtaButton href="/runtime" variant="ghost">
                See how CompozyOS works
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
