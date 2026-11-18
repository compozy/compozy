import type { MouseEvent } from "react";
import { useMemo } from "react";

import type { FilesRootRef } from "../adapters/files-api";
import { isExternalLink, openExternalLink, resolveRelativePath } from "../lib/file-preview";

/** Where a previewed Markdown document's links and images resolve, and how a click follows one. */
export interface MarkdownLinkScope {
  root: FilesRootRef;
  path: string;
  follow: (href: string, event: MouseEvent) => void;
}

/**
 * Markdown links and images follow the file's root: in-page anchors scroll, external links open
 * in the browser, and local targets open in the viewer.
 */
export function useMarkdownLinkScope(
  root: FilesRootRef,
  path: string,
  onOpenFile: ((path: string) => void) | undefined
): MarkdownLinkScope {
  return useMemo<MarkdownLinkScope>(
    () => ({
      root,
      path,
      follow: (href, event) => {
        if (href.startsWith("#")) return;
        event.preventDefault();
        if (isExternalLink(href)) {
          openExternalLink(href);
          return;
        }
        const target = resolveRelativePath(path, href);
        if (target) onOpenFile?.(target);
      },
    }),
    [root, path, onOpenFile]
  );
}
