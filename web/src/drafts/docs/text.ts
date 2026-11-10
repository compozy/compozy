/** Text helpers shared by the prose mapper and the page templates. */
import type { ReactNode } from "react";

export function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (node && typeof node === "object" && "props" in node)
    return textOf((node.props as { children?: ReactNode }).children);
  return "";
}

export const BEAM = "bg-[linear-gradient(180deg,#1df0d7,#1f87fc)]";
export const BEAM_X = "bg-[linear-gradient(90deg,#1df0d7,#1f87fc)]";
export const LINE = "border-[color:var(--light-080)]";
export const FOCUS = "outline-none focus-visible:ring-2 focus-visible:ring-brand";
/** The fixed header is 104px tall: 56 for the bar, 48 for the section tabs. */
export const BELOW_HEADER = "top-[104px]";
