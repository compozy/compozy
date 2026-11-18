/**
 * Fenced code in a post: the shared CodeBlock with its floating copy button and language label, as
 * the blog draft sets it. The lines are the ones the build already highlighted, passed through as
 * they are, so the colours are in the served HTML and nothing recolours after hydration.
 */
import { CodeBlock } from "@compozy/ui";
import type { ComponentProps } from "react";

import { textOf } from "./text-of";

const PLAIN = new Set(["text", "plaintext", "txt"]);

export interface CodeFrameProps extends ComponentProps<"pre"> {
  "data-language"?: string;
}

export function CodeFrame({ children, ...props }: CodeFrameProps) {
  const language = props["data-language"];
  return (
    <CodeBlock
      code={textOf(children).replace(/\n$/, "")}
      language={language && !PLAIN.has(language) ? language : undefined}
      highlighted={children}
      copyLabel="Copy code"
      className="my-8 text-[14px]"
    />
  );
}
