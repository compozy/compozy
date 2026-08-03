import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  remarkReleaseStructure,
  stripLeadingReleaseHeading,
} from "@/lib/changelog/release-markdown";

export function ReleaseMarkdown({
  body,
  pullRequestNumbers,
}: {
  body: string;
  pullRequestNumbers: number[];
}) {
  return (
    <div className="min-w-0 text-base leading-8 text-muted">
      <ReactMarkdown
        skipHtml
        remarkPlugins={[remarkGfm, [remarkReleaseStructure, { pullRequestNumbers }]]}
        components={{
          h3: ({ children, ...props }) => (
            <h2
              {...props}
              className="mt-14 scroll-mt-24 font-sans text-2xl font-semibold tracking-tight text-fg first:mt-0"
            >
              {children}
            </h2>
          ),
          h4: ({ children, ...props }) => (
            <h3 {...props} className="mt-10 scroll-mt-24 font-sans text-lg font-semibold text-fg">
              {children}
            </h3>
          ),
          h5: ({ children, ...props }) => (
            <h4 {...props} className="mt-8 scroll-mt-24 font-sans text-base font-semibold text-fg">
              {children}
            </h4>
          ),
          h6: ({ children, ...props }) => (
            <h5 {...props} className="mt-7 scroll-mt-24 font-sans text-sm font-semibold text-fg">
              {children}
            </h5>
          ),
          p: props => <p {...props} className="mt-5 max-w-[74ch]" />,
          ul: props => (
            <ul {...props} className="mt-5 flex list-disc flex-col gap-2 pl-5 marker:text-accent" />
          ),
          ol: props => (
            <ol
              {...props}
              className="mt-5 flex list-decimal flex-col gap-2 pl-5 marker:text-subtle"
            />
          ),
          a: ({ href, ...props }) => {
            const external = href?.startsWith("http");
            return (
              <a
                {...props}
                href={href}
                className="font-medium text-accent underline decoration-accent/35 underline-offset-4 hover:decoration-accent"
                target={external ? "_blank" : undefined}
                rel={external ? "noreferrer noopener" : undefined}
              />
            );
          },
          code: props => (
            <code
              {...props}
              className="rounded bg-elevated px-1.5 py-0.5 font-mono text-sm text-fg"
            />
          ),
          pre: props => (
            <pre
              {...props}
              className="mt-5 overflow-x-auto rounded-xl border border-line bg-canvas-soft p-4 text-sm"
            />
          ),
          blockquote: props => (
            <blockquote {...props} className="mt-6 border-l-2 border-accent pl-5 text-fg" />
          ),
          hr: () => <hr className="my-10 border-line" />,
          table: props => (
            <div className="mt-6 overflow-x-auto">
              <table {...props} className="w-full border-collapse text-sm" />
            </div>
          ),
          th: props => (
            <th
              {...props}
              className="border-b border-line px-3 py-2 text-left font-semibold text-fg"
            />
          ),
          td: props => <td {...props} className="border-b border-line px-3 py-2 align-top" />,
        }}
      >
        {stripLeadingReleaseHeading(body)}
      </ReactMarkdown>
    </div>
  );
}
