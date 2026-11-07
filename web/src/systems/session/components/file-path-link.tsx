import {
  use,
  type ComponentProps,
  type MouseEvent,
  type ReactElement,
  type ReactNode,
} from "react";

import { useQuery } from "@tanstack/react-query";

import { Kbd, MarkdownAnchor, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { openTargetsOptions } from "@/systems/files";
import { lastOpenTarget } from "@/systems/os";

import type { FilePathLinks } from "../hooks/use-file-path-links";
import { useFilePathEditorOpen } from "../hooks/use-file-path-editor-open";
import { detectFilePath, filePathBasename, type FilePathToken } from "../lib/file-path-links";
import { FilePathInsideLinkContext, FilePathLinksContext } from "../lib/file-path-links-context";

// The path reads as a link: a dotted underline on hover and focus, with the focus ring.
const PATH_LINK_CLASS =
  "inline cursor-pointer appearance-none border-0 bg-transparent p-0 text-left align-baseline font-[inherit] text-[length:inherit] leading-[inherit] text-inherit rounded-xs hover:underline hover:decoration-dotted hover:decoration-current focus-visible:underline focus-visible:outline-hidden focus-visible:shadow-focus-ring";

function modifierPressed(event: MouseEvent, modifier: "meta" | "ctrl"): boolean {
  return modifier === "meta" ? event.metaKey : event.ctrlKey;
}

/** A drag that selected text inside the path is a selection, not a click. */
function selectedWithin(element: Element): boolean {
  const selection = window.getSelection();
  return (
    selection !== null &&
    !selection.isCollapsed &&
    selection.anchorNode !== null &&
    element.contains(selection.anchorNode)
  );
}

/**
 * A transcript path (US-005): a plain click opens it in the session's viewer, and in the desktop
 * app Cmd/Ctrl-click opens it in the editor. Without a session root (user scope) only the
 * desktop modifier click remains, and the path otherwise reads as plain code.
 */
function FilePathAffordance({
  links,
  token,
  children,
}: {
  links: FilePathLinks;
  token: FilePathToken;
  children: ReactNode;
}) {
  const openEditor = useFilePathEditorOpen(links.editor, token);
  const { editor, openInViewer } = links;
  const content = <FilePathInsideLinkContext value>{children}</FilePathInsideLinkContext>;
  const onClick = (event: MouseEvent<HTMLElement>) => {
    if (editor && modifierPressed(event, editor.modifier)) {
      event.preventDefault();
      event.stopPropagation();
      openEditor();
      return;
    }
    if (!openInViewer || selectedWithin(event.currentTarget)) return;
    event.preventDefault();
    openInViewer(token);
  };
  if (!openInViewer) {
    return (
      <span data-file-path={token.raw} onClick={onClick} role="presentation">
        {content}
      </span>
    );
  }
  const name = filePathBasename(token.path, { windows: links.windows });
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            aria-label={`Open ${name} in viewer`}
            className={PATH_LINK_CLASS}
            data-file-path={token.raw}
            data-testid="file-path-link"
            onClick={onClick}
          />
        }
      >
        {content}
      </TooltipTrigger>
      <TooltipContent className="gap-1.5">
        Open in viewer
        {editor ? <ModifierClickHint modifier={editor.modifier} /> : null}
      </TooltipContent>
    </Tooltip>
  );
}

/** The Cmd/Ctrl-click target the transcript link opens in (US-005.AC-5): "⌘-click opens in Cursor". */
function ModifierClickHint({ modifier }: { modifier: "meta" | "ctrl" }) {
  const { data: targets = [] } = useQuery(openTargetsOptions());
  const remembered = lastOpenTarget();
  const target = targets.find(item => item.id === remembered);
  // An unset editor command opens in the system's default editor.
  const name = target ? target.label : remembered === "configured" ? "your default editor" : null;
  return (
    <>
      {" · "}
      <Kbd>{modifier === "meta" ? "⌘-click" : "Ctrl-click"}</Kbd> opens in{" "}
      {name ?? "the last-used app"}
    </>
  );
}

/**
 * `StreamMarkdown` inline-code decorator body: path spans gain the viewer link, except
 * inside a link, which keeps its own click behavior.
 */
export function FilePathInlineCode({ text, element }: { text: string; element: ReactElement }) {
  const links = use(FilePathLinksContext);
  const insideLink = use(FilePathInsideLinkContext);
  const token =
    links && !insideLink ? detectFilePath(text, "inline_code", { windows: links.windows }) : null;
  if (!links || !token) return element;
  return (
    <FilePathAffordance links={links} token={token}>
      {element}
    </FilePathAffordance>
  );
}

// The sanitize pass keeps an href with no protocol or an allowed one; others read "undefined".
const SANITIZE_HREF_PROTOCOLS = ["http", "https", "irc", "ircs", "mailto", "xmpp"];

function sanitizedHref(href: string): string {
  const colon = href.indexOf(":");
  const firstDelimiter = Math.min(
    ...["/", "?", "#"].map(mark => href.indexOf(mark)).filter(index => index > -1),
    Number.POSITIVE_INFINITY
  );
  if (colon < 0 || colon > firstDelimiter) return href;
  const protocol = href.slice(0, colon).toLowerCase();
  return SANITIZE_HREF_PROTOCOLS.includes(protocol) ? href : "undefined";
}

/** The safe mode's rendering of a blocked link, kept for hrefs that are not file paths. */
function BlockedLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <span title={`Blocked URL: ${sanitizedHref(href)}`} className="text-gray-500">
      {children} [blocked]
    </span>
  );
}

function FilePathLinkText({ children }: { children: ReactNode }) {
  return (
    <span className="text-fg-2 underline decoration-fg-2 decoration-1 underline-offset-[3px]">
      {children}
    </span>
  );
}

/** `StreamMarkdown` local-link component: path hrefs open in the viewer and never navigate. */
export function FilePathLocalLink({ href, children }: { href: string; children: ReactNode }) {
  const links = use(FilePathLinksContext);
  const token = links ? detectFilePath(href, "href", { windows: links.windows }) : null;
  if (!links || !token) return <BlockedLink href={href}>{children}</BlockedLink>;
  return (
    <FilePathAffordance links={links} token={token}>
      <FilePathLinkText>{children}</FilePathLinkText>
    </FilePathAffordance>
  );
}

/** Anchor override: absolute POSIX path hrefs reach `a`; every other link keeps `MarkdownAnchor`. */
export function FilePathAnchor({
  href,
  children,
  className,
  ...props
}: ComponentProps<typeof MarkdownAnchor>) {
  const links = use(FilePathLinksContext);
  const token =
    links && typeof href === "string" && href.startsWith("/")
      ? detectFilePath(href, "href", { windows: links.windows })
      : null;
  if (!links || !token) {
    return (
      <MarkdownAnchor href={href} className={className} {...props}>
        <FilePathInsideLinkContext value>{children}</FilePathInsideLinkContext>
      </MarkdownAnchor>
    );
  }
  return (
    <FilePathAffordance links={links} token={token}>
      <FilePathLinkText>{children}</FilePathLinkText>
    </FilePathAffordance>
  );
}
