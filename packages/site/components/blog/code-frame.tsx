/**
 * Fenced code in a post, in the shipped CodeBlock's frame as the blog draft sets it: a floating
 * copy button and the language label, no header row. The lines are the ones the build already
 * highlighted, so the colours are in the served HTML and nothing recolours after hydration.
 */
import { CopyIconButton, Eyebrow, cn } from "@compozy/ui";
import type { ComponentProps } from "react";

import { textOf } from "./text-of";

const PLAIN = new Set(["text", "plaintext", "txt"]);

export interface CodeFrameProps extends ComponentProps<"pre"> {
  "data-language"?: string;
}

export function CodeFrame({ children, className: _className, ...props }: CodeFrameProps) {
  const language = props["data-language"];
  const label = language && !PLAIN.has(language) ? language : undefined;
  return (
    <div
      data-slot="code-block"
      data-language={label}
      className="relative my-8 overflow-hidden rounded-lg border border-line bg-code-bg text-[14px]"
    >
      {label ? (
        <Eyebrow data-slot="code-block-language" className="absolute top-3 left-5 text-subtle">
          {label}
        </Eyebrow>
      ) : null}
      <CopyIconButton
        value={textOf(children).replace(/\n$/, "")}
        copyLabel="Copy code"
        className="absolute top-2 right-2 text-subtle hover:text-fg data-[copied=true]:text-success data-[copy-state=failed]:text-destructive data-[copy-state=failed]:hover:text-destructive"
      />
      <pre
        data-slot="code-block-pre"
        className={cn(
          "m-0 overflow-x-auto bg-transparent px-5 py-4 pr-12 font-mono text-code-block whitespace-pre text-fg",
          "[scrollbar-width:thin] [scrollbar-color:var(--color-line-strong)_transparent]",
          label ? "pt-9" : null
        )}
      >
        {children}
      </pre>
    </div>
  );
}
