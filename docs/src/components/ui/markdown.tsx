"use client";

import {
  CodeBlock,
  CodeBlockBody,
  CodeBlockContent,
  CodeBlockItem,
  type BundledLanguage,
} from "@/components/ui/kibo-ui/code-block";
import { useTheme } from "next-themes";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Code } from "./code";

interface MarkdownProps {
  children: string;
  className?: string;
}

/**
 * Maps common language aliases to Kibo-UI supported languages
 */
function mapLanguage(lang: string): BundledLanguage {
  const langMap: { [key: string]: BundledLanguage } = {
    yml: "yaml",
    js: "javascript",
    ts: "typescript",
    md: "markdown",
  };
  const normalizedLang = lang.toLowerCase();
  return (langMap[normalizedLang] || normalizedLang) as BundledLanguage;
}

/**
 * Reusable Markdown component with Kibo-UI code block integration
 */
export function Markdown({ children, className }: MarkdownProps) {
  const { resolvedTheme } = useTheme();

  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          pre: ({ children }) => <>{children}</>, // Skip <pre> wrapper
          code: ({ className, children, ...props }) => {
            const inline = !className || !className.includes("language-");
            if (inline) {
              return (
                <Code size="sm" {...props}>
                  {children}
                </Code>
              );
            }

            const match = /language-(\w+)/.exec(className || "");
            const lang = match ? match[1] : "text";
            const mappedLang = mapLanguage(lang);
            const codeContent = String(children).replace(/\n$/, "");

            if (!codeContent) {
              return null; // Skip empty code blocks
            }

            const codeData = [
              {
                language: mappedLang,
                filename: "", // No filename for generic markdown
                code: codeContent,
              },
            ];

            return (
              <CodeBlock data={codeData} defaultValue={mappedLang}>
                <CodeBlockBody>
                  {item => (
                    <CodeBlockItem key={item.language} value={item.language} lineNumbers={false}>
                      <CodeBlockContent
                        language={item.language as BundledLanguage}
                        syntaxHighlighting={true}
                        themes={{
                          light: resolvedTheme === "dark" ? "vitesse-dark" : "vitesse-light",
                          dark: resolvedTheme === "dark" ? "vitesse-dark" : "vitesse-light",
                        }}
                      >
                        {item.code}
                      </CodeBlockContent>
                    </CodeBlockItem>
                  )}
                </CodeBlockBody>
              </CodeBlock>
            );
          },
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
