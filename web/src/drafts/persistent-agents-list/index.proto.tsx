import { Button } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  LISTED_EPHEMERAL,
  LISTED_INVALID,
  LISTED_MANY,
  LISTED_NEEDS_YOU,
  LISTED_PAUSED,
  LISTED_PERSISTENT,
  LISTED_SAME_SLUG,
} from "../persistent-agents-shared/agents-fixtures";
import { AgentsPage } from "../persistent-agents-shared/agents-page";

export const meta = definePrototype({
  title: "Persistent agents list",
  spec: "always-on-agent",
  surface: "S2",
  route: "/agents",
  states: [
    { id: "cards", label: "Cards" },
    { id: "rows", label: "Rows" },
    { id: "no-persistent", label: "No persistent agents" },
    { id: "same-slug", label: "Same slug, different definitions" },
    { id: "invalid", label: "Invalid agent" },
    { id: "empty-filter", label: "Empty filter" },
    { id: "paused", label: "Paused" },
    { id: "needs-you", label: "Needs you" },
    { id: "many", label: "500 agents" },
  ],
});

export default function PersistentAgentsListDraft({ state }: PrototypeProps) {
  switch (state) {
    case "rows":
      return (
        <AgentsPage
          ephemeral={LISTED_EPHEMERAL}
          initialView="rows"
          persistent={LISTED_PERSISTENT}
        />
      );
    case "no-persistent":
      return <AgentsPage ephemeral={LISTED_EPHEMERAL} persistent={[]} />;
    case "same-slug":
      return <AgentsPage ephemeral={LISTED_SAME_SLUG} persistent={LISTED_PERSISTENT} />;
    case "invalid":
      return <AgentsPage ephemeral={LISTED_INVALID} persistent={LISTED_PERSISTENT} />;
    case "empty-filter":
      return <AgentsPage ephemeral={[]} persistent={[]} query="kubernetes" />;
    case "paused":
      return <AgentsPage ephemeral={LISTED_EPHEMERAL} persistent={LISTED_PAUSED} />;
    case "needs-you":
      return <AgentsPage ephemeral={LISTED_EPHEMERAL} persistent={LISTED_NEEDS_YOU} />;
    case "many":
      return (
        <AgentsPage
          counts={{ persistent: 12, ephemeral: 488 }}
          ephemeral={LISTED_MANY}
          footer={
            <div className="flex justify-center">
              <Button size="sm" type="button" variant="ghost">
                Load more
              </Button>
            </div>
          }
          initialView="rows"
          persistent={LISTED_PERSISTENT}
        />
      );
    default:
      return <AgentsPage ephemeral={LISTED_EPHEMERAL} persistent={LISTED_PERSISTENT} />;
  }
}
