"use client";

import * as React from "react";
import type { Components } from "streamdown";

import { normalizeCompozyCodeLanguage } from "../../lib/code-theme";
import { cn } from "../../lib/utils";
import { CodeBlock } from "./code-block";
import { Markdown, type MarkdownProps } from "./markdown";
import { INLINE_CODE_CLASS } from "./markdown-components";
import { MarkdownAnchor } from "./markdown-prose-parts";
import type { StreamMarkdownSlots } from "./markdown-slots";

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
    const normalizedLanguage = normalizeCompozyCodeLanguage(rawLanguage);
    return (
      <CodeBlock
        code={code}
        language={rawLanguage || undefined}
        caption={rawLanguage ? (normalizedLanguage ?? rawLanguage) : undefined}
        copyable
        density="compact"
        className="my-5"
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

function slotComponents(slots: StreamMarkdownSlots): Partial<Components> {
  function StreamMarkdownSlotAnchor(props: React.ComponentProps<typeof MarkdownAnchor>) {
    const id =
      typeof props.href === "string" && props.href.startsWith("#") ? props.href.slice(1) : "";
    if (id !== "" && Object.hasOwn(slots, id)) {
      return <>{slots[id]}</>;
    }
    return <MarkdownAnchor {...props} />;
  }
  return { ...STREAM_MARKDOWN_COMPONENTS, a: StreamMarkdownSlotAnchor };
}

export interface StreamMarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  children: string;
  streaming?: boolean;
  /** Density forwarded to `<Markdown>`; see `MarkdownProps["compact"]`. */
  compact?: MarkdownProps["compact"];
  /** Inline nodes rendered where the source places them with `markdownSlotSource`. */
  slots?: StreamMarkdownSlots;
}

/**
 * Streaming wrapper around `<Markdown />` for chat threads. Reuses the canonical
 * prose grammar but swaps in the Compozy `<CodeBlock>` for fenced code blocks (with
 * copy + syntax highlighting). Inline code uses the shared Markdown recipe.
 */
function StreamMarkdown({
  children,
  streaming = false,
  compact = false,
  slots,
  className,
  ...props
}: StreamMarkdownProps) {
  const components = React.useMemo(
    () => (slots ? slotComponents(slots) : STREAM_MARKDOWN_COMPONENTS),
    [slots]
  );
  return (
    <Markdown
      data-slot="stream-markdown"
      streaming={streaming}
      compact={compact}
      components={components}
      className={cn("max-w-[72ch]", className)}
      {...props}
    >
      {children}
    </Markdown>
  );
}

export { StreamMarkdown };
