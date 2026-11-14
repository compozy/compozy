import type { ComponentProps, ReactNode } from "react";
import { createContext, use } from "react";

import {
  delimiterForPath,
  FileHtmlPreview,
  FileNotebookPreview,
  FileTablePreview,
  MarkdownAnchor,
  StreamMarkdown,
} from "@compozy/ui";

import type { FilesRootRef } from "../adapters/files-api";
import { type MarkdownLinkScope, useMarkdownLinkScope } from "../hooks/use-markdown-link-scope";
import {
  isExternalLink,
  openExternalLink,
  pathFromRawUrl,
  rawAssetUrl,
  rawDirectoryUrl,
  rawFileUrl,
  resolveRelativePath,
  type TextPreviewKind,
} from "../lib/file-preview";
import { FileViewerMessageState } from "./file-viewer-states";

export interface FileTextPreviewProps {
  kind: Exclude<TextPreviewKind, "svg">;
  root: FilesRootRef;
  path: string;
  text: string;
  /** Opens another file of the same root in the viewer (a relative link). */
  onOpenFile?: (path: string) => void;
}

const FRONTMATTER = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/;

/** YAML frontmatter shows as a fenced block above the prose instead of a stray rule and text. */
function withFrontmatterBlock(text: string): string {
  const match = FRONTMATTER.exec(text);
  if (!match) return text;
  return `\`\`\`yaml\n${match[1]}\n\`\`\`\n\n${text.slice(match[0].length)}`;
}

function Unreadable({ title }: { title: string }) {
  return (
    <FileViewerMessageState
      testId="file-viewer-preview-unreadable"
      title={title}
      description="Switch to Source to see the file as written."
    />
  );
}

const MarkdownLinkScopeContext = createContext<MarkdownLinkScope | null>(null);

function PreviewAnchor({
  href,
  onClick: _onClick,
  ...props
}: ComponentProps<typeof MarkdownAnchor>) {
  const scope = use(MarkdownLinkScopeContext);
  return (
    <MarkdownAnchor
      href={href}
      onClick={event => {
        if (scope && typeof href === "string") scope.follow(href, event);
      }}
      {...props}
    />
  );
}

function PreviewLocalLink({ href, children }: { href: string; children: ReactNode }) {
  const scope = use(MarkdownLinkScopeContext);
  return (
    <MarkdownAnchor href={href} onClick={event => scope?.follow(href, event)}>
      {children}
    </MarkdownAnchor>
  );
}

/** Relative images load from the same root through the raw-bytes route; remote ones stay blocked. */
function PreviewImage({ src, alt, title }: ComponentProps<"img">) {
  const scope = use(MarkdownLinkScopeContext);
  const label = alt || "image";
  const target = scope && typeof src === "string" ? resolveRelativePath(scope.path, src) : null;
  if (!scope || !target) {
    return <span className="text-muted-foreground italic">{`[image: ${label}]`}</span>;
  }
  return (
    <img
      data-slot="markdown-image"
      src={rawFileUrl(scope.root, target)}
      alt={label}
      title={title}
      className="max-w-full rounded-md"
    />
  );
}

type MarkdownComponents = NonNullable<ComponentProps<typeof StreamMarkdown>["components"]>;

// Streamdown types every override with its loose element props; these read the standard ones.
const PREVIEW_COMPONENTS = { a: PreviewAnchor, img: PreviewImage } as MarkdownComponents;

function dirname(path: string): string {
  const slash = path.lastIndexOf("/");
  return slash < 0 ? "" : path.slice(0, slash);
}

function MarkdownPreview({ root, path, text, onOpenFile }: Omit<FileTextPreviewProps, "kind">) {
  const scope = useMarkdownLinkScope(root, path, onOpenFile);
  return (
    <div
      className="min-h-0 flex-1 overflow-auto bg-canvas"
      data-testid="file-viewer-markdown-preview"
    >
      <MarkdownLinkScopeContext value={scope}>
        <StreamMarkdown
          className="mx-auto max-w-3xl px-8 py-6"
          components={PREVIEW_COMPONENTS}
          localLinkComponent={PreviewLocalLink}
          linkBase={dirname(path)}
        >
          {withFrontmatterBlock(text)}
        </StreamMarkdown>
      </MarkdownLinkScopeContext>
    </div>
  );
}

function NotebookPreview({ root, path, text, onOpenFile }: Omit<FileTextPreviewProps, "kind">) {
  const scope = useMarkdownLinkScope(root, path, onOpenFile);
  return (
    <MarkdownLinkScopeContext value={scope}>
      <FileNotebookPreview
        text={text}
        markdown={{
          components: PREVIEW_COMPONENTS,
          localLinkComponent: PreviewLocalLink,
          linkBase: dirname(path),
        }}
        fallback={<Unreadable title="This notebook can't be rendered" />}
        data-testid="file-viewer-notebook-preview"
      />
    </MarkdownLinkScopeContext>
  );
}

/** The rendered form of a text file whose kind has one (Markdown, HTML, CSV/TSV, Mermaid, notebook). */
export function FileTextPreview({ kind, root, path, text, onOpenFile }: FileTextPreviewProps) {
  switch (kind) {
    case "markdown":
      return <MarkdownPreview root={root} path={path} text={text} onOpenFile={onOpenFile} />;
    case "mermaid":
      return (
        <div
          className="min-h-0 flex-1 overflow-auto bg-canvas"
          data-testid="file-viewer-mermaid-preview"
        >
          <StreamMarkdown className="px-8 py-6">{`\`\`\`mermaid\n${text}\n\`\`\``}</StreamMarkdown>
        </div>
      );
    case "html":
      return (
        <FileHtmlPreview
          html={text}
          title={path}
          baseUrl={rawDirectoryUrl(root, path)}
          resolveAssetUrl={url => rawAssetUrl(root, url)}
          onLinkClick={url => {
            const target = pathFromRawUrl(root, url);
            if (target) onOpenFile?.(target);
            else if (isExternalLink(url)) openExternalLink(url);
          }}
          data-testid="file-viewer-html-preview"
        />
      );
    case "table":
      return (
        <FileTablePreview
          text={text}
          delimiter={delimiterForPath(path)}
          empty={<Unreadable title="This file has no rows" />}
          data-testid="file-viewer-table-preview"
        />
      );
    case "notebook":
      return <NotebookPreview root={root} path={path} text={text} onOpenFile={onOpenFile} />;
  }
}
