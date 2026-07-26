"use client";

import type { Components } from "streamdown";

import {
  INLINE_CODE_CLASS,
  MarkdownAnchor,
  MarkdownBlockquote,
  MarkdownH1,
  MarkdownH2,
  MarkdownH3,
  MarkdownH4,
  MarkdownH5,
  MarkdownH6,
  MarkdownHr,
  MarkdownInlineCode,
  MarkdownLi,
  MarkdownOl,
  MarkdownParagraph,
  MarkdownStrong,
  MarkdownTable,
  MarkdownTd,
  MarkdownTh,
  MarkdownUl,
} from "./markdown-prose-parts";

const MARKDOWN_PROSE_COMPONENTS: Partial<Components> = {
  a: MarkdownAnchor,
  strong: MarkdownStrong,
  blockquote: MarkdownBlockquote,
  hr: MarkdownHr,
  p: MarkdownParagraph,
  h1: MarkdownH1,
  h2: MarkdownH2,
  h3: MarkdownH3,
  h4: MarkdownH4,
  h5: MarkdownH5,
  h6: MarkdownH6,
  ul: MarkdownUl,
  ol: MarkdownOl,
  li: MarkdownLi,
  table: MarkdownTable,
  th: MarkdownTh,
  td: MarkdownTd,
  code: MarkdownInlineCode,
  inlineCode: MarkdownInlineCode,
};

export { INLINE_CODE_CLASS, MARKDOWN_PROSE_COMPONENTS };
