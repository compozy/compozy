import { Eyebrow, PageContent } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  issue1100FlatTreesFixture,
  issue1100GroupedTreesFixture,
  mixedToolTreesFixture,
  wrapperChainTreesFixture,
} from "../mocks/process-fixtures";
import type { SessionProcessTree } from "../lib/session-processes";
import { SessionProcessesComposerControl } from "./session-processes-composer-control";

export const meta = definePrototype({
  title: "Session processes",
  route: "/sessions/$id",
  states: [
    { id: "flat-before", label: "Before: flat, 13 rows" },
    { id: "grouped", label: "Grouped: Terminal + browser" },
    { id: "browser-expanded", label: "Browser tree expanded" },
    { id: "wrapper-chain", label: "Wrapper chains with ports" },
    { id: "tool-call", label: "Tool call running, Stop all" },
    { id: "stopping-tree", label: "Stopping a tree" },
    { id: "empty", label: "Empty" },
    { id: "badge", label: "Badge count before and after" },
  ],
});

const ignore = () => undefined;

interface Scenario {
  trees: readonly SessionProcessTree[];
  count: number;
  expanded?: readonly string[];
  stopping?: ReadonlySet<string>;
}

const browserTree = issue1100GroupedTreesFixture[1]!;

const scenarios: Record<string, Scenario> = {
  "flat-before": { trees: issue1100FlatTreesFixture, count: 13 },
  grouped: { trees: issue1100GroupedTreesFixture, count: 2 },
  "browser-expanded": {
    trees: issue1100GroupedTreesFixture,
    count: 2,
    expanded: [browserTree.id],
  },
  "wrapper-chain": { trees: wrapperChainTreesFixture, count: 2 },
  "tool-call": { trees: mixedToolTreesFixture, count: 2 },
  "stopping-tree": {
    trees: issue1100GroupedTreesFixture,
    count: 2,
    stopping: new Set([browserTree.id]),
  },
  empty: { trees: [], count: 1 },
};

export default function SessionProcessesComposerControlPrototype({ state }: PrototypeProps) {
  if (state === "badge") {
    return (
      <PageContent className="flex max-w-sm flex-col items-start gap-3" density="compact">
        <Eyebrow>Before · one per process</Eyebrow>
        <SessionProcessesComposerControl count={13} trees={[]} />
        <Eyebrow>After · one per tree</Eyebrow>
        <SessionProcessesComposerControl count={2} trees={[]} />
      </PageContent>
    );
  }
  const scenario = scenarios[state] ?? scenarios.grouped!;
  return (
    <PageContent
      className="flex min-h-[600px] max-w-md flex-col items-start justify-end"
      density="compact"
    >
      <SessionProcessesComposerControl
        defaultOpen
        count={scenario.count}
        trees={scenario.trees}
        defaultExpanded={scenario.expanded}
        stopping={scenario.stopping ?? null}
        onStop={ignore}
        onStopAll={ignore}
      />
    </PageContent>
  );
}
