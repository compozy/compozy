import { Fragment, createElement } from "react";
import type { PropsWithChildren, ReactElement } from "react";

export interface UIProviderProps extends PropsWithChildren {}

export function UIProvider({ children }: UIProviderProps): ReactElement {
  return createElement(Fragment, null, children);
}
