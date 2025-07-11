import { cn } from "@/lib/utils";
import { CodeBlock, Pre } from "fumadocs-ui/components/codeblock";
import React from "react";

interface CodeProps extends React.ComponentProps<typeof CodeBlock> {
  children: React.ReactNode;
  showLineNumbers?: boolean;
  className?: string;
}

export function Code({ children, showLineNumbers = false, className, ...props }: CodeProps) {
  return (
    <CodeBlock
      {...props}
      className={cn("w-full min-w-0 overflow-x-auto", className)}
      data-line-numbers={showLineNumbers}
    >
      <Pre className="!w-full !min-w-0 overflow-x-auto whitespace-pre">{children}</Pre>
    </CodeBlock>
  );
}
