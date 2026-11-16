import { CircleAlert, KeyRound, RotateCw, Settings2, Terminal } from "@compozy/ui/icons";
import type { ComponentType, ReactNode } from "react";

import { Button, Disclosure, PopoverPanel, Spinner } from "@compozy/ui";

import { MCPCommandBlock } from "./mcp-panel-parts";

export interface MCPIssuePanelProps {
  server: string;
  /** One line on what went wrong, e.g. "It started but offered no tools". */
  summary?: string;
  /** What the server itself said, verbatim. */
  reason?: string | null;
  /** A check of the server is running. */
  checking?: boolean;
  /** The last check answered and the server still is not working. */
  stillFailing?: boolean;
  /** Asks CompozyOS to try the server again. */
  onCheck: () => void;
  /** Opens the server in Settings; absent when the panel already lives there. */
  onOpenSettings?: () => void;
  onClose: () => void;
}

function Suggestion({
  icon: Icon,
  children,
}: {
  icon: ComponentType<{ className?: string; "aria-hidden"?: true }>;
  children: ReactNode;
}) {
  return (
    <li className="flex items-start gap-2.5 text-small-body text-fg">
      <Icon aria-hidden={true} className="mt-0.5 size-3.5 shrink-0 text-faint" />
      <span>{children}</span>
    </li>
  );
}

/** Why a server is not working, what to check, and Check again. */
export function MCPIssuePanel({
  server,
  summary,
  reason,
  checking = false,
  stillFailing = false,
  onCheck,
  onOpenSettings,
  onClose,
}: MCPIssuePanelProps) {
  return (
    <PopoverPanel
      data-testid="mcp-issue-panel"
      icon={CircleAlert}
      tone="destructive"
      title={`${server} isn't working`}
      subtitle={summary}
      onClose={onClose}
      footer={
        <>
          {onOpenSettings ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="-ml-1.5 text-muted-foreground"
              onClick={onOpenSettings}
            >
              <Settings2 data-icon="inline-start" />
              Open in Settings
            </Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            className="ml-auto"
            disabled={checking}
            onClick={onCheck}
            data-testid="mcp-issue-panel-check"
          >
            {checking ? <Spinner className="size-3.5" /> : <RotateCw data-icon="inline-start" />}
            {checking ? "Checking…" : "Check again"}
          </Button>
        </>
      }
    >
      {stillFailing ? (
        <p
          role="status"
          className="text-small-body text-destructive"
          data-testid="mcp-issue-panel-still"
        >
          Still not working after the check.
        </p>
      ) : null}
      <ul className="flex flex-col gap-2">
        <Suggestion icon={KeyRound}>Check that its key is set and still valid.</Suggestion>
        <Suggestion icon={Terminal}>Make sure its command runs on this computer.</Suggestion>
      </ul>
      {reason ? (
        <Disclosure label="What the server said">
          <MCPCommandBlock>{reason}</MCPCommandBlock>
        </Disclosure>
      ) : null}
    </PopoverPanel>
  );
}
