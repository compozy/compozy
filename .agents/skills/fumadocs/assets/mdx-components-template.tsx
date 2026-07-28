// Canonical `mdx-components.tsx` for a Fumadocs project.
//
// Lives at the repo root for Next.js (consumed by `useMDXComponents`) and
// at `app/components/mdx.tsx` for React Router / TanStack Start / Waku.
//
// Why this shape:
//   1. Spreads `defaultComponents` from `fumadocs-ui/mdx` so Cards, Callouts,
//      Heading anchors, table styling, etc. work out of the box.
//   2. Maps `pre` onto `<CodeBlock><Pre /></CodeBlock>` so MDX code blocks pick
//      up Fumadocs' Shiki rendering, with the React 19 `ref` prop stripped
//      (otherwise `forwardRef` breaks).
//   3. Maps `img` onto `<ImageZoom />` for click-to-zoom.
//   4. Exposes `getMDXComponents(extra)` so per-page consumers can layer in
//      `createRelativeLink(source, page)` for `[link](./other.mdx)` resolution.

import defaultComponents from "fumadocs-ui/mdx";
import { CodeBlock, Pre } from "fumadocs-ui/components/codeblock";
import * as TabsComponents from "fumadocs-ui/components/tabs";
import { ImageZoom } from "fumadocs-ui/components/image-zoom";
import type { MDXComponents } from "mdx/types";

export function getMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...defaultComponents,
    ...TabsComponents,

    // React 19 forwards `ref` as a normal prop on `pre` — strip it before
    // spreading into CodeBlock, otherwise `forwardRef` breaks.
    pre: ({ ref: _ref, ...props }) => (
      <CodeBlock {...props}>
        <Pre>{props.children}</Pre>
      </CodeBlock>
    ),

    // Click-to-zoom for images. On Next.js, ImageZoom injects a default `sizes`
    // when missing.
    img: (props) => <ImageZoom {...(props as any)} />,

    // Add project-specific components here:
    // Mermaid,
    // Callout,
    // Workflow,
    // WorkflowStep,
    // ...

    // User-provided overrides win.
    ...components,
  } satisfies MDXComponents;
}

// Next.js page convention — mounts MDX defaults globally.
export const useMDXComponents = getMDXComponents;

// Per-page usage with relative MDX link resolution (Server Components only):
//
//   import { createRelativeLink } from 'fumadocs-ui/mdx';
//   import { source } from '@/lib/source';
//   import { getMDXComponents } from '@/mdx-components';
//
//   export default async function Page({ params }: PageProps) {
//     const page = source.getPage(params.slug);
//     if (!page) notFound();
//     const MDX = page.data.body;
//     return (
//       <MDX
//         components={getMDXComponents({
//           a: createRelativeLink(source, page),
//         })}
//       />
//     );
//   }
