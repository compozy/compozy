import * as React from "react";

import { cn } from "../../lib/utils";
import {
  INLINE_CODE_CLASS,
  PROSE_LINK,
  PROSE_TYPE,
  PROSE_TYPE_COMPACT,
} from "./markdown-prose-constants";

// Each part picks its own density from `data-compact` on the `group/md` root; margins stay in the root recipes.

type MdProps<T extends keyof React.JSX.IntrinsicElements> = React.ComponentPropsWithoutRef<T> & {
  node?: unknown;
};

export function MarkdownAnchor({ className, node: _node, ...props }: MdProps<"a">) {
  return (
    <a
      className={cn(
        PROSE_LINK.text,
        PROSE_LINK.underline,
        "underline decoration-1 underline-offset-[3px] hover:decoration-2",
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
      className={cn(
        "border-l-2 border-line-focus pl-4 text-muted group-data-[compact=true]/md:pl-3",
        className
      )}
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
        PROSE_TYPE.h1,
        PROSE_TYPE_COMPACT.h1,
        "font-semibold tracking-prose-h1 text-balance text-fg-strong group-data-[compact=true]/md:tracking-body",
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
        PROSE_TYPE.h2,
        PROSE_TYPE_COMPACT.h2,
        "font-semibold tracking-prose-h2 text-balance text-fg-strong group-data-[compact=true]/md:tracking-body",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownH3({ className, node: _node, ...props }: MdProps<"h3">) {
  return (
    <h3
      className={cn(
        PROSE_TYPE.h3,
        PROSE_TYPE_COMPACT.h3,
        "font-semibold tracking-prose-h3 text-fg-strong group-data-[compact=true]/md:tracking-body",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownH4({ className, node: _node, ...props }: MdProps<"h4">) {
  return (
    <h4
      className={cn(
        PROSE_TYPE.h4,
        PROSE_TYPE_COMPACT.h4,
        "font-semibold text-fg-strong group-data-[compact=true]/md:font-medium group-data-[compact=true]/md:text-fg",
        className
      )}
      {...props}
    />
  );
}

const LABEL_HEADING_CLASS = cn(
  PROSE_TYPE.label,
  "font-semibold text-muted group-data-[compact=true]/md:font-medium"
);

export function MarkdownH5({ className, node: _node, ...props }: MdProps<"h5">) {
  return <h5 className={cn(LABEL_HEADING_CLASS, className)} {...props} />;
}

export function MarkdownH6({ className, node: _node, ...props }: MdProps<"h6">) {
  return <h6 className={cn(LABEL_HEADING_CLASS, className)} {...props} />;
}

export function MarkdownUl({ className, node: _node, ...props }: MdProps<"ul">) {
  return (
    <ul
      className={cn(
        "list-disc pl-6 marker:text-muted group-data-[compact=true]/md:pl-5 [&_ul]:list-[circle] [&_ul_ul]:list-[square]",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownOl({ className, node: _node, ...props }: MdProps<"ol">) {
  return (
    <ol
      className={cn(
        "list-decimal pl-6 marker:text-muted marker:tabular-nums group-data-[compact=true]/md:pl-5",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownLi({ className, node: _node, ...props }: MdProps<"li">) {
  return (
    <li className={cn("my-1.5 pl-0.5 group-data-[compact=true]/md:pl-0", className)} {...props} />
  );
}

export function MarkdownTable({ className, node: _node, ...props }: MdProps<"table">) {
  return (
    <div
      data-slot="markdown-table"
      className="w-full overflow-x-auto rounded-sm border border-line-strong"
    >
      <table
        className={cn(
          "w-full border-collapse tabular-nums [&_tr:last-child>td]:border-b-0",
          className
        )}
        {...props}
      />
    </div>
  );
}

export function MarkdownTh({ className, node: _node, ...props }: MdProps<"th">) {
  return (
    <th
      className={cn(
        "border-b border-line-strong bg-surface-glaze px-3 py-2 text-left text-form-label font-medium whitespace-nowrap text-fg-strong",
        "group-data-[compact=true]/md:px-2 group-data-[compact=true]/md:py-1.5",
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
        "border-b border-line px-3 py-2 align-top text-small-body [overflow-wrap:normal]",
        "group-data-[compact=true]/md:px-2 group-data-[compact=true]/md:py-1.5 group-data-[compact=true]/md:text-form-input",
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
