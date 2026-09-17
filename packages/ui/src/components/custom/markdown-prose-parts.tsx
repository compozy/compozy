import * as React from "react";

import { cn } from "../../lib/utils";
import { INLINE_CODE_CLASS } from "./markdown-prose-constants";

type MdProps<T extends keyof React.JSX.IntrinsicElements> = React.ComponentPropsWithoutRef<T> & {
  node?: unknown;
};

export function MarkdownAnchor({ className, node: _node, ...props }: MdProps<"a">) {
  return (
    <a
      className={cn(
        "text-accent-strong underline decoration-1 underline-offset-[3px] decoration-accent-strong/45 transition-colors duration-base ease-out hover:decoration-accent-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownStrong({ className, node: _node, ...props }: MdProps<"strong">) {
  return <strong className={cn("font-semibold text-fg-strong", className)} {...props} />;
}

export function MarkdownBlockquote({ className, node: _node, ...props }: MdProps<"blockquote">) {
  return (
    <blockquote
      className={cn("border-l-2 border-line-focus pl-4 text-muted", className)}
      {...props}
    />
  );
}

export function MarkdownHr({ className, node: _node, ...props }: MdProps<"hr">) {
  return <hr className={cn("my-8 border-line-strong", className)} {...props} />;
}

export function MarkdownParagraph({ className, node: _node, ...props }: MdProps<"p">) {
  return <p className={cn(className)} {...props} />;
}

export function MarkdownH1({ className, node: _node, ...props }: MdProps<"h1">) {
  return (
    <h1
      className={cn(
        "text-prose-h1 font-semibold tracking-prose-h1 text-balance text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownH2({ className, node: _node, ...props }: MdProps<"h2">) {
  return (
    <h2
      className={cn(
        "text-prose-h2 font-semibold tracking-prose-h2 text-balance text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownH3({ className, node: _node, ...props }: MdProps<"h3">) {
  return (
    <h3
      className={cn("text-prose-h3 font-semibold tracking-prose-h3 text-fg-strong", className)}
      {...props}
    />
  );
}

export function MarkdownH4({ className, node: _node, ...props }: MdProps<"h4">) {
  return (
    <h4 className={cn("text-card-title font-semibold text-fg-strong", className)} {...props} />
  );
}

export function MarkdownH5({ className, node: _node, ...props }: MdProps<"h5">) {
  return <h5 className={cn("text-small-body font-semibold text-muted", className)} {...props} />;
}

export function MarkdownH6({ className, node: _node, ...props }: MdProps<"h6">) {
  return <h6 className={cn("text-small-body font-semibold text-muted", className)} {...props} />;
}

export function MarkdownUl({ className, node: _node, ...props }: MdProps<"ul">) {
  return (
    <ul
      className={cn(
        "list-disc pl-6 marker:text-muted [&_ul]:list-[circle] [&_ul_ul]:list-[square]",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownOl({ className, node: _node, ...props }: MdProps<"ol">) {
  return (
    <ol className={cn("list-decimal pl-6 tabular-nums marker:text-muted", className)} {...props} />
  );
}

export function MarkdownLi({ className, node: _node, ...props }: MdProps<"li">) {
  return <li className={cn("my-1.5 pl-0.5", className)} {...props} />;
}

export function MarkdownTable({ className, node: _node, ...props }: MdProps<"table">) {
  return (
    <div
      data-slot="markdown-table"
      className="w-full overflow-x-auto rounded-sm border border-line-strong"
    >
      <table className={cn("w-full border-collapse tabular-nums", className)} {...props} />
    </div>
  );
}

export function MarkdownTh({ className, node: _node, ...props }: MdProps<"th">) {
  return (
    <th
      className={cn(
        "border-b border-line-strong bg-surface-glaze px-3 py-2 text-left text-form-label font-medium whitespace-nowrap text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownTd({ className, node: _node, ...props }: MdProps<"td">) {
  return (
    <td
      className={cn(
        "min-w-[9ch] border-b border-line px-3 py-2 align-top text-small-body [overflow-wrap:normal] [&_code]:whitespace-nowrap",
        className
      )}
      {...props}
    />
  );
}

type CodeProps = MdProps<"code"> & { inline?: boolean; "data-block"?: unknown };

export function MarkdownInlineCode({
  className,
  children,
  inline: _inline,
  "data-block": dataBlock,
  node: _node,
  ...props
}: CodeProps) {
  const isBlock =
    dataBlock !== undefined ||
    (typeof className === "string" && className.includes("language-")) ||
    (typeof children === "string" && children.includes("\n"));
  if (isBlock) {
    return (
      <code
        className={className}
        {...(dataBlock !== undefined
          ? { "data-block": dataBlock === true ? true : dataBlock }
          : {})}
        {...props}
      >
        {children}
      </code>
    );
  }
  return (
    <code className={cn(INLINE_CODE_CLASS, className)} {...props}>
      {children}
    </code>
  );
}
