import { DynamicCodeBlock } from "fumadocs-ui/components/dynamic-codeblock";
import type { Components } from "react-markdown";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { tv } from "tailwind-variants";
import { cn } from "../../lib/utils";

interface MarkdownProps {
  children: string;
  className?: string;
}

const markdownVariants = tv({
  slots: {
    container: ["mt-2 [&>*:first-child]:mt-0 [&>*:last-child]:mb-0", "[&_li>p]:m-0"],
  },
});

/**
 * Reusable Markdown component with Fumadocs default code block integration
 */
export function Markdown({ children, className }: MarkdownProps) {
  const styles = markdownVariants();

  const components: Components = {
    code({ node: _node, className, children, ...props }) {
      const match = /language-(\w+)/.exec(className || "");
      const isInline = !match && !("inline" in props && props.inline === false);

      if (isInline) {
        return (
          <code className={className} {...props}>
            {children}
          </code>
        );
      }

      const language = match ? match[1] : "text";
      const codeString = String(children).replace(/\n$/, "");

      return (
        <DynamicCodeBlock
          lang={language}
          code={codeString}
          options={{
            themes: {
              light: "vitesse-light",
              dark: "vitesse-dark",
            },
          }}
        />
      );
    },
  };

  return (
    <div className={cn(className, styles.container())}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  );
}
