"use client";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";
import { ExternalLink, FileText, MoreHorizontal } from "lucide-react";

import {
  SITE_MENU_CONTENT_CLASS,
  SITE_MENU_ITEM_CLASS,
} from "@/components/docs/page-actions/menu-classes";

export interface ViewOptionsProps {
  markdownUrl: string;
  githubUrl: string;
}

export function ViewOptions({ markdownUrl, githubUrl }: ViewOptionsProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="Page options"
            className="site-doc-page-action site-doc-page-action--icon h-search w-(length:--height-search)!"
            size="icon-sm"
            variant="outline"
          >
            <MoreHorizontal aria-hidden />
          </Button>
        }
      />
      <DropdownMenuContent align="end" className={SITE_MENU_CONTENT_CLASS} sideOffset={6}>
        <DropdownMenuItem
          className={SITE_MENU_ITEM_CLASS}
          render={
            <a href={githubUrl} rel="noreferrer noopener" target="_blank">
              <GithubLogo aria-hidden className="size-3" />
              Open on GitHub
              <ExternalLink aria-hidden className="ms-auto size-3!" />
            </a>
          }
        />
        <DropdownMenuItem
          className={SITE_MENU_ITEM_CLASS}
          render={
            <a href={markdownUrl} rel="noreferrer noopener" target="_blank">
              <FileText aria-hidden />
              View as Markdown
              <ExternalLink aria-hidden className="ms-auto size-3!" />
            </a>
          }
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
