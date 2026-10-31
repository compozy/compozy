import { PageContent } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SESSION_PULL_REQUEST_CHIP_STATES } from "../mocks/session-pull-request-fixtures";
import { SessionPullRequestChip } from "./session-pull-request-chip";

export const meta = definePrototype({
  title: "Session pull request chip",
  spec: "sessions-kanban",
  surface: "S1",
  route: "/sessions/$id",
  states: [
    { id: "lookup-pending", label: "Lookup pending" },
    { id: "branch", label: "Branch, no PR" },
    { id: "unavailable", label: "Unavailable" },
    { id: "incomplete", label: "Incomplete" },
    { id: "checks-pending", label: "Checks pending" },
    { id: "checks-failing", label: "Checks failing" },
    { id: "changes-requested", label: "Changes requested" },
    { id: "draft", label: "Draft" },
    { id: "ready-to-merge", label: "Ready to merge" },
    { id: "conflicts", label: "Conflicts" },
    { id: "merged", label: "Merged" },
    { id: "closed", label: "Closed" },
    { id: "no-checks", label: "No checks" },
    { id: "stale", label: "Stale facts" },
    { id: "refreshing", label: "Refreshing" },
  ],
});

const ignore = () => undefined;

export default function SessionPullRequestChipPrototype({ state }: PrototypeProps) {
  const scenario =
    SESSION_PULL_REQUEST_CHIP_STATES.find(item => item.id === state) ??
    SESSION_PULL_REQUEST_CHIP_STATES[0];
  if (!scenario) throw new Error("No pull request chip fixtures");
  return (
    <PageContent density="compact">
      <SessionPullRequestChip
        controllable
        facts={scenario.facts}
        onPrompt={ignore}
        onRefresh={ignore}
        refreshing={scenario.refreshing ?? false}
      />
    </PageContent>
  );
}
