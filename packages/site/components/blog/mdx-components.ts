import type { ComponentType } from "react";
import { CodeFrame } from "./code-frame";
import {
  Callout,
  ProseBlockquote,
  ProseCode,
  ProseFigure,
  ProseH2,
  ProseH3,
  ProseLink,
  ProseList,
  ProseListItem,
  ProseOrderedList,
  ProseRule,
  ProseTable,
  ProseTableCell,
  ProseTableHeader,
} from "./prose";

export type MdxComponents = Record<string, ComponentType<Record<string, unknown>>>;

export const mdxComponents = {
  h2: ProseH2,
  h3: ProseH3,
  a: ProseLink,
  table: ProseTable,
  th: ProseTableHeader,
  td: ProseTableCell,
  ul: ProseList,
  ol: ProseOrderedList,
  li: ProseListItem,
  blockquote: ProseBlockquote,
  hr: ProseRule,
  code: ProseCode,
  pre: CodeFrame,
  figure: ProseFigure,
  Callout,
} as unknown as MdxComponents;
