/** S9: what it is, what would run on its own, and the way in to read it all. */
import { Check, FileCode, Plug, Terminal, X } from "@compozy/ui/icons";
import { useState } from "react";

import { Alert, AlertDescription, Button, Eyebrow, Item, Receipt } from "@compozy/ui";

import { TRIAGE } from "../stays-on-agents-shared/fixtures";
import { Notice } from "../stays-on-reports/report-entry";
import { SkillViewerDialog } from "./skill-viewer-dialog";

export interface SkillFile {
  path: string;
  language: string;
  body: string;
  /** Hook or MCP entry point. */
  automatic?: boolean;
}

export interface SkillProposal {
  name: string;
  why: string;
  files: readonly SkillFile[];
  automatic: readonly { kind: "hook" | "mcp" | "script"; label: string; detail: string }[];
}

const AUTOMATIC_ICON = { hook: Terminal, mcp: Plug, script: FileCode };

function RunsAutomatically({ items }: { items: SkillProposal["automatic"] }) {
  if (items.length === 0) {
    return (
      <p className="text-small-body text-muted-foreground">
        Nothing runs automatically. This skill is only instructions.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-1.5">
      <Eyebrow className="text-subtle">Runs automatically</Eyebrow>
      <ul className="flex flex-col gap-1">
        {items.map(item => {
          const Glyph = AUTOMATIC_ICON[item.kind];
          return (
            <li key={item.label}>
              <Item className="gap-2.5 px-0 py-1">
                <Glyph aria-hidden="true" className="size-4 shrink-0 text-subtle" />
                <span className="flex min-w-0 flex-col">
                  <span className="text-body text-fg">{item.label}</span>
                  <span className="truncate font-mono text-small-body text-muted-foreground">
                    {item.detail}
                  </span>
                </span>
              </Item>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export type ReviewVariant = "pending" | "stale";

export function SkillReviewCard({
  skill,
  variant = "pending",
  when = "09:14",
  defaultOpen = false,
  initialPath,
}: {
  skill: SkillProposal;
  variant?: ReviewVariant;
  when?: string;
  defaultOpen?: boolean;
  initialPath?: string;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [decision, setDecision] = useState<"approved" | "rejected" | null>(null);
  if (decision === "approved") {
    return (
      <Receipt icon={<Check aria-hidden="true" />} tone="allowed">
        You approved <b>{skill.name}</b>. It's active in compozy.
      </Receipt>
    );
  }
  if (decision === "rejected") {
    return (
      <Receipt icon={<X aria-hidden="true" />} tone="rejected">
        You rejected <b>{skill.name}</b>. It stays off unless its contents change.
      </Receipt>
    );
  }
  const stale = variant === "stale";
  return (
    <>
      <Notice
        actions={
          <Button onClick={() => setOpen(true)} size="sm" type="button">
            {stale ? "Review the latest" : "Review skill"}
          </Button>
        }
        agent={TRIAGE}
        body={skill.why}
        detail={
          <div className="flex min-w-0 flex-col gap-3">
            {stale ? (
              <Alert variant="warning">
                <AlertDescription>
                  This changed after you opened it. Read the latest version before deciding.
                </AlertDescription>
              </Alert>
            ) : null}
            <RunsAutomatically items={skill.automatic} />
          </div>
        }
        state="needs-you"
        title={`New skill to review: ${skill.name}`}
        when={when}
      />
      <SkillViewerDialog
        initialPath={initialPath}
        onApprove={() => setDecision("approved")}
        onOpenChange={setOpen}
        onReject={() => setDecision("rejected")}
        open={open}
        skill={skill}
        stale={stale}
      />
    </>
  );
}
