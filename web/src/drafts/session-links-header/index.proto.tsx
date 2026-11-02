import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { HeaderDirection } from "./header-direction";

export const meta = definePrototype({
  title: "Session links · In the header",
  route: "/sessions/$id",
  states: [
    { id: "none", label: "Nothing linked yet" },
    { id: "one-pr", label: "One pull request" },
    { id: "mixed-rest", label: "Several PRs, at rest" },
    { id: "mixed-open", label: "Several PRs, list open" },
    { id: "worktree-no-pr", label: "Worktree, no pull request yet" },
    { id: "worktree-removed", label: "Worktree removed, PR alive" },
    { id: "lookup-failed", label: "Can't reach GitHub" },
    { id: "added-by", label: "Added by you, by the agent, by the session" },
    { id: "runs-here", label: "Runs here vs. linked" },
    { id: "unlink", label: "Unlink" },
    { id: "details-failing", label: "Details, checks failing" },
    { id: "details-pending", label: "Details, checks running" },
    { id: "details-ready", label: "Details, ready to merge" },
    { id: "details-stale", label: "Details, GitHub out of date" },
    { id: "details-merged", label: "Details, merged" },
    { id: "details-branch", label: "Details, PR status unknown" },
    { id: "orchestrator", label: "Orchestrator with 12 links" },
    { id: "fifty", label: "50 links" },
    { id: "narrow", label: "Narrow window" },
  ],
});

export default function SessionLinksHeaderDraft({ state }: PrototypeProps) {
  return <HeaderDirection key={state} state={state} />;
}
