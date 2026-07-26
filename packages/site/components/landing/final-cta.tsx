import { baseOptions } from "@/lib/layout.shared";
import { Eyebrow } from "@agh/ui";
import { Star } from "lucide-react";
import { CtaButton } from "./primitives/cta-button";
import { SectionFrame } from "./primitives/section-frame";

export function FinalCta() {
  return (
    <SectionFrame background="surface" padY="lg" className="border-b border-line">
      <div className="grid gap-8 rounded-diagram border border-line bg-canvas px-6 py-10 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-center lg:px-10">
        <div>
          <Eyebrow className="text-accent">Ship it</Eyebrow>
          <h2 className="mt-4 max-w-[18ch] text-site-cta-title leading-none font-normal tracking-tight text-fg">
            Install AGH. Run a session. Join the network.
          </h2>
          <p className="mt-5 max-w-[52ch] text-sm leading-7 text-muted">
            One binary. No infrastructure. Alpha runtime included.
          </p>
        </div>

        <div className="flex flex-col items-start gap-3">
          <CtaButton
            href="/runtime/core/getting-started/installation"
            variant="primary"
            className="w-full justify-center sm:w-auto"
          >
            Install AGH
          </CtaButton>
          <CtaButton href="/protocol" variant="ghost" className="w-full justify-center sm:w-auto">
            Read agh-network/v0 spec
          </CtaButton>
          <a
            href={baseOptions.githubUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-1 inline-flex items-center gap-2 text-muted transition-colors hover:text-accent"
          >
            <Star aria-hidden className="size-3" />
            <Eyebrow>Star on GitHub</Eyebrow>
          </a>
        </div>
      </div>
    </SectionFrame>
  );
}
