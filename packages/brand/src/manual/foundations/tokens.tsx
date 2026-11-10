import tokensCss from "@compozy/ui/tokens.css?raw";

import { btn, btnSecondary, btnSm } from "../cls";
import { Section } from "../parts/section";

export function Tokens() {
  return (
    <Section
      id="tokens"
      n="12"
      file="tokens.css"
      title={
        <>
          Tokens for <em>developers</em>
        </>
      }
      bg="black"
      status="approved"
      intro={
        <p>
          One file, <code>packages/ui/src/tokens.css</code>, is the source of truth. The product,
          the site theme and this manual read from it. A test fails if a value drifts or if a text
          pair drops below AA.
        </p>
      }
    >
      <div className="mb-4 flex flex-wrap gap-2">
        <button
          type="button"
          data-copy-from="tokens-src"
          className={`${btn} ${btnSm} ${btnSecondary}`}
        >
          Copy tokens.css
        </button>
      </div>
      <pre
        id="tokens-src"
        className="m-0 max-h-[560px] overflow-auto rounded-card border border-(--b-line) bg-(--b-card) p-6 font-mono text-[12.5px] leading-[1.7] text-(--b-muted)"
      >
        {tokensCss}
      </pre>
    </Section>
  );
}
