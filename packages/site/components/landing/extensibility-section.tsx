import { Eyebrow } from "@agh/ui";
import { ArrowUpRight, BookOpen, Box, FileCode2, Plug, Sparkles, Timer } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { FeatureCard } from "./primitives/feature-card";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

const EXTENSIONS_DOCS_HREF = "/runtime/core/extensions";

const FEATURES = [
  {
    icon: <FileCode2 className="size-4" />,
    eyebrow: "Hooks",
    title: "Typed dispatch on every state transition",
    description:
      "Not an event bus. ~24 typed lifecycle hooks fire at the call site that owns the transition: session, prompt, tool, permission, autonomy. Hooks can deny or narrow, never bypass.",
    cite: { href: "/runtime/core/hooks", label: "hooks catalog" },
  },
  {
    icon: <Sparkles className="size-4" />,
    eyebrow: "Skills",
    title: "Drop-in SKILL.md bundles",
    description:
      "Share reusable instruction sets with YAML frontmatter and Markdown body. Bundled defaults + global + workspace scopes.",
    cite: { href: "/runtime/core/skills", label: "skills guide" },
  },
  {
    icon: <Timer className="size-4" />,
    eyebrow: "Automation",
    title: "Cron + webhook + event triggers",
    description:
      "Durable jobs and triggers stored in SQLite. Schedule work. Delegate to peers. Track runs.",
    cite: { href: "/runtime/core/automation", label: "automation" },
  },
  {
    icon: <Box className="size-4" />,
    eyebrow: "Sandbox",
    title: "Run agents away from the host filesystem",
    description:
      "Stay local when isolation isn't needed, or bind a workspace to a Daytona sandbox with explicit sync, lifecycle, and provider metadata.",
    cite: { href: "/runtime/core/sandbox/profiles", label: "sandbox profiles" },
  },
  {
    icon: <Plug className="size-4" />,
    eyebrow: "Extensions",
    title: "Install from local or marketplace",
    description:
      "Extensions bundle skills, hooks, bridge adapters, and MCP servers. Ship them as zip files or via a GitHub registry.",
    cite: { href: EXTENSIONS_DOCS_HREF, label: "extensions" },
  },
];

export function ExtensibilitySection() {
  return (
    <SectionFrame background="canvas" padY="lg" className="border-b border-line">
      <SectionHeader
        align="start"
        eyebrow="Extensibility"
        title="Hooks, skills, automation, sandbox, extensions."
        description="The daemon is extensible at every seam you actually need. No plugins to write; contracts are plain files."
      />

      <div className="mt-20 grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {FEATURES.map(feature => (
          <FeatureCard
            key={feature.eyebrow}
            icon={feature.icon}
            eyebrow={feature.eyebrow}
            title={feature.title}
            description={feature.description}
            cite={feature.cite}
          />
        ))}
        <article className="group relative flex min-h-55 flex-col items-start justify-center gap-4 rounded-diagram border border-dashed border-line bg-transparent p-6 transition-colors hover:border-accent/55 hover:bg-accent/4">
          <span className="flex size-12 items-center justify-center rounded-diagram border border-dashed border-line text-muted transition-colors group-hover:border-accent group-hover:text-accent">
            <BookOpen aria-hidden className="size-5" />
          </span>
          <Eyebrow className="text-subtle">Reference</Eyebrow>
          <h3 className="text-base font-medium leading-snug text-fg">
            Every extensibility surface, in one reference.
          </h3>
          <p className="text-sm leading-relaxed text-muted">
            Hooks, skills, automation, sandbox, extensions: schemas, CLI verbs, examples.
          </p>
          <Link
            href={EXTENSIONS_DOCS_HREF}
            className="eyebrow font-semibold! mt-1 inline-flex items-center gap-1.5 text-accent before:absolute before:inset-0 before:rounded-diagram before:content-[''] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          >
            Read extensions docs
            <ArrowUpRight
              aria-hidden
              className="size-3 transition-transform duration-slow group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
            />
          </Link>
        </article>
      </div>

      <div className="mt-10 grid gap-8 lg:grid-cols-[minmax(0,360px)_minmax(0,1fr)] lg:items-center lg:gap-10">
        <div className="max-w-[56ch] text-sm leading-relaxed text-muted">
          <h3 className="font-display text-2xl mb-2 mt-8 text-fg-strong">
            A skill is a Markdown file with frontmatter.
          </h3>
          <p>
            A hook is a TOML block in your config. Everything the daemon loads is inspectable with{" "}
            <code className="font-mono text-fg">agh skill view</code>,{" "}
            <code className="font-mono text-fg">agh hooks list</code>, and{" "}
            <code className="font-mono text-fg">agh extension list</code>.
          </p>
          <Eyebrow className="mt-4 text-subtle">Contract on disk, not a plugin API.</Eyebrow>
        </div>
        <Image
          src="/images/extensibility-skill-contract-v1.png"
          alt="deploy-staging.skill.md shown as a Markdown skill contract with frontmatter, deployment capabilities, and a staged execution trace."
          width={1200}
          height={760}
          decoding="async"
          sizes="(min-width: 1024px) 60vw, 100vw"
          quality={90}
          className="block w-full object-cover object-center opacity-95"
        />
      </div>
    </SectionFrame>
  );
}
