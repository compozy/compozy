/** The docs' own MDX components, redrawn: notes, callouts, link lists, workflows and diagrams. */
import { cn } from "@compozy/ui";
import { ArrowRight, ArrowUpRight, Info, TriangleAlert } from "@compozy/ui/icons";
import { use } from "react";

import { useMermaid } from "./hooks/use-mermaid";
import type { Block, LinkItem } from "./mdx";
import { slugFromHref } from "./nav";
import { DocsNavContext } from "./nav-context";
import { DocsCode, DocsProse } from "./prose";
import { BEAM, FOCUS, LINE } from "./text";

const INSET = "text-[15px] leading-[1.65] [&_p]:my-2 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0";

function DocsNote({ label, text }: { label: string; text: string }) {
  return (
    <aside className="relative my-8 rounded-[14px] border border-[color:var(--light-090)] bg-[color:var(--light-025)] py-5 pr-6 pl-7">
      <span aria-hidden className={cn("absolute inset-y-5 left-0 w-0.5 rounded-full", BEAM)} />
      <p className="m-0 mb-2 font-mono text-[12.5px] text-brand-strong">{label}</p>
      <DocsProse text={text} className={INSET} />
    </aside>
  );
}

const TONES: Record<string, string> = {
  warn: "border-warning/40 [&_svg]:text-warning",
  warning: "border-warning/40 [&_svg]:text-warning",
  error: "border-danger/45 [&_svg]:text-danger",
};

function DocsCallout({ tone, text }: { tone: string; text: string }) {
  const Glyph = tone === "info" ? Info : TriangleAlert;
  return (
    <aside
      className={cn(
        "my-8 flex gap-3.5 rounded-[14px] border bg-[color:var(--light-025)] p-5",
        TONES[tone] ?? "border-brand/40 [&_svg]:text-brand-strong"
      )}
    >
      <Glyph className="mt-1 size-4 shrink-0" aria-hidden />
      <DocsProse text={text} className={cn(INSET, "min-w-0")} />
    </aside>
  );
}

function RouteRows({
  items,
  onPick,
}: {
  items: readonly LinkItem[];
  onPick: (href: string) => void;
}) {
  return (
    <ul
      className={cn("my-8 list-none divide-y divide-[color:var(--light-080)] border-y p-0", LINE)}
    >
      {items.map(item => (
        <li key={item.href}>
          <button
            type="button"
            onClick={() => onPick(item.href)}
            className={cn(
              "group grid w-full cursor-pointer gap-x-6 gap-y-1 bg-transparent py-5 text-left sm:grid-cols-[120px_minmax(0,1fr)_auto]",
              FOCUS
            )}
          >
            <span className="pt-0.5 font-mono text-[12.5px] text-subtle">{item.label}</span>
            <span className="min-w-0">
              <span className="block text-[16.5px]/[1.35] text-fg">{item.title}</span>
              <span className="mt-1 block text-[14.5px]/[1.55] text-muted-foreground">
                {item.description}
              </span>
            </span>
            <span className="flex items-center gap-3 self-start pt-0.5 text-[13px] text-subtle">
              {item.meta}
              <ArrowRight
                className="size-4 transition-transform group-hover:translate-x-1 group-hover:text-fg"
                aria-hidden
              />
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

function GuideCards({
  items,
  onPick,
}: {
  items: readonly LinkItem[];
  onPick: (href: string) => void;
}) {
  return (
    <div className="my-8 grid gap-4 sm:grid-cols-2">
      {items.map(item => (
        <button
          key={item.href}
          type="button"
          onClick={() => onPick(item.href)}
          className={cn(
            "group flex cursor-pointer flex-col rounded-[16px] border border-[color:var(--light-090)] bg-[color:var(--light-025)] p-5 text-left transition-colors hover:border-[color:var(--light-200)] hover:bg-[color:var(--light-040)]",
            FOCUS
          )}
        >
          <span className="flex items-center justify-between font-mono text-[12.5px] text-subtle">
            {item.label || item.meta}
            <ArrowUpRight
              className="size-4 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-fg"
              aria-hidden
            />
          </span>
          <span className="mt-4 block text-[17px]/[1.3] text-fg">{item.title}</span>
          <span className="mt-2 block text-[14.5px]/[1.55] text-muted-foreground">
            {item.description}
          </span>
          {item.label && item.meta ? (
            <span className="mt-4 text-[13px] text-subtle">{item.meta}</span>
          ) : null}
        </button>
      ))}
    </div>
  );
}

function DocsWorkflow({ steps }: { steps: readonly { title: string; text: string }[] }) {
  return (
    <ol className="relative my-9 list-none p-0">
      <span
        aria-hidden
        className="absolute top-3 bottom-3 left-[13px] w-px bg-[color:var(--light-120)]"
      />
      {steps.map((step, order) => (
        <li
          key={step.title}
          className="relative grid grid-cols-[28px_minmax(0,1fr)] gap-5 pb-8 last:pb-0"
        >
          <span className="relative z-10 inline-flex size-7 items-center justify-center rounded-full border border-[color:var(--light-200)] bg-[#0a0a0a] font-mono text-[12px] text-brand-strong tabular-nums">
            {order + 1}
          </span>
          <div className="min-w-0 pt-0.5">
            <DocsProse text={`**${step.title}**`} className="[&_p]:my-0 [&_strong]:text-[16.5px]" />
            <DocsProse text={step.text} className={cn(INSET, "mt-2 text-fg/75")} />
          </div>
        </li>
      ))}
    </ol>
  );
}

function DocsDiagram({ chart, caption }: { chart: string; caption: string }) {
  const image = useMermaid(chart);
  return (
    <figure className="my-9">
      {image ? (
        <div
          className={cn(
            "flex justify-center overflow-x-auto rounded-[16px] border bg-[color:var(--light-025)] p-7",
            LINE
          )}
        >
          <img src={image} alt={caption || "Diagram"} className="h-auto max-h-[520px] max-w-full" />
        </div>
      ) : (
        <DocsCode code={chart} language="mermaid" label="Diagram source" />
      )}
      {caption ? (
        <figcaption className="mt-3 text-center text-[13.5px] text-subtle">{caption}</figcaption>
      ) : null}
    </figure>
  );
}

function ApiStub() {
  return (
    <DocsNote
      label="Generated reference"
      text="This page is generated from the OpenAPI document. The prototype carries the Sessions resource in full: open **Sessions** in the sidebar to see the endpoint layout."
    />
  );
}

function DocBlock({ block, onPick }: { block: Block; onPick: (href: string) => void }) {
  switch (block.kind) {
    case "md":
      return <DocsProse text={block.text} />;
    case "note":
      return <DocsNote label={block.label} text={block.text} />;
    case "callout":
      return <DocsCallout tone={block.tone} text={block.text} />;
    case "links":
      return block.variant === "routes" ? (
        <RouteRows items={block.items} onPick={onPick} />
      ) : (
        <GuideCards items={block.items} onPick={onPick} />
      );
    case "workflow":
      return <DocsWorkflow steps={block.steps} />;
    case "diagram":
      return <DocsDiagram chart={block.chart} caption={block.caption} />;
    case "openapi":
      return <ApiStub />;
    default:
      return null;
  }
}

export function DocBlocks({ blocks }: { blocks: readonly Block[] }) {
  const open = use(DocsNavContext);
  const pick = (href: string) => {
    const slug = slugFromHref(href);
    if (slug !== null) open(slug);
  };
  return (
    <>
      {blocks.map(block => (
        <DocBlock key={block.id} block={block} onPick={pick} />
      ))}
    </>
  );
}
