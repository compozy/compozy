"use client";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@agh/ui";
import { ExternalLink, Sparkles } from "lucide-react";

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
          <Button size="sm" variant="outline">
            <Sparkles aria-hidden />
            Open with AI
          </Button>
        }
      />
      <DropdownMenuContent align="end" sideOffset={6}>
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
