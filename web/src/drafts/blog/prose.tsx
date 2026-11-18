/** The article body: long-form reading type on the shipped Markdown primitive. */
import { CodeBlock, Markdown, cn, type MarkdownProps } from "@compozy/ui";
import type { ComponentProps } from "react";

import { slugify, textOf, type Segment } from "./posts";

function ProseH2({ children }: ComponentProps<"h2">) {
  return (
    <h2
      id={slugify(textOf(children))}
      className="mt-16 mb-5 scroll-mt-28 font-display text-[30px]/[1.15] font-normal tracking-[-0.02em] text-balance text-fg md:text-[34px]/[1.15]"
    >
      {children}
    </h2>
  );
}

function ProseH3({ children }: ComponentProps<"h3">) {
  return <h3 className="mt-10 mb-3 text-[21px]/[1.3] font-medium text-fg">{children}</h3>;
}

/** Fenced code becomes the shipped CodeBlock; everything else stays inline. */
function ProseCode({ className, children }: ComponentProps<"code">) {
  const source = textOf(children);
  const language = /language-(\w+)/.exec(className ?? "")?.[1];
  if (language || source.includes("\n")) {
    return (
      <CodeBlock
        code={source.replace(/\n$/, "")}
        language={language === "text" ? undefined : language}
        copyable
        className="my-8 text-[14px]"
      />
    );
  }
  return (
    <code className="rounded-[6px] bg-[color:var(--light-080)] px-1.5 py-0.5 font-mono text-[0.86em] text-fg">
      {children}
    </code>
  );
}

const COMPONENTS = { h2: ProseH2, h3: ProseH3, code: ProseCode } as MarkdownProps["components"];

const PROSE = cn(
  "text-[17.5px] leading-[1.75] text-fg/85",
  "[&_p]:my-6 [&_li]:my-2 [&_li>p]:my-2 [&_ol]:my-6 [&_ul]:my-6 [&_strong]:text-fg",
  "[&_blockquote]:my-8 [&_blockquote]:border-l-2 [&_blockquote]:border-[color:var(--light-250)] [&_blockquote]:pl-5 [&_blockquote]:text-muted-foreground",
  "[&_[data-slot=markdown-table]]:my-8 [&_td]:text-[15px] [&_th]:text-[15px]"
);

const TONE: Record<string, string> = {
  warning: "border-warning/40 [&_.callout-label]:text-warning",
  destructive: "border-danger/45 [&_.callout-label]:text-danger",
  brand: "border-brand/45 [&_.callout-label]:text-brand-strong",
};

function BlogCallout({ segment }: { segment: Extract<Segment, { kind: "callout" }> }) {
  return (
    <aside
      className={cn(
        "my-9 rounded-[16px] border bg-[color:var(--light-025)] p-6",
        TONE[segment.tone] ?? "border-[color:var(--light-120)] [&_.callout-label]:text-fg"
      )}
    >
      <p className="callout-label m-0 mb-2 text-[14px] font-medium">{segment.label}</p>
      <Markdown className="text-[16.5px] leading-[1.7] text-fg/85 [&_strong]:text-fg">
        {segment.text}
      </Markdown>
    </aside>
  );
}

export function ArticleBody({ segments }: { segments: readonly Segment[] }) {
  return (
    <>
      {segments.map(segment =>
        segment.kind === "callout" ? (
          <BlogCallout key={segment.label} segment={segment} />
        ) : (
          <Markdown key={segment.text.slice(0, 48)} className={PROSE} components={COMPONENTS}>
            {segment.text}
          </Markdown>
        )
      )}
    </>
  );
}
