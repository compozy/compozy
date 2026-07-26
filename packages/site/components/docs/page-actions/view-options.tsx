"use client";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@agh/ui";
import { GithubLogo } from "@agh/ui/logos";
import { ExternalLink, FileText, MoreHorizontal } from "lucide-react";

export interface ViewOptionsProps {
  markdownUrl: string;
  githubUrl: string;
}

export function ViewOptions({ markdownUrl, githubUrl }: ViewOptionsProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button aria-label="Page options" size="icon-sm" variant="outline">
            <MoreHorizontal aria-hidden />
          </Button>
        }
      />
      <DropdownMenuContent align="end" sideOffset={6}>
        <DropdownMenuItem
          render={
            <a href={githubUrl} rel="noreferrer noopener" target="_blank">
              <GithubLogo aria-hidden className="size-3" />
              Open on GitHub
              <ExternalLink aria-hidden className="ms-auto size-3!" />
            </a>
          }
        />
        <DropdownMenuItem
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
