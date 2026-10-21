"use client";

import * as React from "react";
import type { Components } from "streamdown";

import { normalizeCompozyCodeLanguage } from "../../lib/code-theme";
import { cn } from "../../lib/utils";
import { CodeBlock } from "./code-block";
import { MarkdownMermaid } from "./markdown-mermaid";
import { Markdown, type MarkdownProps } from "./markdown";
import { extendSafeMarkdownPlugins } from "./markdown-plugins";
import { INLINE_CODE_CLASS } from "./markdown-components";
import {
  MARKDOWN_SLOT_TAG,
  rehypeMarkdownSlots,
  remarkMarkdownSlotLiterals,
  restoreSlotText,
  type StreamMarkdownSlots,
} from "./markdown-slots";

type StreamMarkdownCodeProps = React.ComponentProps<"code"> & {
  node?: unknown;
  inline?: boolean;
  metastring?: string;
  "data-block"?: unknown;
};

const StreamMarkdownStreamingContext = React.createContext(false);

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
  const streaming = React.use(StreamMarkdownStreamingContext);
  const code = toCodeString(children);
  const rawLanguage = extractCodeLanguage(className);
  const isBlock = dataBlock !== undefined || rawLanguage !== "" || code.includes("\n");

  if (isBlock) {
    if (rawLanguage === "mermaid") return <MarkdownMermaid code={code} streaming={streaming} />;
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

const NO_SLOTS: StreamMarkdownSlots = {};
const StreamMarkdownSlotsContext = React.createContext<StreamMarkdownSlots>(NO_SLOTS);

function StreamMarkdownSlotNode({
  "data-markdown-slot": id,
  "data-markdown-slot-literal": literal,
}: {
  "data-markdown-slot"?: string;
  "data-markdown-slot-literal"?: string;
}) {
  const slots = React.use(StreamMarkdownSlotsContext);
  const slot = id !== undefined && Object.hasOwn(slots, id) ? slots[id] : undefined;
  if (!slot) return null;
  // A link label or raw HTML holds only text, so the slot reads as its literal there.
  return <>{literal === "true" ? slot.text : slot.node}</>;
}

// Code text keeps each slot's sentinel; it reads as the slot's literal text.
function StreamMarkdownSlotCode({ children, ...props }: StreamMarkdownCodeProps) {
  const slots = React.use(StreamMarkdownSlotsContext);
  const restored = React.Children.map(children, child =>
    typeof child === "string" ? restoreSlotText(child, slots) : child
  );
  return <StreamMarkdownCode {...props}>{restored}</StreamMarkdownCode>;
}

const SLOT_COMPONENTS = {
  ...STREAM_MARKDOWN_COMPONENTS,
  code: StreamMarkdownSlotCode,
  inlineCode: StreamMarkdownSlotCode,
  [MARKDOWN_SLOT_TAG]: StreamMarkdownSlotNode,
} as Partial<Components>;

const SLOT_PLUGINS = extendSafeMarkdownPlugins({
  remark: [remarkMarkdownSlotLiterals],
  rehype: [rehypeMarkdownSlots],
});

export interface StreamMarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  children: string;
  streaming?: boolean;
  /** Density forwarded to `<Markdown>`; see `MarkdownProps["compact"]`. */
  compact?: MarkdownProps["compact"];
  /** Inline nodes rendered where a `composeMarkdownSlots` source places them. */
  slots?: StreamMarkdownSlots;
  components?: MarkdownProps["components"];
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
  components,
  className,
  ...props
}: StreamMarkdownProps) {
  const markdown = (
    <Markdown
      data-slot="stream-markdown"
      streaming={streaming}
      compact={compact}
      components={{ ...(slots ? SLOT_COMPONENTS : STREAM_MARKDOWN_COMPONENTS), ...components }}
      plugins={slots ? SLOT_PLUGINS : undefined}
      className={cn("max-w-[72ch]", className)}
      {...props}
    >
      {children}
    </Markdown>
  );
  return (
    <StreamMarkdownStreamingContext value={streaming}>
      <StreamMarkdownSlotsContext value={slots ?? NO_SLOTS}>{markdown}</StreamMarkdownSlotsContext>
    </StreamMarkdownStreamingContext>
  );
}

export { StreamMarkdown };
