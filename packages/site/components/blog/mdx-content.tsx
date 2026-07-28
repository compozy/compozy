import { runSync } from "@mdx-js/mdx";
import * as runtime from "react/jsx-runtime";
import type { ComponentType } from "react";

import { mdxComponents, type MdxComponents } from "./mdx-components";

export interface MdxContentProps {
  code: string;
  components?: Partial<MdxComponents>;
}

export function MdxContent({ code, components }: MdxContentProps) {
  const result = runSync(code, runtime) as {
    default: ComponentType<{ components?: MdxComponents }>;
  };
  const Component = result.default;
  return <Component components={{ ...mdxComponents, ...components } as MdxComponents} />;
}
