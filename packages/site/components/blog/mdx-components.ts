import type { ComponentType } from "react";
import { BlogCodeBlock } from "./code-block";
import { BlogKindChip } from "./kind-chip";
import { MonoBadge } from "./mono-badge";
import {
  BlogWireCard,
  Callout,
  Mono,
  ProseH2,
  ProseH3,
  ProseLink,
  ProseList,
  ProseOrderedList,
  ProseParagraph,
  ProseTable,
  ProseTableCell,
  ProseTableHeader,
  PullQuote,
} from "./prose";

export type MdxComponents = Record<string, ComponentType<Record<string, unknown>>>;

export const mdxComponents = {
  h2: ProseH2,
  h3: ProseH3,
  p: ProseParagraph,
  a: ProseLink,
  table: ProseTable,
  th: ProseTableHeader,
  td: ProseTableCell,
  ul: ProseList,
  ol: ProseOrderedList,
  blockquote: PullQuote,
  code: Mono,
  pre: BlogCodeBlock,
  Callout,
  WireCard: BlogWireCard,
  KindChip: BlogKindChip,
  MonoBadge,
} as unknown as MdxComponents;
