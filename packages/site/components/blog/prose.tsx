/** The reading column: MDX elements set as long-form type, ported from the blog draft. */
import { cn } from "@compozy/ui";
import type { ComponentProps, ReactNode } from "react";

import { textOf } from "./text-of";

function slugifyHeading(heading: string): string {
  return heading
    .replace(/<[^>]+>/g, "")
    .replace(/`([^`]+)`/g, "$1")
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/\s+/g, "-");
}

function headingId(id: string | undefined, children: ReactNode): string | undefined {
  if (id && id !== "$undefined") return id;
  const generated = slugifyHeading(textOf(children));
  return generated.length > 0 ? generated : undefined;
}

export function ProseH2({ id, children, className, ...props }: ComponentProps<"h2">) {
  return (
    <h2
      id={headingId(id, children)}
      {...props}
      className={cn(
        "mt-12 mb-2.5 scroll-mt-28 font-display text-[30px]/[1.15] font-normal tracking-[-0.02em] text-balance text-fg md:text-[34px]/[1.15]",
        className
      )}
    >
      {children}
    </h2>
  );
}

export function ProseH3({ id, children, className, ...props }: ComponentProps<"h3">) {
  return (
    <h3
      id={headingId(id, children)}
      {...props}
      className={cn("mt-8 mb-1.5 scroll-mt-28 text-[21px]/[1.3] font-medium text-fg", className)}
    >
      {children}
    </h3>
  );
}

export function ProseLink({ className, ...props }: ComponentProps<"a">) {
  return (
    <a
      {...props}
      className={cn(
        "text-fg-2 underline decoration-fg-2 decoration-1 underline-offset-[3px] hover:decoration-2",
        className
      )}
    />
  );
}

export function ProseList({ className, ...props }: ComponentProps<"ul">) {
  return <ul {...props} className={cn("list-disc pl-6 marker:text-muted-foreground", className)} />;
}

export function ProseOrderedList({ className, ...props }: ComponentProps<"ol">) {
  return (
    <ol
      {...props}
      className={cn(
        "list-decimal pl-6 marker:text-muted-foreground marker:tabular-nums",
        className
      )}
    />
  );
}

export function ProseListItem({ className, ...props }: ComponentProps<"li">) {
  return <li {...props} className={cn("pl-0.5", className)} />;
}

export function ProseBlockquote({ className, ...props }: ComponentProps<"blockquote">) {
  return (
    <blockquote
      {...props}
      className={cn(
        "my-8 border-l-2 border-site-light-250 pl-5 text-muted-foreground [&>:first-child]:mt-0 [&>:last-child]:mb-0",
        className
      )}
    />
  );
}

/** The highlighter's wrapper around fenced code; the frame owns the spacing. */
export function ProseFigure({ className, ...props }: ComponentProps<"figure">) {
  return <figure {...props} className={cn("m-0", className)} />;
}

export function ProseRule({ className, ...props }: ComponentProps<"hr">) {
  return <hr {...props} className={cn("my-12 border-site-light-080", className)} />;
}

/** Tables keep their columns and scroll inside the reading column instead of the page. */
export function ProseTable({ className, ...props }: ComponentProps<"table">) {
  return (
    <div
      data-slot="blog-table"
      role="region"
      aria-label="Table"
      tabIndex={0}
      className="my-8 w-full overflow-x-auto rounded-sm border border-line-strong outline-none [scrollbar-width:thin] focus-visible:ring-2 focus-visible:ring-brand"
    >
      <table
        {...props}
        className={cn(
          "w-full min-w-[560px] border-collapse tabular-nums [&_tr:last-child>td]:border-b-0",
          className
        )}
      />
    </div>
  );
}

export function ProseTableHeader({ className, ...props }: ComponentProps<"th">) {
  return (
    <th
      scope="col"
      {...props}
      className={cn(
        "border-b border-line px-3 py-2 text-left text-[15px] font-medium text-muted-foreground",
        className
      )}
    />
  );
}

export function ProseTableCell({ className, ...props }: ComponentProps<"td">) {
  return (
    <td
      {...props}
      className={cn(
        "border-b border-line px-3 py-2 align-top text-[15px] leading-[1.6]",
        className
      )}
    />
  );
}

export interface InlineCodeProps extends ComponentProps<"code"> {
  "data-language"?: string;
  "data-theme"?: string;
}

/** Inline code gets a chip; code inside a highlighted block keeps the highlighter's tokens. */
export function ProseCode({ className, children, ...props }: InlineCodeProps) {
  if (props["data-language"] || props["data-theme"]) {
    return (
      <code {...props} className={cn("font-mono text-inherit", className)}>
        {children}
      </code>
    );
  }
  return (
    <code
      {...props}
      className={cn(
        "rounded-[6px] bg-site-light-080 px-1.5 py-0.5 font-mono text-[0.86em] text-fg",
        className
      )}
    >
      {children}
    </code>
  );
}

export interface CalloutProps {
  tone?: "brand" | "success" | "destructive" | "warning" | "info";
  eyebrow?: string;
  children?: ReactNode;
}

const TONE: Partial<Record<NonNullable<CalloutProps["tone"]>, string>> = {
  warning: "border-warning/40 [&_.callout-label]:text-warning",
  destructive: "border-destructive/45 [&_.callout-label]:text-destructive",
  brand: "border-brand/45 [&_.callout-label]:text-brand-strong",
  success: "border-success/40 [&_.callout-label]:text-success",
};

export function Callout({ tone = "brand", eyebrow, children }: CalloutProps) {
  return (
    <aside
      role="note"
      className={cn(
        "my-9 rounded-[16px] border bg-site-light-025 p-6",
        TONE[tone] ?? "border-site-light-120 [&_.callout-label]:text-fg"
      )}
    >
      {eyebrow ? (
        <p className="callout-label m-0! mb-2! text-[14px] font-medium">{eyebrow}</p>
      ) : null}
      <div className="text-[16.5px] leading-[1.7] text-fg/85 [&_p]:my-3.5! [&>:first-child]:mt-0! [&>:last-child]:mb-0! [&_strong]:text-fg">
        {children}
      </div>
    </aside>
  );
}
