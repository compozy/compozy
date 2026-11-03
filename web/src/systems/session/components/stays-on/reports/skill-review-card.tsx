/**
 * S9: a skill the agent proposed. The card says why and what would run on its own, first; Review
 * skill opens the exact contents, where the operator approves or rejects them.
 */
import { Check, FileCode, Plug, Terminal, X } from "@compozy/ui/icons";
import { useState } from "react";

import { Alert, AlertDescription, Button, Eyebrow, Item, Receipt, Skeleton } from "@compozy/ui";

import {
  AgentSkillViewerDialog,
  useAgentSkillReview,
  type AgentSkillReviewView,
  type SkillAutomaticItem,
} from "@/systems/agent";

import type { ReportAgent } from "../../../lib/session-report-identity";
import { Notice } from "./report-entry";

const AUTOMATIC_ICON = { hook: Terminal, mcp: Plug, script: FileCode };

export function RunsAutomatically({ items }: { items: readonly SkillAutomaticItem[] }) {
  if (items.length === 0) {
    return (
      <p className="text-small-body text-muted-foreground">
        Nothing runs automatically. This skill is only instructions.
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-1.5" data-testid="skill-review-automatic">
      <Eyebrow className="text-subtle">Runs automatically</Eyebrow>
      <ul className="flex flex-col gap-1">
        {items.map(item => {
          const Glyph = AUTOMATIC_ICON[item.kind];
          return (
            <li key={item.kind}>
              <Item className="gap-2.5 px-0 py-1">
                <Glyph aria-hidden="true" className="size-4 shrink-0 text-subtle" />
                <span className="flex min-w-0 flex-col">
                  <span className="text-body text-fg">{item.label}</span>
                  {item.detail ? (
                    <span className="truncate font-mono text-small-body text-muted-foreground">
                      {item.detail}
                    </span>
                  ) : null}
                </span>
              </Item>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** The card for a loaded review; the container below reads and decides it. */
export function SkillReviewCardView({
  agent,
  when,
  view,
  defaultOpen = false,
}: {
  agent: ReportAgent;
  when: string;
  view: AgentSkillReviewView;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const { review } = view;
  if (!review) return null;
  const decide = (decision: "approve" | "reject") => view.decide(decision);
  const dialog = (
    <AgentSkillViewerDialog
      agentLabel={agent.label}
      decided={view.state !== "pending"}
      deciding={view.deciding}
      error={view.decideError?.message}
      files={view.files}
      key={review.digest}
      name={review.name}
      onApprove={() => decide("approve")}
      onOpenChange={setOpen}
      onReject={() => decide("reject")}
      open={open && view.state === "pending"}
      stale={view.stale}
    />
  );
  if (view.state === "approved") {
    return (
      <Receipt icon={<Check aria-hidden="true" />} tone="allowed">
        You approved <b>{review.name}</b>. It's active now.
      </Receipt>
    );
  }
  if (view.state === "rejected") {
    return (
      <Receipt icon={<X aria-hidden="true" />} tone="rejected">
        You rejected <b>{review.name}</b>. It stays off unless its contents change.
      </Receipt>
    );
  }
  return (
    <>
      <Notice
        actions={
          <Button onClick={() => setOpen(true)} size="sm" type="button">
            {view.stale ? "Review the latest" : "Review skill"}
          </Button>
        }
        agent={agent}
        body={
          view.why ??
          "I wrote down a procedure as a skill. Nothing in it runs until you approve it."
        }
        detail={
          <div className="flex min-w-0 flex-col gap-3">
            {view.stale ? (
              <Alert variant="warning">
                <AlertDescription>
                  This changed after you opened it. Read the latest version before deciding.
                </AlertDescription>
              </Alert>
            ) : null}
            <RunsAutomatically items={view.automatic} />
          </div>
        }
        state="needs-you"
        title={`New skill to review: ${review.name}`}
        when={when}
      />
      {dialog}
    </>
  );
}

export function SkillReviewCard({
  agent,
  when,
  reviewId,
  profile,
}: {
  agent: ReportAgent;
  when: string;
  reviewId: string;
  profile?: string;
}) {
  const view = useAgentSkillReview({ id: reviewId, profile });
  if (view.loading) return <Skeleton className="h-28 w-full rounded-lg" />;
  if (view.loadError || !view.review) {
    return (
      <Notice
        agent={agent}
        body="I proposed a skill, but it couldn't be loaded here. Try again in a moment."
        title="A skill is waiting for review"
        when={when}
      />
    );
  }
  return <SkillReviewCardView agent={agent} view={view} when={when} />;
}
