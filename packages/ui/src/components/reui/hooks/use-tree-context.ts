import type { ItemInstance, TreeInstance } from "@headless-tree/core";
import { createContext, use } from "react";

export type ToggleIconType = "chevron" | "plus-minus";

interface TreeContextValue {
  indent: number;
  currentItem?: unknown;
  tree?: unknown;
  toggleIconType: ToggleIconType;
}

export const TreeContext = createContext<TreeContextValue>({
  indent: 20,
  currentItem: undefined,
  tree: undefined,
  toggleIconType: "plus-minus",
});

export interface TypedTreeContext<T> {
  indent: number;
  currentItem?: ItemInstance<T>;
  tree?: TreeInstance<T>;
  toggleIconType: ToggleIconType;
}

export function useTreeContext<T>(): TypedTreeContext<T> {
  return use(TreeContext) as TypedTreeContext<T>;
}
