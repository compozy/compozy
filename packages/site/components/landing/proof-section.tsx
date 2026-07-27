import { Eyebrow } from "@compozy/ui";

import { CtaButton } from "./primitives/cta-button";
import { SectionFrame } from "./primitives/section-frame";

const INSTALL_METHODS = [
  { label: "Installer", command: "curl -fsSL https://compozy.com/install.sh | sh" },
  { label: "npm", command: "npm install -g @compozy/cli@beta" },
  { label: "Go", command: "go install github.com/compozy/compozy@v0.3.0-beta.1" },
] as const;

export function ProofSection() {
  return (
    <SectionFrame id="install" background="surface" padY="lg" className="border-b border-line">
      <div className="grid gap-10 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] lg:gap-18">
        <div>
          <Eyebrow className="text-accent">Proof, not a promise</Eyebrow>
          <h2 className="mt-5 max-w-[13ch] text-site-hero-section leading-none font-normal tracking-tight text-fg">
            Start with the beta. Keep the work.
          </h2>
          <p className="mt-6 max-w-[48ch] text-base leading-relaxed text-muted">
            The beta is the full operating surface in public: one daemon, its CLI, the web UI, and
            the same durable records behind each of them.
          </p>
          <div className="mt-8">
            <CtaButton href="/runtime/core/getting-started/installation" variant="primary">
              Install Compozy beta
            </CtaButton>
          </div>
        </div>

        <div className="border-y border-line">
          {INSTALL_METHODS.map(method => (
            <div
              key={method.label}
              className="grid gap-2 border-b border-line py-5 last:border-b-0 sm:grid-cols-[6rem_1fr]"
            >
              <p className="text-small-body font-medium text-fg">{method.label}</p>
              <code className="overflow-x-auto font-mono text-small-body leading-relaxed text-muted">
                {method.command}
              </code>
            </div>
          ))}
          <p className="py-5 text-small-body leading-relaxed text-subtle">
            Beta installs use the beta channel and versioned Go module shown here.
          </p>
        </div>
      </div>
    </SectionFrame>
  );
}
