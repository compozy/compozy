import { use, type ComponentProps, type ReactElement, type ReactNode } from "react";

import { Button, Kbd, MarkdownAnchor, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { FileText } from "@compozy/ui/icons";

import { useFilePathOpenAction } from "../hooks/use-file-path-open-action";
import type { OpenInEditor } from "../hooks/use-open-in-editor";
import { detectFilePath, filePathBasename, type FilePathToken } from "../lib/file-path-links";
import { FilePathOpenerContext } from "../lib/file-path-opener-context";

// Hover underline on the path text only; the button appears on hover of the span or on focus.
const PATH_TEXT_CLASS =
  "cursor-text group-hover/path:underline group-hover/path:decoration-dotted group-hover/path:decoration-current";
const OPEN_BUTTON_CLASS =
  "ml-0.5 inline-flex align-middle opacity-0 group-hover/path:opacity-100 focus-visible:opacity-100 disabled:opacity-100";

export function OpenInEditorButton({
  token,
  pending,
  modifier,
  onOpen,
}: {
  token: FilePathToken;
  pending: boolean;
  modifier: OpenInEditor["modifier"];
  onOpen: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={`Open ${filePathBasename(token.path)} in editor`}
            className={OPEN_BUTTON_CLASS}
            data-testid="open-in-editor-button"
            disabled={pending}
            onClick={event => {
              event.preventDefault();
              onOpen();
            }}
          />
        }
      >
        <FileText aria-hidden />
      </TooltipTrigger>
      <TooltipContent className="gap-1.5">
        Open in editor
        <Kbd>{modifier === "meta" ? "⌘-click" : "Ctrl-click"}</Kbd>
      </TooltipContent>
    </Tooltip>
  );
}

function FilePathAffordance({
  opener,
  token,
  children,
}: {
  opener: OpenInEditor;
  token: FilePathToken;
  children: ReactNode;
}) {
  const { pending, run, onModifierClick } = useFilePathOpenAction(opener, token);
  return (
    <span className="group/path" data-file-path={token.raw}>
      {/* Cmd/Ctrl+click is a mouse shortcut; the adjacent button is the keyboard path. */}
      <span className={PATH_TEXT_CLASS} onClick={onModifierClick} role="presentation">
        {children}
      </span>
      <OpenInEditorButton
        token={token}
        pending={pending}
        modifier={opener.modifier}
        onOpen={() => void run()}
      />
    </span>
  );
}

/** `StreamMarkdown` inline-code decorator body: path spans gain the open affordance. */
export function FilePathInlineCode({ text, element }: { text: string; element: ReactElement }) {
  const opener = use(FilePathOpenerContext);
  const token = opener ? detectFilePath(text, "inline_code") : null;
  if (!opener || !token) return element;
  return (
    <FilePathAffordance opener={opener} token={token}>
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

/** `StreamMarkdown` local-link component: path hrefs open in the editor and never navigate. */
export function FilePathLocalLink({ href, children }: { href: string; children: ReactNode }) {
  const opener = use(FilePathOpenerContext);
  const token = opener ? detectFilePath(href, "href") : null;
  if (!opener || !token) return <BlockedLink href={href}>{children}</BlockedLink>;
  return (
    <FilePathAffordance opener={opener} token={token}>
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
  const opener = use(FilePathOpenerContext);
  const token =
    opener && typeof href === "string" && href.startsWith("/")
      ? detectFilePath(href, "href")
      : null;
  if (!opener || !token) {
    return (
      <MarkdownAnchor href={href} className={className} {...props}>
        {children}
      </MarkdownAnchor>
    );
  }
  return (
    <FilePathAffordance opener={opener} token={token}>
      <FilePathLinkText>{children}</FilePathLinkText>
    </FilePathAffordance>
  );
}
