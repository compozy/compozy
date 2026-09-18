"use client";

import * as React from "react";
import { Streamdown, type Components } from "streamdown";

import { cn } from "../../lib/utils";
import { STREAMDOWN_SAFE_CONFIG } from "./markdown-config";
import { MARKDOWN_PROSE_COMPONENTS } from "./markdown-components";
import { PROSE_TYPE } from "./markdown-prose-constants";

/**
 * Markdown Safe-Mode Contract.
 *
 * Operator-authored markdown is user input rendered to other operators — an XSS surface.
 * This is the explicit allowlist the runtime ships:
 *
 * - Raw HTML markup is stripped at the parser via `skipHtml: true`.
 * - Output-side security-sensitive elements are blocked via `disallowedElements`.
 * - URL schemes are constrained to https/http/mailto/tel/internal hashes via streamdown's
 *   `defaultUrlTransform` (rehype-harden). `javascript:`, `data:`, `vbscript:`, `file:`,
 *   `about:`, and other schemes are rewritten to a `[blocked]` span.
 * - External images are rewritten to a textual `[image: alt text]` fallback. Relative
 *   URLs (`./`, `../`, `/`) keep rendering through a styled `<img>`.
 *
 * Owned by `<Markdown />` — every markdown surface in the runtime UI (description
 * cards, chat messages, tool-call panels) consumes the same contract.
 */
/**
 * Canonical markdown primitive for the Compozy runtime UI. Wraps `streamdown` with
 * the `STREAMDOWN_SAFE_CONFIG` security contract and an explicit prose component
 * map driven by Compozy design tokens, so every markdown surface — description cards,
 * chat messages, tool call inputs/outputs — renders against the same grammar.
 *
 * Use `compact` for dense surfaces (tool call panels, inline previews); use
 * `streaming` to opt into streamdown's incremental parser for in-flight model
 * output. Extra `components` are merged on top of the safe-mode defaults.
 */
export interface MarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Markdown source — operator-authored or model-streamed. */
  children: string;
  /** Dense surfaces keep the small heading tier; `"relaxed"` pairs it with prose paragraph breaks. */
  compact?: boolean | "relaxed";
  /** Enable streamdown's incremental parser for partial markdown. */
  streaming?: boolean;
  /** Merge extra component overrides on top of the safe-mode defaults. */
  components?: Partial<Components>;
}

const PROSE_BASE = [
  PROSE_TYPE.body,
  "group/md leading-prose text-fg",
  "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
  "[&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-canvas [&_pre]:p-3 [&_pre]:text-form-input [&_pre]:font-mono",
  "[&_pre_code]:bg-transparent [&_pre_code]:px-0",
  "[&_blockquote>:first-child]:mt-0 [&_blockquote>:last-child]:mb-0",
].join(" ");

// Block gaps exceed the prose line gap; headings sit closer to the text they introduce.
const PROSE_NORMAL = [
  "[&_p]:my-3.5 [&_li>p]:my-1.5",
  "[&_blockquote]:my-5 [&_pre]:my-3 [&_[data-slot=markdown-table]]:my-5",
  "[&_ol]:my-3.5 [&_ul]:my-3.5",
  "[&_p:has(+ol)]:mb-2 [&_p:has(+ul)]:mb-2 [&_p+ol]:mt-0 [&_p+ul]:mt-0",
  "[&_li>ol]:mt-1.5 [&_li>ol]:mb-2 [&_li>ul]:mt-1.5 [&_li>ul]:mb-2",
  "[&_h1]:mt-8 [&_h1]:mb-3",
  "[&_h2]:mt-8 [&_h2]:mb-2.5",
  "[&_h3]:mt-6 [&_h3]:mb-1.5",
  "[&_h4]:mt-5 [&_h4]:mb-1",
  "[&_h5]:mt-5 [&_h5]:mb-1",
  "[&_h6]:mt-5 [&_h6]:mb-1",
  "[&_:is(h1,h2,h3,h4,h5)+:is(h2,h3,h4,h5,h6)]:mt-3",
  // Margins collapse to the larger side, so the block after a heading yields to the heading.
  "[&_:is(h1,h2,h3,h4,h5,h6)+:is(p,ol,ul)]:mt-0",
  // Item margins separate items only, so a list's own margin sets its outer gap.
  "[&_:is(ol,ul)>li:first-child]:mt-0 [&_:is(ol,ul)>li:last-child]:mb-0",
].join(" ");

// Dense-surface margins; sizes, weights, and padding switch inside each part via `group/md`.
const PROSE_COMPACT = [
  "[&_p]:my-1",
  "[&_blockquote]:my-2 [&_pre]:my-2 [&_[data-slot=markdown-table]]:my-2 [&_hr]:my-2",
  "[&_[data-slot=code-block]]:my-2",
  "[&_ol]:my-1 [&_ul]:my-1 [&_li]:my-0",
  "[&_h1]:mt-3 [&_h1]:mb-1",
  "[&_h2]:mt-3 [&_h2]:mb-1",
  "[&_h3]:mt-2.5 [&_h3]:mb-1",
  "[&_h4]:mt-2 [&_h4]:mb-0.5",
  "[&_h5]:mt-2 [&_h5]:mb-0.5",
  "[&_h6]:mt-2 [&_h6]:mb-0.5",
].join(" ");

// Small heading tier with prose paragraph breaks, for muted reading panels such as reasoning.
const PROSE_COMPACT_RELAXED = [
  "[&_p]:my-2",
  "[&_blockquote]:my-3 [&_pre]:my-3 [&_[data-slot=markdown-table]]:my-3 [&_hr]:my-4",
  "[&_[data-slot=code-block]]:my-2",
  "[&_ol]:my-2 [&_ul]:my-2 [&_li]:my-0.5",
  "[&_h1]:mt-5 [&_h1]:mb-2",
  "[&_h2]:mt-5 [&_h2]:mb-2",
  "[&_h3]:mt-4 [&_h3]:mb-1.5",
  "[&_h4]:mt-3 [&_h4]:mb-1",
  "[&_h5]:mt-3 [&_h5]:mb-1",
  "[&_h6]:mt-3 [&_h6]:mb-1",
].join(" ");

function proseRecipe(compact: MarkdownProps["compact"]): string {
  if (compact === "relaxed") return PROSE_COMPACT_RELAXED;
  return compact ? PROSE_COMPACT : PROSE_NORMAL;
}

function MarkdownInner({
  children,
  compact = false,
  streaming = false,
  components,
  className,
  ...props
}: MarkdownProps) {
  const mergedComponents: Partial<Components> = {
    ...(STREAMDOWN_SAFE_CONFIG.components as Partial<Components>),
    ...MARKDOWN_PROSE_COMPONENTS,
    ...components,
  };
  const streamingProps = streaming
    ? ({ mode: "streaming" as const, parseIncompleteMarkdown: true } as const)
    : undefined;
  return (
    <div
      data-slot="markdown"
      data-compact={compact ? "true" : undefined}
      data-rhythm={compact === "relaxed" ? "relaxed" : undefined}
      className={cn(PROSE_BASE, proseRecipe(compact), className)}
      {...props}
    >
      <Streamdown {...STREAMDOWN_SAFE_CONFIG} {...streamingProps} components={mergedComponents}>
        {children}
      </Streamdown>
    </div>
  );
}

const Markdown: React.FC<MarkdownProps> = MarkdownInner;
Markdown.displayName = "Markdown";

export { Markdown };
export { STREAMDOWN_SAFE_CONFIG };
