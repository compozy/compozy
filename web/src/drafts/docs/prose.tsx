/** Reading type for the docs: headings that anchor, links that stay in the docs, and framed code. */
import { CodeBlock, CopyIconButton, Markdown, cn, type MarkdownProps } from "@compozy/ui";
import { Hash } from "@compozy/ui/icons";
import { use, type ComponentProps } from "react";

import { slugify } from "./mdx";
import { slugFromHref } from "./nav";
import { DocsNavContext } from "./nav-context";
import { LINE, textOf } from "./text";

const SHELL = new Set(["bash", "sh", "shell", "zsh", "console"]);

function ProseH2({ children }: ComponentProps<"h2">) {
  const text = textOf(children);
  const id = slugify(text);
  const step = typeof children === "string" ? /^(\d+)\.\s+(.*)$/.exec(children) : null;
  return (
    <h2
      id={id}
      className="group relative mt-14 mb-4 flex items-start gap-3.5 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-balance text-fg first:mt-0"
    >
      {step ? (
        <span className="mt-[3px] inline-flex h-7 min-w-7 shrink-0 items-center justify-center rounded-[9px] border border-[color:var(--light-150)] px-1.5 docs-count font-mono text-[13px] text-brand-strong">
          {step[1]?.padStart(2, "0")}
        </span>
      ) : null}
      <span className="min-w-0">{step ? step[2] : children}</span>
      <a
        href={`#${id}`}
        aria-label={`Link to ${text}`}
        className="mt-1.5 text-subtle opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
      >
        <Hash className="size-4" aria-hidden />
      </a>
    </h2>
  );
}

function ProseH3({ children }: ComponentProps<"h3">) {
  return (
    <h3 id={slugify(textOf(children))} className="mt-9 mb-3 text-[19px]/[1.35] font-medium text-fg">
      {children}
    </h3>
  );
}

function ProseLink({ href, children }: ComponentProps<"a">) {
  const open = use(DocsNavContext);
  const slug = slugFromHref(href ?? "");
  return (
    <a
      href={href}
      onClick={event => {
        if (slug === null) return;
        event.preventDefault();
        open(slug);
      }}
      className="text-fg underline decoration-[color:var(--light-350)] underline-offset-[3px] transition-colors hover:decoration-brand-strong"
    >
      {children}
    </a>
  );
}

/** One shell command reads as a prompt line: no header, no box inside a box. */
function CommandLine({ code }: { code: string }) {
  return (
    <div
      className={cn(
        "my-5 flex items-center gap-3 rounded-[12px] border bg-[#0d0d0d] py-1.5 pr-1.5 pl-4 font-mono text-[13.5px]/[1.6]",
        LINE
      )}
    >
      <span aria-hidden className="text-brand-strong select-none">
        $
      </span>
      <code className="min-w-0 flex-1 overflow-x-auto py-1.5 whitespace-pre text-fg">{code}</code>
      <CopyIconButton value={code} />
    </div>
  );
}

/** A code frame with its own header, so the label never crowds the first line. */
export function DocsCode({
  code,
  language,
  label,
}: {
  code: string;
  language?: string;
  label?: string;
}) {
  const shell = !language || SHELL.has(language);
  if (shell && language && !label && !code.includes("\n") && !code.startsWith("#")) {
    return <CommandLine code={code} />;
  }
  return (
    <div className={cn("my-6 overflow-hidden rounded-[14px] border bg-[#0d0d0d]", LINE)}>
      <div className={cn("flex h-10 items-center justify-between border-b pr-2 pl-4", LINE)}>
        <span className="flex items-center gap-2 font-mono text-[12.5px] text-subtle">
          {shell ? (
            <span aria-hidden className="text-brand-strong">
              $
            </span>
          ) : null}
          {label ?? (shell ? "Terminal" : language)}
        </span>
        <CopyIconButton value={code} />
      </div>
      <CodeBlock
        code={code}
        language={language}
        caption=""
        copyable={false}
        className="rounded-none border-0 bg-transparent text-[13.5px]"
      />
    </div>
  );
}

function ProseCode({ className, children }: ComponentProps<"code">) {
  const source = textOf(children);
  const language = /language-(\w+)/.exec(className ?? "")?.[1];
  if (language || source.includes("\n")) {
    return (
      <DocsCode
        code={source.replace(/\n$/, "")}
        language={language === "text" ? undefined : language}
      />
    );
  }
  return (
    <code className="rounded-[6px] border border-[color:var(--light-080)] bg-[color:var(--light-050)] px-1.5 py-0.5 font-mono text-[0.86em] text-fg">
      {children}
    </code>
  );
}

const COMPONENTS = {
  h2: ProseH2,
  h3: ProseH3,
  a: ProseLink,
  code: ProseCode,
} as MarkdownProps["components"];

const PROSE = cn(
  "docs-prose text-[16px] leading-[1.72] text-fg/80",
  "[&_pre]:m-0 [&_pre]:rounded-none [&_pre]:border-0 [&_pre]:bg-transparent",
  "[&_p]:my-5 [&_li]:my-1.5 [&_li>p]:my-1.5 [&_ol]:my-5 [&_ul]:my-5 [&_strong]:font-medium [&_strong]:text-fg",
  "[&_blockquote]:my-7 [&_blockquote]:border-l-2 [&_blockquote]:border-[color:var(--light-250)] [&_blockquote]:pl-5 [&_blockquote]:text-muted-foreground",
  "[&_[data-slot=markdown-table]]:my-7 [&_td]:py-3 [&_td]:text-[14.5px] [&_td]:leading-[1.55] [&_th]:text-[13px] [&_th]:font-normal [&_th]:text-subtle"
);

export function DocsProse({ text, className }: { text: string; className?: string }) {
  return (
    <Markdown className={cn(PROSE, className)} components={COMPONENTS}>
      {text}
    </Markdown>
  );
}
