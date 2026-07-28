import * as React from "react";

import { cn } from "../../lib/utils";

type MdProps<T extends keyof React.JSX.IntrinsicElements> = React.ComponentPropsWithoutRef<T> & {
  node?: unknown;
};

export const INLINE_CODE_CLASS =
  "rounded-xs bg-surface-glaze px-1 py-px font-mono text-form-input text-fg-strong";

export function MarkdownAnchor({ className, node: _node, ...props }: MdProps<"a">) {
  return (
    <a
      className={cn(
        "text-fg-strong underline underline-offset-2 decoration-line hover:decoration-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownStrong({ className, node: _node, ...props }: MdProps<"strong">) {
  return <strong className={cn("font-medium text-fg-strong", className)} {...props} />;
}

export function MarkdownBlockquote({ className, node: _node, ...props }: MdProps<"blockquote">) {
  return (
    <blockquote
      className={cn("border-l-2 border-line-strong pl-3 text-muted", className)}
      {...props}
    />
  );
}

export function MarkdownHr({ className, node: _node, ...props }: MdProps<"hr">) {
  return <hr className={cn("my-4 border-line", className)} {...props} />;
}

export function MarkdownParagraph({ className, node: _node, ...props }: MdProps<"p">) {
  return <p className={cn(className)} {...props} />;
}

export function MarkdownH1({ className, node: _node, ...props }: MdProps<"h1">) {
  return (
    <h1 className={cn("text-item-title font-semibold text-fg-strong", className)} {...props} />
  );
}

export function MarkdownH2({ className, node: _node, ...props }: MdProps<"h2">) {
  return (
    <h2 className={cn("text-card-title font-semibold text-fg-strong", className)} {...props} />
  );
}

export function MarkdownH3({ className, node: _node, ...props }: MdProps<"h3">) {
  return (
    <h3 className={cn("text-small-body font-semibold text-fg-strong", className)} {...props} />
  );
}

export function MarkdownH4({ className, node: _node, ...props }: MdProps<"h4">) {
  return <h4 className={cn("text-small-body font-medium text-fg", className)} {...props} />;
}

export function MarkdownH5({ className, node: _node, ...props }: MdProps<"h5">) {
  return <h5 className={cn("text-small-body font-medium text-muted", className)} {...props} />;
}

export function MarkdownH6({ className, node: _node, ...props }: MdProps<"h6">) {
  return <h6 className={cn("text-small-body font-medium text-muted", className)} {...props} />;
}

export function MarkdownUl({ className, node: _node, ...props }: MdProps<"ul">) {
  return (
    <ul
      className={cn("list-disc pl-5 [&_ul]:list-[circle] [&_ul_ul]:list-[square]", className)}
      {...props}
    />
  );
}

export function MarkdownOl({ className, node: _node, ...props }: MdProps<"ol">) {
  return <ol className={cn("list-decimal pl-5", className)} {...props} />;
}

export function MarkdownLi({ className, node: _node, ...props }: MdProps<"li">) {
  return <li className={cn("my-0.5", className)} {...props} />;
}

export function MarkdownTable({ className, node: _node, ...props }: MdProps<"table">) {
  return (
    <div className="w-full overflow-x-auto">
      <table className={cn("w-full border-collapse", className)} {...props} />
    </div>
  );
}

export function MarkdownTh({ className, node: _node, ...props }: MdProps<"th">) {
  return (
    <th
      className={cn(
        "border-b border-line-strong px-2 py-1.5 text-left text-form-label font-medium text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function MarkdownTd({ className, node: _node, ...props }: MdProps<"td">) {
  return (
    <td className={cn("border-b border-line px-2 py-1.5 text-form-input", className)} {...props} />
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
