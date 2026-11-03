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
import { MarkdownAuthoredAnchor } from "./markdown-authored-anchor";
import { remarkAuthoredLineBreaks } from "./markdown-line-breaks";
import {
  MARKDOWN_LOCAL_LINK_TAG,
  rehypeMarkdownLocalLinks,
  remarkMarkdownLocalLinks,
} from "./markdown-local-links";
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

/** Decorates rendered inline code; receives its literal text and the default element. */
export type StreamMarkdownInlineCodeDecorator = (code: {
  text: string;
  element: React.ReactElement;
}) => React.ReactNode;

/** Renders a link the safe mode would block; receives the authored destination. */
export type StreamMarkdownLocalLinkComponent = React.ComponentType<{
  href: string;
  children: React.ReactNode;
}>;

const StreamMarkdownStreamingContext = React.createContext(false);
const StreamMarkdownInlineCodeContext = React.createContext<
  StreamMarkdownInlineCodeDecorator | undefined
>(undefined);
type RenderLocalLink = (href: string, children: React.ReactNode) => React.ReactNode;
const StreamMarkdownLocalLinkContext = React.createContext<RenderLocalLink | undefined>(undefined);

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
  const decorateInlineCode = React.use(StreamMarkdownInlineCodeContext);
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
  const element = (
    <code className={cn(INLINE_CODE_CLASS, className)} {...props}>
      {children}
    </code>
  );
  return decorateInlineCode ? decorateInlineCode({ text: code, element }) : element;
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

function StreamMarkdownLocalLink({
  "data-href": href,
  children,
}: {
  "data-href"?: string;
  children?: React.ReactNode;
}) {
  const renderLocalLink = React.use(StreamMarkdownLocalLinkContext);
  if (!renderLocalLink || href === undefined) return <>{children}</>;
  return renderLocalLink(href, children);
}

const LOCAL_LINK_COMPONENTS = {
  [MARKDOWN_LOCAL_LINK_TAG]: StreamMarkdownLocalLink,
} as Partial<Components>;

// Streamdown caches parsers by plugin identity, so each combination is built once.
const PLUGIN_CACHE = new Map<string, ReturnType<typeof extendSafeMarkdownPlugins>>();

function streamMarkdownPlugins(slotted: boolean, authored: boolean, localLinks: boolean) {
  if (!slotted && !authored && !localLinks) return undefined;
  const key = `${slotted}:${authored}:${localLinks}`;
  let plugins = PLUGIN_CACHE.get(key);
  if (!plugins) {
    plugins = extendSafeMarkdownPlugins({
      remark: [
        ...(slotted ? [remarkMarkdownSlotLiterals] : []),
        ...(authored ? [remarkAuthoredLineBreaks] : []),
        ...(localLinks ? [remarkMarkdownLocalLinks] : []),
      ],
      rehype: [
        ...(slotted ? [rehypeMarkdownSlots] : []),
        ...(localLinks ? [rehypeMarkdownLocalLinks] : []),
      ],
    });
    PLUGIN_CACHE.set(key, plugins);
  }
  return plugins;
}

const AUTHORED_COMPONENTS: Partial<Components> = { a: MarkdownAuthoredAnchor };

export interface StreamMarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  children: string;
  streaming?: boolean;
  /** Density forwarded to `<Markdown>`; see `MarkdownProps["compact"]`. */
  compact?: MarkdownProps["compact"];
  /** Inline nodes rendered where a `composeMarkdownSlots` source places them. */
  slots?: StreamMarkdownSlots;
  /**
   * Typed chat input rather than model output: each authored line break
   * renders as a line break, and links open externally behind the safe
   * external-link affordance (`MarkdownAuthoredAnchor`).
   */
  authored?: boolean;
  components?: MarkdownProps["components"];
  /**
   * Decorates each inline code span after slot literals are restored: it
   * receives the code's literal text and the default rendered element, and
   * returns what renders in its place.
   */
  inlineCodeDecorator?: StreamMarkdownInlineCodeDecorator;
  /**
   * Renders each link the safe mode would block as `[blocked]`, other than
   * http(s), mailto, and script-capable destinations, with its authored href.
   * Without it those links keep the blocked rendering.
   */
  localLinkComponent?: StreamMarkdownLocalLinkComponent;
}

/**
 * Streaming wrapper around `<Markdown />` for chat threads. Reuses the canonical
 * prose grammar but swaps in the Compozy `<CodeBlock>` for fenced code blocks (with
 * copy + syntax highlighting). Inline code uses the shared Markdown recipe.
 * It carries no reading measure: prose, tables, and code fill the container, so
 * a surface that wants a measure sets it through `className`.
 */
function StreamMarkdown({
  children,
  streaming = false,
  compact = false,
  slots,
  authored = false,
  components,
  inlineCodeDecorator,
  localLinkComponent: LocalLink,
  className,
  ...props
}: StreamMarkdownProps) {
  const renderLocalLink: RenderLocalLink | undefined = LocalLink
    ? (href, linkChildren) => <LocalLink href={href}>{linkChildren}</LocalLink>
    : undefined;
  const markdown = (
    <Markdown
      data-slot="stream-markdown"
      streaming={streaming}
      compact={compact}
      components={{
        ...(slots ? SLOT_COMPONENTS : STREAM_MARKDOWN_COMPONENTS),
        ...(authored ? AUTHORED_COMPONENTS : undefined),
        ...(LocalLink ? LOCAL_LINK_COMPONENTS : undefined),
        ...components,
      }}
      plugins={streamMarkdownPlugins(slots !== undefined, authored, LocalLink !== undefined)}
      className={cn(authored && "[&_:not(pre)>code]:whitespace-pre-wrap", className)}
      {...props}
    >
      {children}
    </Markdown>
  );
  return (
    <StreamMarkdownStreamingContext value={streaming}>
      <StreamMarkdownSlotsContext value={slots ?? NO_SLOTS}>
        <StreamMarkdownInlineCodeContext value={inlineCodeDecorator}>
          <StreamMarkdownLocalLinkContext value={renderLocalLink}>
            {markdown}
          </StreamMarkdownLocalLinkContext>
        </StreamMarkdownInlineCodeContext>
      </StreamMarkdownSlotsContext>
    </StreamMarkdownStreamingContext>
  );
}

export { StreamMarkdown };
