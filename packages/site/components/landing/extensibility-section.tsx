import { Eyebrow } from "@compozy/ui";
import { ArrowUpRight } from "lucide-react";
import Image from "next/image";
import Link from "next/link";

import { SectionFrame } from "./primitives/section-frame";

const EXTENSIONS_DOCS_HREF = "/runtime/core/extensions";

const SURFACES = [
  {
    label: "Hooks",
    detail: "Carry policy into lifecycle transitions without opening a second control plane.",
  },
  {
    label: "Skills",
    detail: "Keep reusable instructions in versionable, inspectable files.",
  },
  {
    label: "Automation",
    detail: "Schedule and trigger work through durable jobs rather than a sidecar script.",
  },
  {
    label: "Extensions",
    detail: "Package skills, hooks, bridge adapters, and MCP servers as one capability surface.",
  },
] as const;

export function ExtensibilitySection() {
  return (
    <SectionFrame background="surface" padY="lg" className="border-b border-line">
      <div className="grid gap-10 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:items-center lg:gap-18">
        <div>
          <Eyebrow className="text-accent">Criterion two</Eyebrow>
          <h2 className="mt-5 max-w-[14ch] text-site-hero-section leading-none font-normal tracking-tight text-fg">
            Built to be built on.
          </h2>
          <p className="mt-6 max-w-[52ch] text-base leading-relaxed text-muted">
            An operating system has an extension surface that is part of the system, not an escape
            hatch around it. Compozy keeps those contracts readable, local, and reachable by both
            people and agents.
          </p>
          <dl className="mt-8 border-t border-line">
            {SURFACES.map(surface => (
              <div
                key={surface.label}
                className="grid gap-2 border-b border-line py-4 sm:grid-cols-[8rem_1fr]"
              >
                <dt className="text-small-body font-medium text-fg">{surface.label}</dt>
                <dd className="text-small-body leading-relaxed text-muted">{surface.detail}</dd>
              </div>
            ))}
          </dl>
          <Link
            href={EXTENSIONS_DOCS_HREF}
            className="mt-7 inline-flex items-center gap-2 text-sm font-medium text-fg transition-colors hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          >
            Read the extensions reference <ArrowUpRight aria-hidden className="size-4" />
          </Link>
        </div>
        <Image
          src="/images/extensibility-skill-contract-v1.png"
          alt="A Compozy skill contract shown as a Markdown file with frontmatter and an execution trace."
          width={1200}
          height={760}
          decoding="async"
          sizes="(min-width: 1024px) 60vw, 100vw"
          quality={90}
          className="block w-full rounded-diagram border border-line object-cover object-center"
        />
      </div>
    </SectionFrame>
  );
}
