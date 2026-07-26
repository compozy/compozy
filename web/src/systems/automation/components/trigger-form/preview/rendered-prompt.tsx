import { FileText } from "lucide-react";

import { Eyebrow } from "@agh/ui";

import type { RenderToken, TemplateToken } from "../../../lib/trigger-template";
import { PreviewCard } from "./preview-card";

function RenderedTokens({ tokens }: { tokens: RenderToken[] }) {
  return (
    <>
      {tokens.map(token => {
        if (token.type === "text") {
          return <span key={token.id}>{token.value}</span>;
        }
        if (token.type === "var") {
          return (
            <span
              key={token.id}
              className="rounded-[3px] bg-badge-fill px-0.5 font-mono text-form-hint text-fg-strong ring-1 ring-line-soft ring-inset"
            >
              {token.value}
            </span>
          );
        }
        return (
          <span
            key={token.id}
            className="rounded-[3px] bg-danger-tint px-0.5 font-mono text-form-hint text-danger"
          >
            {`‹${token.name}?›`}
          </span>
        );
      })}
    </>
  );
}

interface RenderedPromptProps {
  rendered: RenderToken[];
  templateTokens: TemplateToken[];
}

/** The prompt with variables substituted (missing flagged) over the tinted source. */
export function RenderedPrompt({ rendered, templateTokens }: RenderedPromptProps) {
  return (
    <PreviewCard icon={FileText} label="Prompt the agent receives">
      <div className="text-small-body leading-relaxed whitespace-pre-wrap text-fg">
        <RenderedTokens tokens={rendered} />
      </div>
      <div className="mt-2.5 border-t border-line-soft pt-2.5">
        <Eyebrow className="text-faint">Template</Eyebrow>
        <p className="mt-1.5 font-mono text-mono-id leading-relaxed whitespace-pre-wrap text-subtle">
          {templateTokens.map(token =>
            token.type === "var" ? (
              <span key={token.id} className="text-accent-strong">
                {token.value}
              </span>
            ) : (
              <span key={token.id}>{token.value}</span>
            )
          )}
        </p>
      </div>
    </PreviewCard>
  );
}
