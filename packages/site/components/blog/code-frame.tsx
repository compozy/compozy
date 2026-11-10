/** Fenced code in a post: a header row for the language and copy, then the highlighted lines. */
import { CopyIconButton, cn } from "@compozy/ui";
import type { ComponentProps } from "react";

import { textOf } from "./text-of";

const PLAIN = new Set(["text", "plaintext", "txt"]);

export interface CodeFrameProps extends ComponentProps<"pre"> {
  "data-language"?: string;
}

export function CodeFrame({ className, children, ...props }: CodeFrameProps) {
  const language = props["data-language"];
  const label = language && !PLAIN.has(language) ? language : null;
  const source = textOf(children).replace(/\n$/, "");
  return (
    <div
      data-slot="blog-code"
      className="my-8 overflow-hidden rounded-lg border border-line bg-code-bg text-[14px]"
    >
      <div className="flex h-10 items-center justify-between gap-4 border-b border-site-light-080 pr-2 pl-5">
        <span className="font-mono text-[12px] text-subtle">{label}</span>
        <CopyIconButton
          value={source}
          copyLabel="Copy code"
          className="text-subtle hover:text-fg data-[copied=true]:text-success data-[copy-state=failed]:text-destructive"
        />
      </div>
      <pre
        {...props}
        className={cn(
          "m-0 overflow-x-auto bg-transparent px-5 py-4 font-mono text-[14px] leading-[1.6] whitespace-pre text-fg",
          "[scrollbar-width:thin] [scrollbar-color:var(--color-line-strong)_transparent]",
          className
        )}
      >
        {children}
      </pre>
    </div>
  );
}
