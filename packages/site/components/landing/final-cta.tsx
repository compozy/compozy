import { baseOptions } from "@/lib/layout.shared";
import { Eyebrow } from "@compozy/ui";
import { CtaButton } from "./primitives/cta-button";
import { SectionFrame } from "./primitives/section-frame";

export function FinalCta() {
  return (
    <SectionFrame background="surface" padY="lg" className="border-b border-line">
      <div className="grid gap-8 border-y border-line py-10 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-center lg:py-14">
        <div>
          <Eyebrow className="text-accent">CompozyOS beta</Eyebrow>
          <h2 className="mt-4 max-w-[18ch] text-site-cta-title leading-none font-normal tracking-tight text-fg">
            Give agent work an operating system.
          </h2>
          <p className="mt-5 max-w-[52ch] text-sm leading-7 text-muted">
            Install the beta, run a session, and keep the work connected as it grows.
          </p>
        </div>

        <div className="flex flex-col items-start gap-3">
          <CtaButton
            href="/runtime/core/getting-started/installation"
            variant="primary"
            className="w-full justify-center sm:w-auto"
          >
            Install the beta
          </CtaButton>
          <CtaButton href="/protocol" variant="ghost" className="w-full justify-center sm:w-auto">
            Read the protocol
          </CtaButton>
          <a
            href={baseOptions.githubUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-1 text-small-body text-muted transition-colors hover:text-accent"
          >
            View the source on GitHub
          </a>
        </div>
      </div>
    </SectionFrame>
  );
}
