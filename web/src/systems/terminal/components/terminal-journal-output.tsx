import { Eyebrow } from "@compozy/ui";

import { terminalJournalOutputSummary } from "../lib/terminal-journal-copy";

/**
 * What the journal row can say about output: byte count and whether it was cut.
 *
 * The row has no last-output tail and no redaction length. Those stay unsaid.
 */
export function TerminalJournalOutput({
  outputBytes,
  truncated,
}: {
  outputBytes: number;
  truncated: boolean;
}) {
  return (
    <section className="flex flex-col gap-1.5 px-4 pb-3.5" data-testid="terminal-journal-output">
      <Eyebrow>Output size</Eyebrow>
      <div className="rounded-xs bg-terminal-bg px-2.5 py-2 font-mono text-form-input text-terminal-fg">
        {terminalJournalOutputSummary(outputBytes, truncated)}
      </div>
    </section>
  );
}
