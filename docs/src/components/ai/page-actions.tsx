"use client";
import { useCopyButton } from "fumadocs-ui/utils/use-copy-button";
import { Check, Copy, ExternalLinkIcon, FileText, Github, MoreHorizontal } from "lucide-react";
import { useState } from "react";
import { Button } from "../ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";

const cache = new Map<string, string>();

export function LLMCopyButton({
  /**
   * A URL to fetch the raw Markdown/MDX content of page
   */
  markdownUrl,
}: {
  markdownUrl: string;
}) {
  const [isLoading, setLoading] = useState(false);
  const [checked, onClick] = useCopyButton(async () => {
    const cached = cache.get(markdownUrl);
    if (cached) return navigator.clipboard.writeText(cached);

    setLoading(true);

    try {
      await navigator.clipboard.write([
        new ClipboardItem({
          "text/plain": fetch(markdownUrl).then(async res => {
            const content = await res.text();
            cache.set(markdownUrl, content);

            return content;
          }),
        }),
      ]);
    } finally {
      setLoading(false);
    }
  });

  return (
    <Button disabled={isLoading} size="sm" variant="outline" onClick={onClick}>
      {checked ? <Check /> : <Copy />}
      Copy Page
    </Button>
  );
}

export function ViewOptions({
  markdownUrl,
  githubUrl,
}: {
  /**
   * A URL to the raw Markdown/MDX content of page
   */
  markdownUrl: string;

  /**
   * Source file URL on GitHub
   */
  githubUrl: string;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" variant="outline">
          <MoreHorizontal className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuItem asChild>
          <a
            href={githubUrl}
            rel="noreferrer noopener"
            target="_blank"
            className="flex items-center gap-2"
          >
            <Github className="h-4 w-4" />
            Open on GitHub
            <ExternalLinkIcon className="!size-3 ms-auto" />
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a
            href={markdownUrl}
            rel="noreferrer noopener"
            target="_blank"
            className="flex items-center gap-2"
          >
            <FileText className="h-4 w-4" />
            View in Markdown
            <ExternalLinkIcon className="!size-3 ms-auto" />
          </a>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
