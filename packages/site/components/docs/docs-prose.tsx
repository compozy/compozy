/** Reading type for the docs: headings that anchor, numbered steps, links, and framed code. */
import { CopyIconButton, cn } from "@compozy/ui";
import { Hash } from "@compozy/ui/icons";
import Link from "next/link";
import {
  isValidElement,
  type ComponentProps,
  type CSSProperties,
  type ReactElement,
  type ReactNode,
} from "react";

import { LINE, textOf } from "@/components/docs/docs-classes";

const SHELL = new Set(["bash", "sh", "shell", "shellscript", "zsh", "console"]);
const PLAIN = new Set(["text", "txt", "plaintext", "plain"]);

export function DocsH2({ children, id }: ComponentProps<"h2">) {
  const text = textOf(children);
  const step = typeof children === "string" ? /^(\d+)\.\s+(.*)$/.exec(children) : null;
  return (
    <h2
      id={id}
      className="group relative mt-8 mb-2.5 flex items-start gap-3.5 font-display text-[27px]/[1.2] font-normal tracking-[-0.02em] text-balance text-fg first:mt-0"
    >
      {step ? (
        <span className="docs-count mt-[3px] inline-flex h-7 min-w-7 shrink-0 items-center justify-center rounded-[9px] border border-site-light-150 px-1.5 font-mono text-[13px] text-brand-strong">
          {step[1]?.padStart(2, "0")}
        </span>
      ) : null}
      <span className="min-w-0">{step ? step[2] : children}</span>
      {id ? (
        <a
          href={`#${id}`}
          aria-label={`Link to ${text}`}
          className="mt-1.5 text-subtle opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
        >
          <Hash className="size-4" aria-hidden />
        </a>
      ) : null}
    </h2>
  );
}

export function DocsH3({ children, id }: ComponentProps<"h3">) {
  return (
    <h3 id={id} className="mt-6 mb-1.5 text-[19px]/[1.35] font-medium text-fg">
      {children}
    </h3>
  );
}

const LINK =
  "font-[450] text-fg underline decoration-site-light-350 underline-offset-[3px]! transition-colors hover:decoration-brand-strong";

export function DocsLink({
  href = "",
  children,
  className: _className,
  ...props
}: ComponentProps<"a">) {
  if (href.startsWith("/") && !href.startsWith("//")) {
    return (
      <Link href={href} className={LINK}>
        {children}
      </Link>
    );
  }
  const external = /^https?:/.test(href);
  return (
    <a
      {...props}
      href={href}
      className={LINK}
      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
    >
      {children}
    </a>
  );
}

export function DocsInlineCode({ children, className, ...props }: ComponentProps<"code">) {
  // Fenced code reaches here inside a Shiki `pre`, which owns its own styling.
  if (className?.includes("language-") || props.style) {
    return (
      <code className={className} {...props}>
        {children}
      </code>
    );
  }
  return (
    <code className="rounded-[6px] bg-[#1a1a1a] px-1.5 py-px font-mono text-[0.9em] break-words text-fg">
      {children}
    </code>
  );
}

export function DocsTable(props: ComponentProps<"table">) {
  return (
    <div className="my-7 w-full overflow-x-auto rounded-[6px] border border-[#383838]">
      <table
        {...props}
        className="w-full border-collapse text-left [&_td]:border-b [&_td]:border-[#262626] [&_td]:p-3 [&_td]:align-top [&_td]:text-[14.5px] [&_td]:leading-[1.55] [&_th]:border-b [&_th]:border-[#262626] [&_th]:px-3 [&_th]:py-2 [&_th]:text-[13px] [&_th]:font-[450] [&_th]:whitespace-nowrap [&_th]:text-subtle [&_tr:last-child>td]:border-b-0"
      />
    </div>
  );
}

/** One shell command reads as a prompt line: no header, no box inside a box. */
export function CommandLine({ code }: { code: string }) {
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
export function CodeFrame({
  code,
  label,
  shell,
  children,
}: {
  code: string;
  label: string;
  shell: boolean;
  children: ReactNode;
}) {
  return (
    <div className={cn("not-prose my-6 overflow-hidden rounded-[14px] border bg-[#0d0d0d]", LINE)}>
      <div className={cn("flex h-10 items-center justify-between border-b pr-2 pl-4", LINE)}>
        <span className="flex items-center gap-2 font-mono text-[12.5px] text-subtle">
          {shell ? (
            <span aria-hidden className="text-brand-strong">
              $
            </span>
          ) : null}
          {label}
        </span>
        <CopyIconButton value={code} />
      </div>
      {children}
    </div>
  );
}

/** Plain code (no Shiki pass), as the release install commands render. */
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
    <CodeFrame code={code} shell={shell} label={label ?? (shell ? "Terminal" : (language ?? ""))}>
      <pre className="overflow-x-auto px-4 py-3.5 font-mono text-[13.5px]/[1.6] text-fg">
        <code>{code}</code>
      </pre>
    </CodeFrame>
  );
}

function languageOf(children: ReactNode): string | undefined {
  if (!isValidElement(children)) return undefined;
  const className = (children as ReactElement<{ className?: string }>).props.className ?? "";
  const language = /language-([\w-]+)/.exec(className)?.[1];
  return language && !PLAIN.has(language) ? language : undefined;
}

/** Fenced code from the MDX pipeline: Shiki tokens kept, the draft's frame or prompt line around them. */
export function DocsPre({
  children,
  title,
  className,
  style,
}: ComponentProps<"pre"> & { title?: string; icon?: unknown }) {
  const code = textOf(children).replace(/\n$/, "");
  const language = languageOf(children);
  const shell = !language || SHELL.has(language);
  if (shell && language && !title && !code.includes("\n") && !code.startsWith("#")) {
    return <CommandLine code={code} />;
  }
  return (
    <CodeFrame code={code} shell={shell} label={title ?? (shell ? "Terminal" : (language ?? ""))}>
      <pre
        className={cn(
          className,
          "m-0 overflow-x-auto bg-transparent! px-4 py-3.5 font-mono text-[13.5px]/[1.6] [&_.line]:min-h-[1lh] [&_.line]:px-0!"
        )}
        style={style as CSSProperties}
      >
        {children}
      </pre>
    </CodeFrame>
  );
}
