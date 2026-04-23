import type { HTMLAttributes, ReactElement } from "react";
import ReactMarkdown, { type Options as ReactMarkdownOptions } from "react-markdown";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";

import { cn } from "../lib/utils";

export interface MarkdownProps extends Omit<HTMLAttributes<HTMLDivElement>, "children"> {
  children: string;
  components?: ReactMarkdownOptions["components"];
}

const proseClasses = [
  "prose prose-sm prose-invert max-w-none",
  "text-foreground [&_*]:leading-relaxed",
  "[&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:font-display [&_h1]:text-2xl [&_h1]:tracking-[-0.02em] [&_h1]:text-foreground",
  "[&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:font-display [&_h2]:text-xl [&_h2]:tracking-[-0.02em] [&_h2]:text-foreground",
  "[&_h3]:mt-4 [&_h3]:mb-2 [&_h3]:text-base [&_h3]:font-semibold [&_h3]:text-foreground",
  "[&_h4]:mt-3 [&_h4]:mb-1 [&_h4]:text-sm [&_h4]:font-semibold [&_h4]:text-foreground",
  "[&_p]:my-2 [&_p]:text-sm [&_p]:text-foreground/90",
  "[&_a]:text-[color:var(--primary)] [&_a]:underline [&_a]:underline-offset-2 hover:[&_a]:brightness-110",
  "[&_strong]:font-semibold [&_strong]:text-foreground",
  "[&_em]:italic",
  "[&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_ul]:text-sm",
  "[&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_ol]:text-sm",
  "[&_li]:my-1 [&_li]:text-foreground/90",
  "[&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-[color:var(--primary)]/60 [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground",
  "[&_code]:rounded [&_code]:bg-[color:var(--tone-neutral-bg)] [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em] [&_code]:text-foreground",
  "[&_pre]:my-3 [&_pre]:overflow-auto [&_pre]:rounded-[var(--radius-md)] [&_pre]:border [&_pre]:border-border [&_pre]:bg-black/30 [&_pre]:p-3 [&_pre]:text-xs [&_pre]:leading-snug",
  "[&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-foreground",
  "[&_table]:my-3 [&_table]:w-full [&_table]:border-collapse [&_table]:text-left [&_table]:text-sm",
  "[&_th]:border-b [&_th]:border-border [&_th]:px-2 [&_th]:py-1.5 [&_th]:font-semibold [&_th]:text-foreground",
  "[&_td]:border-b [&_td]:border-border/50 [&_td]:px-2 [&_td]:py-1.5 [&_td]:text-foreground/90",
  "[&_hr]:my-5 [&_hr]:border-border",
  "[&_img]:my-3 [&_img]:rounded-[var(--radius-md)] [&_img]:border [&_img]:border-border",
];

export function Markdown({
  children,
  className,
  components,
  ...props
}: MarkdownProps): ReactElement {
  return (
    <div className={cn(proseClasses, className)} {...props}>
      <ReactMarkdown
        components={components}
        rehypePlugins={[rehypeSanitize]}
        remarkPlugins={[remarkGfm]}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
