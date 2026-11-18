"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { useHtmlPreviewLinks } from "./hooks/use-html-preview-links";
import { prepareHtmlPreview } from "./html-preview-document";

export interface FileHtmlPreviewProps extends Omit<
  React.ComponentProps<"iframe">,
  "children" | "src" | "srcDoc" | "sandbox"
> {
  /** The HTML document as authored. */
  html: string;
  /** Accessible name of the frame, such as the file name. */
  title: string;
  /**
   * Absolute URL of the file's directory: relative links and assets resolve against it, and its
   * origin is the only one the document may load images, styles, fonts and media from.
   */
  baseUrl?: string;
  /** Rewrites each asset URL (already absolute against `baseUrl`), such as to add a scope query. */
  resolveAssetUrl?: (url: string) => string;
  /** A link in the document was activated; receives its resolved absolute URL. */
  onLinkClick?: (url: string) => void;
}

/**
 * An HTML file rendered in a sandboxed frame. The sandbox grants only `allow-same-origin`, so
 * relative assets load with the app's credentials while scripts, forms, popups and top
 * navigation stay blocked; link clicks never navigate the frame and go to `onLinkClick`.
 */
function FileHtmlPreview({
  html,
  title,
  baseUrl,
  resolveAssetUrl,
  onLinkClick,
  className,
  ...props
}: FileHtmlPreviewProps) {
  const document = React.useMemo(
    () => prepareHtmlPreview(html, baseUrl, resolveAssetUrl),
    [html, baseUrl, resolveAssetUrl]
  );
  const frameRef = React.useRef<HTMLIFrameElement | null>(null);
  useHtmlPreviewLinks(frameRef, onLinkClick);
  return (
    <iframe
      ref={frameRef}
      data-slot="file-html-preview"
      title={title}
      sandbox="allow-same-origin"
      referrerPolicy="no-referrer"
      srcDoc={document}
      className={cn("min-h-0 w-full flex-1 border-0 bg-white", className)}
      {...props}
    />
  );
}

export { FileHtmlPreview };
