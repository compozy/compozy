import { definePrototype, type PrototypeProps } from "@compozy/proto";
import type { ReactNode } from "react";

import { Markdown } from "@compozy/ui";

import { SubagentComposerControl } from "./subagent-composer-control";
import { subagentCounts } from "./subagent-format";
import { CROWDED_FLEET, FAILED_FLEET, WORKING_FLEET } from "./stories/subagent-panel-fixtures";
import type { SubagentView } from "./types";

export const meta = definePrototype({
  title: "Subagent panel",
  surface: "Composer subagent control",
  route: "/session",
  states: [
    { id: "list", label: "List · Working" },
    { id: "failed", label: "List · One failed" },
    { id: "crowd", label: "List · Twelve at once" },
    { id: "open-working", label: "Opened · Working" },
    { id: "open-done", label: "Opened · Finished" },
    { id: "open-failed", label: "Opened · Failed" },
    { id: "open-queued", label: "Opened · Queued" },
    { id: "closed", label: "Closed" },
  ],
});

function content(subagent: SubagentView): ReactNode {
  if (!subagent.result) return undefined;
  return <Markdown>{subagent.result}</Markdown>;
}

function control(subagents: readonly SubagentView[], selected?: string, open = true) {
  return (
    <SubagentComposerControl
      counts={subagentCounts(subagents)}
      defaultOpen={open}
      defaultSelectedId={selected}
      onOpen={() => undefined}
      onStop={() => Promise.resolve()}
      renderContent={content}
      subagents={subagents}
    />
  );
}

export default function Prototype({ state }: PrototypeProps) {
  switch (state) {
    case "failed":
      return control(FAILED_FLEET);
    case "crowd":
      return control(CROWDED_FLEET);
    case "open-working":
      return control(WORKING_FLEET, "map");
    case "open-done":
      return control(WORKING_FLEET, "usage");
    case "open-failed":
      return control(FAILED_FLEET, "e2e");
    case "open-queued":
      return control(WORKING_FLEET, "browser");
    case "closed":
      return control(WORKING_FLEET, undefined, false);
    default:
      return control(WORKING_FLEET);
  }
}
