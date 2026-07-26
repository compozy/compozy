import Link from "next/link";
import Image from "next/image";
import { ArrowUpRight } from "lucide-react";
import { LandingCodeBlock } from "./primitives/code-block";
import { SectionFrame } from "./primitives/section-frame";
import { Eyebrow } from "@agh/ui";

const MEMORY_CODE = `agh memory write \\
  --name "Conversation language" \\
  --type user \\
  --description "Pedro prefers BR-PT in conversation" \\
  --content @personal-notes.md
agh memory search "BR-PT"
agh memory dream trigger`;

const STEPS = [
  {
    eyebrow: "Plain files",
    title: "Memory as scoped Markdown",
    description:
      "Typed files: user, feedback, project, reference. They resolve across global, workspace, and agent tiers. Version them. Diff them. Port them across providers.",
  },
  {
    eyebrow: "Dream consolidation",
    title: "Time → Sessions → Lock cascade",
    description:
      "Default gates: 24h, 3 touched sessions, file-lock. When all three pass, AGH spawns an ephemeral session that synthesizes recent activity into durable facts. No surprise compute.",
  },
  {
    eyebrow: "Agent-managed",
    title: "Same surface for you and the agent",
    description:
      "agh memory write | search | dream trigger works from CLI, HTTP, and UDS. Operators inspect the same files agents write; no privileged path.",
  },
];

export function MemoryDreamSection() {
  return (
    <SectionFrame
      className="relative border-b border-line"
      background="canvas"
      padY="lg"
      ariaLabel="Memory and dream consolidation"
    >
      <div className="grid min-w-0 gap-12 lg:grid-cols-[minmax(0,400px)_1fr] lg:items-start lg:gap-16">
        <div className="flex h-full min-w-0 flex-col justify-between lg:sticky lg:top-24">
          <div>
            <Eyebrow className="text-accent">Memory</Eyebrow>
            <h2 className="mt-3 text-site-subsection-title leading-tight font-normal tracking-tight text-fg">
              Memory that compounds
              <br />
              <span className="italic text-subtle">while you sleep.</span>
            </h2>
            <p className="mt-4 max-w-[50ch] text-sm leading-relaxed text-muted">
              Memory is not a vector database. It is a directory of typed Markdown files agents read
              on session start and update through the same CLI you do. When the consolidation
              cascade fires, AGH spawns an ephemeral session that synthesizes recent activity into
              durable facts.
            </p>
            <Link
              href="/runtime"
              className="mt-6 inline-flex items-center gap-1.5 text-sm font-medium text-accent transition-colors hover:text-accent-hover"
            >
              Read the memory and dream guide
              <ArrowUpRight aria-hidden className="size-4" />
            </Link>
          </div>
          <div className="mt-12 hidden lg:block">
            <Image
              src="/images/runtime/memory-dream-landing-v1.png"
              alt="AGH memory interface diagram showing scoped Markdown files, memory indexing, and dream consolidation into durable memory."
              width={800}
              height={640}
              decoding="async"
              sizes="400px"
              quality={90}
              className="block w-full max-w-100 select-none object-contain opacity-95"
            />
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-0">
          <ol className="flex flex-col divide-y divide-line">
            {STEPS.map((step, index) => (
              <li
                key={step.eyebrow}
                className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-6 gap-y-2 py-7 first:pt-0"
              >
                <Eyebrow aria-hidden="true" className="text-accent tabular-nums">
                  {String(index + 1).padStart(2, "0")}
                </Eyebrow>
                <div className="min-w-0">
                  <Eyebrow className="text-accent">{step.eyebrow}</Eyebrow>
                  <h3 className="mt-2 text-base font-medium leading-snug text-fg">{step.title}</h3>
                  <p className="mt-2 max-w-[60ch] text-sm leading-relaxed text-muted">
                    {step.description}
                  </p>
                </div>
              </li>
            ))}
          </ol>

          <div className="mt-10">
            <LandingCodeBlock code={MEMORY_CODE} caption="agh memory" shell />
          </div>
        </div>
      </div>
    </SectionFrame>
  );
}
