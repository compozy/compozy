import { Eyebrow } from "@compozy/ui";
import Image from "next/image";

import { CtaButton } from "./primitives/cta-button";

const DEFINITION =
  "A window on top of an agent isn't an OS. An OS runs the work, keeps the memory, sets the permissions, connects agents to each other — and lets you build on it. That's the test, and Compozy is the only one built to pass it.";

export function Hero() {
  return (
    <section className="border-b border-line bg-canvas px-4 py-10 md:py-16">
      <div className="mx-auto max-w-site-layout-width">
        <div className="grid gap-10 lg:grid-cols-[minmax(0,0.88fr)_minmax(0,1.12fr)] lg:items-center lg:gap-14">
          <div className="max-w-[43rem]">
            <Eyebrow className="text-accent">CompozyOS · beta</Eyebrow>
            <h1 className="mt-5 max-w-[12ch] text-site-hero leading-[0.92] font-normal tracking-tight text-fg">
              The only true OS for AI agents.
            </h1>
            <p className="mt-7 max-w-[58ch] text-site-lead leading-relaxed text-muted">
              {DEFINITION}
            </p>
            <div className="mt-8 flex flex-col items-start gap-3 sm:flex-row">
              <CtaButton href="/runtime/core/getting-started/installation" variant="primary">
                Install the beta
              </CtaButton>
              <CtaButton href="/runtime/core/extensions" variant="ghost">
                Explore the system
              </CtaButton>
            </div>
          </div>

          <figure className="relative min-w-0" aria-labelledby="hero-shell-caption">
            <div className="overflow-hidden rounded-diagram border border-line-strong bg-rail shadow-overlay">
              <Image
                src="/images/landing/os-shell-hero.png"
                alt="Compozy OS shell with multiple agent workspaces, a task board, and an active loop run."
                width={1800}
                height={1177}
                priority
                sizes="(min-width: 1024px) 58vw, 100vw"
                className="block h-auto w-full"
              />
            </div>
            <figcaption
              id="hero-shell-caption"
              className="mt-4 max-w-[58ch] text-small-body leading-relaxed text-subtle"
            >
              One operating surface: windows for agent work, a task board for durable contracts, and
              loop runs that retain their evidence.
            </figcaption>
          </figure>
        </div>
      </div>
    </section>
  );
}
