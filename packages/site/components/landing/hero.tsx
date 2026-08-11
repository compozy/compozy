import { Eyebrow } from "@compozy/ui";

import { HeroVisual } from "./hero-visual";
import { CtaButton } from "./primitives/cta-button";
import { BUILTIN_PROVIDER_COUNT } from "./provider-data";

const featuredAgentNames = ["Claude Code", "OpenClaw", "Hermes"];
const additionalProviderCount = Math.max(0, BUILTIN_PROVIDER_COUNT - featuredAgentNames.length);
const featuredAgentDetail =
  additionalProviderCount > 0
    ? `${featuredAgentNames.join(", ")}, and ${additionalProviderCount} more built-in integrations.`
    : `${featuredAgentNames.join(", ")}.`;

// Locked hero (COPY.md §2 Hero Lock): the headline and this subhead ship together, verbatim.
// `lib/og/templates/landing.tsx` carries the same pair for the OG image.
const SUBHEAD =
  "One complete environment to create, automate, and supervise agent work, without scripts, plugin chains, or orchestration frameworks.";

const signalItems = [
  {
    label: "Create",
    detail: "Loops, capabilities, and agent sessions, defined once and run on demand.",
  },
  {
    label: "Automate",
    detail: "Cron schedules, webhooks, and event triggers the daemon owns.",
  },
  {
    label: "Supervise",
    detail: "Approvals, permissions, and full run history on every session.",
  },
  {
    label: `${BUILTIN_PROVIDER_COUNT} built-in providers`,
    detail: `Runs the agents you already use: ${featuredAgentDetail}`,
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
        <div className="grid gap-12 lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)] lg:items-center lg:gap-10">
          <div className="lg:pr-2">
            <Eyebrow className="text-muted flex items-center gap-3">
              <span className="text-accent">CompozyOS</span>
              <span className="h-px w-10 bg-line" />
              <span>An operating system for AI agents · beta</span>
            </Eyebrow>

            <h1 className="mt-6 max-w-[18ch] font-display text-site-hero leading-none font-normal tracking-tight text-fg">
              The system around the agent, already built.
            </h1>

            <p className="mt-6 max-w-[60ch] text-base leading-relaxed text-muted md:text-lg">
              {SUBHEAD}
            </p>

            <div className="mt-8 flex flex-col items-start gap-3 sm:flex-row sm:flex-wrap">
              <CtaButton href="/docs/getting-started/installation" variant="primary">
                Install the beta
              </CtaButton>
              <CtaButton href="/docs" variant="ghost">
                See how CompozyOS works
              </CtaButton>
            </div>
          </div>

          {/* The 3D shell capture; on lg it overruns the layout column so the
              section's overflow-hidden crops it at the viewport edge, deck-cover style. */}
          <div className="relative">
            <HeroVisual className="mx-auto w-full max-w-140 lg:mx-0 lg:w-[120%] lg:max-w-none" />
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
