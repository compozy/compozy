import { Fragment } from "react";
import type { PropsWithChildren, ReactElement } from "react";

export interface UIProviderProps extends PropsWithChildren {}

export function UIProvider({ children }: UIProviderProps): ReactElement {
  return <Fragment>{children}</Fragment>;
}
