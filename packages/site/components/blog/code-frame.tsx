/** Fenced code in a post: the shipped CodeBlock with its floating copy button, as the blog draft sets it. */
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
      copyable
      copyLabel="Copy code"
      className="my-8 text-[14px]"
    />
  );
}
