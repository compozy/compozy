import { Eyebrow } from "@agh/ui";

import { LLMCopyButton, OpenWithAI, ViewOptions } from "@/components/docs/page-actions";

interface DocPageMastheadProps {
  kind: "runtime" | "protocol";
  slug: string[];
  title: string;
  description?: string;
  markdownUrl?: string;
  pageUrl?: string;
  githubUrl?: string;
}

function toLabel(value?: string) {
  if (!value) {
    return "Overview";
  }

  const words: string[] = [];
  for (const part of value.split("-")) {
    if (part.length > 0) {
      words.push(part[0].toUpperCase() + part.slice(1));
    }
  }
  return words.join(" ");
}

function resolveRuntimeSection(slug: string[]) {
  if (slug.length === 0) {
    return "Runtime Overview";
  }

  const [root, section] = slug;

  if (root === "core") {
    return section ? toLabel(section) : "Core Concepts";
  }

  if (root === "cli-reference") {
    return section ? toLabel(section) : "CLI Reference";
  }

  if (root === "api-reference") {
    return "API Reference";
  }

  return toLabel(root);
}

function resolveMeta(kind: "runtime" | "protocol", slug: string[]) {
  if (kind === "runtime") {
    return {
      eyebrow: "AGH Runtime",
      audience: "Operators running durable agent work",
      section: resolveRuntimeSection(slug),
    };
  }

  const family = slug[0];

  return {
    eyebrow: family === "specification" ? "AGH Network Protocol" : "AGH Network",
    audience: "Implementers designing interoperable agents",
    section: toLabel(family ?? "overview"),
  };
}

export function DocPageMasthead({
  kind,
  slug,
  title,
  description,
  markdownUrl,
  pageUrl,
  githubUrl,
}: DocPageMastheadProps) {
  const meta = resolveMeta(kind, slug);
  const showActions = Boolean(markdownUrl && pageUrl && githubUrl);

  return (
    <header className="not-prose border-b border-line pb-8">
      <Eyebrow className="flex flex-wrap items-center gap-3 text-muted">
        <span className="text-accent">{meta.eyebrow}</span>
        <span className="h-px w-8 bg-line" />
        <span>{meta.section}</span>
      </Eyebrow>

      <div className="mt-5 flex flex-col gap-6 md:flex-row md:items-end md:justify-between md:gap-8">
        <h1 className="max-w-[12ch] font-display text-site-doc-title leading-none font-normal tracking-tight text-fg">
          {title}
        </h1>
        {showActions && markdownUrl && pageUrl && githubUrl ? (
          <div className="flex shrink-0 items-center gap-2">
            <LLMCopyButton markdownUrl={markdownUrl} />
            <OpenWithAI pageUrl={pageUrl} />
            <ViewOptions githubUrl={githubUrl} markdownUrl={markdownUrl} />
          </div>
        ) : null}
      </div>

      {description && (
        <p className="mt-4 max-w-[68ch] text-base leading-8 text-muted">{description}</p>
      )}

      <dl className="mt-6 grid gap-5 border-t border-line pt-4 md:grid-cols-2 xl:max-w-3xl">
        <div>
          <dt className="eyebrow font-semibold! text-muted">Audience</dt>
          <dd className="mt-2 text-sm leading-6 text-muted">{meta.audience}</dd>
        </div>
        <div>
          <dt className="eyebrow font-semibold! text-muted">Focus</dt>
          <dd className="mt-2 text-sm leading-6 text-muted">
            {meta.section} guidance shaped for scanability, day-two clarity, and operator context.
          </dd>
        </div>
      </dl>
    </header>
  );
}
