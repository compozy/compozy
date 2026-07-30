"use client";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { ChevronDown, ExternalLink, Sparkles } from "lucide-react";

export interface OpenWithAIProps {
  pageUrl: string;
}

export function OpenWithAI({ pageUrl }: OpenWithAIProps) {
  const prompt = `Read ${pageUrl}, I want to ask questions about it`;
  const encoded = encodeURIComponent(prompt);
  const targets = [
    { id: "chatgpt", label: "ChatGPT", href: `https://chatgpt.com/?q=${encoded}` },
    { id: "claude", label: "Claude", href: `https://claude.ai/new?q=${encoded}` },
  ];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button className="site-doc-page-action h-search" size="sm" variant="outline">
            <Sparkles aria-hidden />
            Open with AI
            <ChevronDown aria-hidden className="size-3!" />
          </Button>
        }
      />
      <DropdownMenuContent align="end" sideOffset={6}>
        <DropdownMenuLabel>Ask about this page</DropdownMenuLabel>
        {targets.map(target => (
          <DropdownMenuItem
            key={target.id}
            render={
              <a href={target.href} rel="noreferrer noopener" target="_blank">
                {target.label}
                <ExternalLink aria-hidden className="ms-auto size-3!" />
              </a>
            }
          />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
