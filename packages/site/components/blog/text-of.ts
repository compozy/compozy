import { isValidElement, type ReactNode } from "react";

/** The plain text of a rendered node tree: heading anchors and copy buttons read it. */
export function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children);
  return "";
}
