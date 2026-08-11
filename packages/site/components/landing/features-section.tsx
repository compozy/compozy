import Image from "next/image";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";
import { Eyebrow } from "@compozy/ui";

const FEATURES = [
  {
    eyebrow: "Memory",
    title: "Context that survives restarts",
    description:
      "Global and per-workspace memory in plain Markdown. Four types, one index per scope.",
    image: "/images/everything/illustration_02.png",
    imageAlt: "Memory cards stored in a global Markdown index.",
  },
  {
    eyebrow: "Sessions",
    title: "Agent work that outlives the terminal",
    description:
      "Durable sessions keep the timeline, the edits, and the runs. Pause one, resume it, and the context is still there.",
    image: "/images/everything/illustration_01.png",
    imageAlt:
      "Session timeline showing a Claude Code run editing a file, running tests, and committing.",
  },
  {
    eyebrow: "Observability",
    title: "Every run leaves a trace",
    description:
      "Traces, event counts, and tool usage come from the same records the runtime writes. Nothing to instrument.",
    image: "/images/everything/illustration_03.png",
    imageAlt: "Trace timeline beside an events-per-day chart and a top-tools breakdown.",
  },
  {
    eyebrow: "Automation",
    title: "Cron + webhooks, durable",
    description:
      "Schedule recurring work. Trigger sessions from external events. Every run durable and inspectable.",
    image: "/images/everything/illustration_06.png",
    imageAlt: "Automation job fan-out to archive, notify, webhook, and summary actions.",
  },
];

export function FeaturesSection() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="Built in"
        title="Comes with what you would otherwise build."
        description="Loops, memory, automation, permissions, approvals, and run history are core objects in one runtime, reachable from CLI, HTTP, and UDS. Same primitives for you and for the agents you run."
      />

      <ul className="mt-12 grid gap-4 md:grid-cols-2">
        {FEATURES.map(feature => (
          <li key={feature.eyebrow}>
            <article
              data-testid="feature-card"
              className="group flex h-full min-h-105 flex-col overflow-hidden rounded-diagram border border-line bg-canvas-soft p-4 transition-colors duration-slow hover:border-accent/40 sm:p-5"
            >
              <div className="overflow-hidden rounded-md">
                <Image
                  src={feature.image}
                  alt={feature.imageAlt}
                  width={960}
                  height={600}
                  decoding="async"
                  sizes="(min-width: 768px) 50vw, 100vw"
                  quality={90}
                  className="block aspect-16/10 w-full object-contain opacity-95 transition-transform duration-slow ease-out group-hover:scale-[1.02]"
                />
              </div>
              <div className="flex flex-1 flex-col pt-5">
                <Eyebrow className="text-accent">{feature.eyebrow}</Eyebrow>
                <h3 className="mt-3 text-base font-medium leading-snug tracking-tight text-fg">
                  {feature.title}
                </h3>
                <p className="mt-3 text-sm leading-relaxed text-muted">{feature.description}</p>
              </div>
            </article>
          </li>
        ))}
      </ul>
    </SectionFrame>
  );
}
