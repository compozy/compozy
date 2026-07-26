"use client";

import * as React from "react";
import type { Components } from "streamdown";

import { normalizeAghCodeLanguage } from "../../lib/code-theme";
import { cn } from "../../lib/utils";
import { CodeBlock } from "./code-block";
import { Markdown } from "./markdown";
import { INLINE_CODE_CLASS } from "./markdown-components";

type StreamMarkdownCodeProps = React.ComponentProps<"code"> & {
  node?: unknown;
  inline?: boolean;
  metastring?: string;
  "data-block"?: unknown;
};

function extractCodeLanguage(className?: string): string {
  const match = /language-([-\w]+)/.exec(className ?? "");
  return match?.[1]?.toLowerCase() ?? "";
}

function toCodeString(children: React.ReactNode): string {
  return React.Children.toArray(children).join("").replace(/\n$/, "");
}

function StreamMarkdownCode({
  children,
  className,
  node: _node,
  inline: _inline,
  metastring: _metastring,
  "data-block": dataBlock,
  ...props
}: StreamMarkdownCodeProps) {
  const code = toCodeString(children);
  const rawLanguage = extractCodeLanguage(className);
  const isBlock = dataBlock !== undefined || rawLanguage !== "" || code.includes("\n");

  if (isBlock) {
    const normalizedLanguage = normalizeAghCodeLanguage(rawLanguage);
    return (
      <CodeBlock
        code={code}
        language={rawLanguage || undefined}
        caption={rawLanguage ? (normalizedLanguage ?? rawLanguage) : undefined}
        showPrompt={false}
        copyable
        density="compact"
        className="my-2"
      />
    );
  }

  // Inline code uses the shared Markdown recipe (one grammar, both entry points).
  return (
    <code className={cn(INLINE_CODE_CLASS, className)} {...props}>
      {children}
    </code>
  );
}

const STREAM_MARKDOWN_COMPONENTS: Partial<Components> = {
  code: StreamMarkdownCode,
  inlineCode: StreamMarkdownCode,
};

export interface StreamMarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  children: string;
  streaming?: boolean;
}

/**
 * Streaming wrapper around `<Markdown />` for chat threads. Reuses the canonical
 * prose grammar but swaps in the AGH `<CodeBlock>` for fenced code blocks (with
 * copy + syntax highlighting). Inline code uses the shared Markdown recipe.
 */
function StreamMarkdown({ children, streaming = false, className, ...props }: StreamMarkdownProps) {
  return (
    <Markdown
      data-slot="stream-markdown"
      streaming={streaming}
      components={STREAM_MARKDOWN_COMPONENTS}
      className={cn("max-w-[72ch]", className)}
      {...props}
    >
      {children}
    </Markdown>
  );
}

export { StreamMarkdown };
