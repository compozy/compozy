"use client";

import * as React from "react";
import { Streamdown, type Components } from "streamdown";

import { cn } from "../../lib/utils";
import { STREAMDOWN_SAFE_CONFIG } from "./markdown-config";
import { MARKDOWN_PROSE_COMPONENTS } from "./markdown-components";

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
 * Canonical markdown primitive for the AGH runtime UI. Wraps `streamdown` with
 * the `STREAMDOWN_SAFE_CONFIG` security contract and an explicit prose component
 * map driven by AGH design tokens, so every markdown surface — description cards,
 * chat messages, tool call inputs/outputs — renders against the same grammar.
 *
 * Use `compact` for dense surfaces (tool call panels, inline previews); use
 * `streaming` to opt into streamdown's incremental parser for in-flight model
 * output. Extra `components` are merged on top of the safe-mode defaults.
 */
export interface MarkdownProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Markdown source — operator-authored or model-streamed. */
  children: string;
  /** Reduce vertical margins for dense surfaces (tool inputs/outputs, inline previews). */
  compact?: boolean;
  /** Enable streamdown's incremental parser for partial markdown. */
  streaming?: boolean;
  /** Merge extra component overrides on top of the safe-mode defaults. */
  components?: Partial<Components>;
}

const PROSE_BASE = [
  "text-card-title leading-prose text-fg",
  "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
  "[&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-canvas [&_pre]:p-3 [&_pre]:text-form-input [&_pre]:font-mono",
  "[&_pre_code]:bg-transparent [&_pre_code]:px-0",
].join(" ");

const PROSE_NORMAL = [
  "[&_p]:my-2",
  "[&_blockquote]:my-3 [&_pre]:my-3 [&_table]:my-3",
  "[&_ol]:my-2 [&_ul]:my-2",
  "[&_h1]:mt-5 [&_h1]:mb-2",
  "[&_h2]:mt-5 [&_h2]:mb-2",
  "[&_h3]:mt-4 [&_h3]:mb-1.5",
  "[&_h4]:mt-3 [&_h4]:mb-1",
  "[&_h5]:mt-3 [&_h5]:mb-1",
  "[&_h6]:mt-3 [&_h6]:mb-1",
].join(" ");

const PROSE_COMPACT = [
  "[&_p]:my-1",
  "[&_blockquote]:my-2 [&_pre]:my-2 [&_table]:my-2 [&_hr]:my-2",
  "[&_ol]:my-1 [&_ul]:my-1 [&_li]:my-0",
  "[&_h1]:mt-3 [&_h1]:mb-1",
  "[&_h2]:mt-3 [&_h2]:mb-1",
  "[&_h3]:mt-2.5 [&_h3]:mb-1",
  "[&_h4]:mt-2 [&_h4]:mb-0.5",
  "[&_h5]:mt-2 [&_h5]:mb-0.5",
  "[&_h6]:mt-2 [&_h6]:mb-0.5",
].join(" ");

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
      className={cn(PROSE_BASE, compact ? PROSE_COMPACT : PROSE_NORMAL, className)}
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
