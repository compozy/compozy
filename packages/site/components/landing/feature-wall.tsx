import { Eyebrow } from "@compozy/ui";
import { ArrowUpRight } from "lucide-react";
import Link from "next/link";

import { SectionFrame } from "./primitives/section-frame";

const SYSTEM_SURFACES = [
  {
    title: "Run the work",
    description:
      "Durable sessions and loop runs give agent work a home that outlives one terminal window.",
    href: "/runtime/core/sessions",
    label: "Sessions",
  },
  {
    title: "Keep the memory",
    description:
      "Memory is kept as scoped, inspectable files that can carry intent between workspaces and agents.",
    href: "/runtime/core/memory",
    label: "Memory",
  },
  {
    title: "Set the permissions",
    description:
      "Providers, tools, and runtime policy meet in one control path instead of in disconnected prompts.",
    href: "/runtime/core/tools",
    label: "Control plane",
  },
  {
    title: "Connect the agents",
    description:
      "The implemented network gives peers a durable way to discover, delegate, and close work with receipts.",
    href: "/protocol",
    label: "Network",
  },
] as const;

export function FeatureWall() {
  return (
    <SectionFrame background="canvas" padY="xl" className="border-b border-line">
      <div className="max-w-[44rem]">
        <Eyebrow className="text-accent">The operating system</Eyebrow>
        <h2 className="mt-5 text-site-hero-section leading-none font-normal tracking-tight text-fg">
          The work is one system, not a stack of windows.
        </h2>
        <p className="mt-5 max-w-[60ch] text-base leading-relaxed text-muted">
          Sessions, memory, policy, and coordination stay connected so an agent&apos;s work remains
          legible after the first prompt and useful after the first run.
        </p>
      </div>

      <div className="mt-14 grid border-y border-line md:grid-cols-2">
        {SYSTEM_SURFACES.map(surface => (
          <article
            key={surface.title}
            className="group flex min-h-60 flex-col justify-between border-line px-0 py-8 md:px-8 md:py-10 [&:nth-child(odd)]:md:border-r [&:nth-child(n+3)]:border-t"
          >
            <div>
              <Eyebrow className="text-subtle">{surface.label}</Eyebrow>
              <h3 className="mt-5 text-site-feature-title leading-none font-normal tracking-tight text-fg">
                {surface.title}
              </h3>
              <p className="mt-4 max-w-[43ch] text-sm leading-relaxed text-muted">
                {surface.description}
              </p>
            </div>
            <Link
              href={surface.href}
              className="mt-8 inline-flex w-fit items-center gap-2 text-small-body font-medium text-fg transition-colors hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              Open {surface.label.toLowerCase()} <ArrowUpRight aria-hidden className="size-4" />
            </Link>
          </article>
        ))}
      </div>
    </SectionFrame>
  );
}
