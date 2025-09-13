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
      Copy Markdown
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

export function OpenWithAI({
  pageUrl,
}: {
  /**
   * Absolute URL of the current docs page
   */
  pageUrl: string;
}) {
  const prompt = `Read ${pageUrl}, I want to ask questions about it`;
  const encoded = encodeURIComponent(prompt);
  const chatgpt = `https://chatgpt.com/?q=${encoded}`;
  const claude = `https://claude.ai/new?q=${encoded}`;
  const t3Base = process.env.NEXT_PUBLIC_T3_CHAT_URL;
  const sciraBase = process.env.NEXT_PUBLIC_SCIRA_CHAT_URL;
  const t3 = t3Base ? `${t3Base}${t3Base.includes("?") ? "&" : "?"}q=${encoded}` : undefined;
  const scira = sciraBase
    ? `${sciraBase}${sciraBase.includes("?") ? "&" : "?"}q=${encoded}`
    : undefined;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="sm" variant="outline">
          Open with AI
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuItem asChild>
          <a
            href={chatgpt}
            rel="noreferrer noopener"
            target="_blank"
            className="flex items-center gap-2"
          >
            <ExternalLinkIcon className="h-4 w-4" />
            ChatGPT
            <ExternalLinkIcon className="!size-3 ms-auto" />
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a
            href={claude}
            rel="noreferrer noopener"
            target="_blank"
            className="flex items-center gap-2"
          >
            <ExternalLinkIcon className="h-4 w-4" />
            Claude
            <ExternalLinkIcon className="!size-3 ms-auto" />
          </a>
        </DropdownMenuItem>
        {t3 && (
          <DropdownMenuItem asChild>
            <a
              href={t3}
              rel="noreferrer noopener"
              target="_blank"
              className="flex items-center gap-2"
            >
              <ExternalLinkIcon className="h-4 w-4" />
              T3 Chat
              <ExternalLinkIcon className="!size-3 ms-auto" />
            </a>
          </DropdownMenuItem>
        )}
        {scira && (
          <DropdownMenuItem asChild>
            <a
              href={scira}
              rel="noreferrer noopener"
              target="_blank"
              className="flex items-center gap-2"
            >
              <ExternalLinkIcon className="h-4 w-4" />
              Scira AI
              <ExternalLinkIcon className="!size-3 ms-auto" />
            </a>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
