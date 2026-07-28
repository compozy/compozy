import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { LandingCodeBlock } from "./primitives/code-block";
import { SectionFrame } from "./primitives/section-frame";
import { Eyebrow } from "@compozy/ui";

const MEMORY_CODE = `compozy memory write \\
  --name "Release checklist" \\
  --type project \\
  --description "Steps this team runs before every release" \\
  --content @release-notes.md
compozy memory search "release checklist"
compozy memory dream trigger`;

const STEPS = [
  {
    eyebrow: "Plain files",
    title: "Memory as scoped Markdown",
    description:
      "Typed files: user, feedback, project, reference. They resolve across global, workspace, and agent tiers. Version them. Diff them. Port them across providers.",
  },
  {
    eyebrow: "Dream consolidation",
    title: "Time → Sessions → Lock → Signal cascade",
    description:
      "Default gates: 24 hours since the last successful run, 3 completed sessions, a per-workspace dreaming lock, then a recall-signal threshold. When all four pass, CompozyOS spawns an ephemeral session that synthesizes recent activity into durable facts. No surprise compute.",
  },
  {
    eyebrow: "Agent-managed",
    title: "Same surface for you and the agent",
    description:
      "compozy memory write | search | dream trigger works from CLI, HTTP, and UDS. You read the same files agents write; there is no privileged path.",
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
        <div className="flex min-w-0 flex-col lg:sticky lg:top-24">
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
              cascade fires, CompozyOS spawns an ephemeral session that synthesizes recent activity
              into durable facts.
            </p>
            <Link
              href="/runtime/core/memory/dream"
              className="mt-6 inline-flex items-center gap-1.5 text-sm font-medium text-accent transition-colors hover:text-accent-hover"
            >
              Read the memory and dream guide
              <ArrowUpRight aria-hidden className="size-4" />
            </Link>
          </div>
          {/* The memory storyboard is withheld until the artwork drops the retired AGH wordmark
              stamped on the device. The rail reads on its own; a wrong-brand image does not. */}
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
            <LandingCodeBlock code={MEMORY_CODE} caption="compozy memory" shell />
          </div>
        </div>
      </div>
    </SectionFrame>
  );
}
