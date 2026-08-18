import type { RawLoopNode } from "./codec";
import type { LoopNodeClass } from "./loop-graph";

/**
 * The "Add node" palette (design §4.6), grouped by class. Action carries the three
 * ADR-021 reserved kinds, a "Channel post" shortcut that inserts a pre-filled
 * `compozy__network_send` node, and a "Call tool…" entry that adds a blank ToolID action
 * the inspector fills. Control/Source list their closed kinds. Each item seeds the raw
 * node with its DSL-shaped defaults so a dropped node is valid-shaped (though the
 * linter still gates reachability until the author connects it — fork-and-edit).
 */

export interface PaletteItem {
  label: string;
  /** The kind badge shown on the palette row (a ToolID for tool actions). */
  kindLabel: string;
  nodeClass: LoopNodeClass;
  /** Builds the seed raw node for a generated id. */
  buildRaw: (id: string) => RawLoopNode;
  /** Base for the generated snake_case node id. */
  idBase: string;
  hint?: string;

  keywords?: string[];
}

export interface PaletteGroup {
  label: string;
  items: PaletteItem[];
}

export const LOOP_PALETTE: PaletteGroup[] = [
  {
    label: "Action",
    items: [
      {
        label: "Run agent",
        kindLabel: "run-agent",
        nodeClass: "action",
        idBase: "run_agent",
        buildRaw: id => ({
          id,
          class: "action",
          kind: "run-agent",
          params: { agent: "", prompt: "" },
          session: { isolated: true },
          timeout: "30m",
          // `retry.max` is retired (`retry_max_unsupported`); `max_attempts` is the key the
          // runtime resolves into the attempt budget (`lifecycle_config.go`).
          retry: { max_attempts: 2 },
        }),
      },
      {
        label: "Goal",
        kindLabel: "goal",
        nodeClass: "action",
        idBase: "goal",
        hint: "Advances one continuous agent session until an external judge approves or the limit is reached.",
        buildRaw: id => ({
          id,
          class: "action",
          kind: "goal",
          params: {
            agent: "",
            objective: "",
            judge: [{ id: "criterion_1", type: "command", check: "" }],
            max_turns: 20,
            on_exhausted: "halt",
          },
          session: { mode: "continuous" },
          retry: { max_attempts: 2, on_failure: "fresh_session" },
        }),
      },
      {
        label: "Run loop",
        kindLabel: "run-loop",
        nodeClass: "action",
        idBase: "run_loop",
        buildRaw: id => ({
          id,
          class: "action",
          kind: "run-loop",
          params: { loop: "", mode: "await" },
        }),
      },
      {
        label: "Transform",
        kindLabel: "transform",
        nodeClass: "action",
        idBase: "transform",
        buildRaw: id => ({ id, class: "action", kind: "transform", params: { map: {} } }),
      },
      {
        label: "Channel post",
        kindLabel: "compozy__network_send",
        nodeClass: "action",
        idBase: "channel_post",
        hint: "A pre-filled compozy__network_send node (the retired channel-post primitive, ADR-021).",
        buildRaw: id => ({ id, class: "action", kind: "compozy__network_send", params: {} }),
      },
      {
        label: "Call tool…",
        // Intent-revealing (the seeded node has kind ""); not a literal kind like the others.
        kindLabel: "tool…",
        nodeClass: "action",
        idBase: "call_tool",
        hint: "Adds a blank ToolID action; set the tool in the inspector.",
        buildRaw: id => ({ id, class: "action", kind: "", params: {} }),
      },
    ],
  },
  {
    label: "Control",
    items: [
      {
        label: "Fan-out",
        kindLabel: "fan-out",
        nodeClass: "control",
        idBase: "fan_out",
        buildRaw: id => ({
          id,
          class: "control",
          kind: "fan-out",
          collection: "",
          batch_size: 1,
          max_parallel: 1,
          max_fan_out: 8,
        }),
      },
      {
        label: "Collect",
        kindLabel: "collect",
        nodeClass: "control",
        idBase: "collect",
        buildRaw: id => ({ id, class: "control", kind: "collect" }),
      },
      {
        label: "Branch",
        kindLabel: "branch",
        nodeClass: "control",
        idBase: "branch",
        buildRaw: id => ({ id, class: "control", kind: "branch", condition: "" }),
      },
      {
        label: "Gate",
        kindLabel: "gate",
        nodeClass: "control",
        idBase: "gate",
        buildRaw: id => ({
          id,
          class: "control",
          kind: "gate",
          criteria: [],
          verdict_policy: "fixed_passes",
          on_result: {},
        }),
      },
      {
        label: "Sub-loop",
        kindLabel: "sub-loop",
        nodeClass: "control",
        idBase: "sub_loop",
        buildRaw: id => ({ id, class: "control", kind: "sub-loop" }),
      },
      {
        label: "Wait",
        kindLabel: "wait",
        nodeClass: "control",
        idBase: "wait",
        hint: "A durable timer, timestamp, or event pause. The run parks here with its clocks suspended.",
        // Seeded with exactly one discriminator — a wait declaring zero or several is
        // `wait_shape_invalid`, so a dropped node is valid-shaped from the first render.
        buildRaw: id => ({ id, class: "control", kind: "wait", params: { for: "1h" } }),
      },
      {
        label: "Ask",
        kindLabel: "ask",
        nodeClass: "control",
        idBase: "ask",
        keywords: ["human", "question", "answer", "request", "input"],
        hint: "Parks the run until a person answers. The answer becomes this node's output.",

        buildRaw: id => ({
          id,
          class: "control",
          kind: "ask",
          params: { prompt: "", expect: { type: "object" } },
        }),
      },
      {
        label: "Route",
        kindLabel: "route",
        nodeClass: "control",
        idBase: "route",
        keywords: ["branch", "condition", "switch", "when", "default"],
        hint: "Sends the run down exactly one forward branch. A default is required.",

        buildRaw: id => ({ id, class: "control", kind: "route", routes: [], default: "" }),
      },
    ],
  },
  {
    label: "Source",
    items: [
      {
        label: "Watch source",
        kindLabel: "watch-source",
        nodeClass: "source",
        idBase: "watch_source",
        buildRaw: id => ({ id, class: "source", kind: "watch-source", watch: {} }),
      },
      {
        label: "Watch events",
        kindLabel: "watch-events",
        nodeClass: "source",
        idBase: "watch_events",
        hint: "Parks the loop until a subscribed internal CompozyOS event (task status, loop terminal, …) commits, matched by a CEL filter.",
        buildRaw: id => ({ id, class: "source", kind: "watch-events", events: [] }),
      },
      {
        label: "File import",
        kindLabel: "file-import",
        nodeClass: "source",
        idBase: "file_import",
        buildRaw: id => ({
          id,
          class: "source",
          kind: "file-import",
          pattern: "",
          parse: "json",
        }),
      },
      {
        label: "Input",
        kindLabel: "input",
        nodeClass: "source",
        idBase: "input",
        buildRaw: id => ({ id, class: "source", kind: "input", input_ref: "" }),
      },
    ],
  },
];

export function paletteKindKey(kindLabel: string): string {
  return kindLabel === "tool…" ? "" : kindLabel;
}

export function loopPaletteItems(): PaletteItem[] {
  return LOOP_PALETTE.flatMap(group => group.items);
}

export function filterPaletteItems(items: readonly PaletteItem[], query: string): PaletteItem[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") return [...items];
  return items.filter(item =>
    [item.label, item.kindLabel, item.hint ?? "", ...(item.keywords ?? [])]
      .join(" ")
      .toLowerCase()
      .includes(needle)
  );
}

/** Generates a snake_case node id from a base, suffixing `_N` until it is unique. */
export function uniqueNodeId(base: string, existing: ReadonlySet<string>): string {
  const normalized =
    base
      .toLowerCase()
      .replace(/[^a-z0-9_]/g, "_")
      .replace(/^([0-9])/, "n$1") || "node";
  if (!existing.has(normalized)) return normalized;
  let index = 2;
  while (existing.has(`${normalized}_${index}`)) index += 1;
  return `${normalized}_${index}`;
}
