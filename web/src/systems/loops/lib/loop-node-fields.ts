import type { RawLoopNode } from "./codec";
import { LOOP_CEILINGS } from "./loop-limits";
import type { FieldSpec, TextFieldSpec } from "./loop-node-schema-types";

/**
 * The per class/kind inspector field builders. Each returns the `FieldSpec[]` the
 * inspector renders for one node kind, derived from the canonical `agh.loop/v1` DSL types
 * + the ADR-021 reserved kinds. Where the design mockup diverged (`action_ref`, a closed
 * 3-item kind select), the DSL wins — those fields do not exist here. Editable fields carry
 * a `path` into the raw node JSON so a single immutable setter (loop-editor-draft) applies
 * every edit and the bijective codec round-trips the rest.
 */

export function str(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function idField(): TextFieldSpec {
  return { type: "text", key: "id", label: "Node id", path: ["id"], mono: true };
}

export function inputFields(raw: RawLoopNode, inputNames: string[]): FieldSpec[] {
  const current = str(raw.input_ref);
  // Editable select over the declared inputs (ADR-023: input_ref must name a declared
  // input). Keep the current value selectable even if it is not (yet) a declared input, and
  // fall back to a free-text field when the Loop declares none.
  const options = current && !inputNames.includes(current) ? [current, ...inputNames] : inputNames;
  const refField: FieldSpec =
    options.length > 0
      ? {
          type: "select",
          key: "input_ref",
          label: "Input ref",
          path: ["input_ref"],
          options,
          hint: "The declared loop input this node exposes into the graph.",
        }
      : {
          type: "text",
          key: "input_ref",
          label: "Input ref",
          path: ["input_ref"],
          mono: true,
          hint: "Must name a declared input — add inputs in the Loop's inputs block first.",
        };
  return [
    idField(),
    refField,
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${str(raw.id)}.output`,
      badge: "derived",
    },
    {
      type: "hint",
      key: "hint",
      hint: "Exposes a declared typed input into the graph; produces derives from the input's declared type.",
    },
  ];
}

export function fileImportFields(raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "pattern",
      label: "Pattern",
      path: ["pattern"],
      mono: true,
      reference: true,
      hint: "Template over the loop namespace. Globs workspace files at generation start.",
    },
    {
      type: "select",
      key: "parse",
      label: "Parse",
      path: ["parse"],
      options: ["json", "text"],
    },
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${str(raw.id)}.output`,
      badge: "derived",
    },
    {
      type: "hint",
      key: "hint",
      hint: "Large payloads are never inlined: files land in the content-addressed store and the output carries artifact refs.",
    },
  ];
}

export function watchSourceFields(raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "textarea",
      key: "watch",
      label: "Watch spec",
      path: ["watch"],
      mono: true,
      json: true,
      hint: "The poll → ready → settle → confirm watch specification (ADR-016). Bridge-backed source.",
    },
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${str(raw.id)}.output`,
      badge: "derived",
    },
  ];
}

export function watchEventsFields(raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "events",
      key: "events",
      label: "Subscriptions",
      path: ["events"],
      hint: "One or more internal-event subscriptions. Each names a supported hook kind and an optional CEL filter over `event`, `inputs`, and `nodes`.",
    },
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${str(raw.id)}.output`,
      badge: "derived",
    },
    {
      type: "hint",
      key: "hint",
      hint: "Parks the loop at zero cost until a subscribed internal event commits, then wakes and re-derives the matched batch from the durable ledger. A silent subscription is healthy dormancy — a watch-events loop never stalls on silence.",
    },
  ];
}

export function fanOutFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "collection",
      label: "Collection",
      path: ["collection"],
      mono: true,
      reference: true,
      hint: "Template resolving to a finite collection. Validated against the source's declared output schema at publish.",
    },
    { type: "number", key: "batch_size", label: "Batch size", path: ["batch_size"] },
    { type: "number", key: "max_parallel", label: "Max parallel", path: ["max_parallel"] },
    {
      type: "number",
      key: "max_fan_out",
      label: "Max fan-out",
      path: ["max_fan_out"],
      ceiling: LOOP_CEILINGS.fanOutBreadth,
      hint: "Structural cap on materialized branches. Capped by the daemon ceiling.",
    },
    {
      type: "hint",
      key: "hint",
      hint: "Fans the collection into batch branches; item and index enter scope inside the branch body. Pair with a collect barrier.",
    },
  ];
}

export function collectFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    { type: "static", key: "joins", label: "Joins", value: "paired fan-out", badge: "barrier" },
    {
      type: "hint",
      key: "hint",
      hint: "Barrier that waits for every fanned branch before the run continues.",
    },
  ];
}

export function gateFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    { type: "criteria", key: "criteria", label: "Criteria", path: ["criteria"] },
    {
      type: "select",
      key: "verdict_policy",
      label: "Verdict policy",
      path: ["verdict_policy"],
      options: ["revise_until_clean", "fixed_passes"],
      hint: "revise_until_clean requires an agent-judge or human criterion as the verdict source.",
    },
    {
      type: "text",
      key: "on_pass",
      label: "On pass → target",
      path: ["on_result", "pass"],
      mono: true,
      hint: "A node id or terminal state (done/failed/…) the run routes to on a passing verdict.",
    },
    {
      type: "text",
      key: "on_fail",
      label: "On fail → target",
      path: ["on_result", "fail"],
      mono: true,
    },
    {
      type: "number",
      key: "max_revisions",
      label: "Max revisions",
      path: ["max_revisions"],
      ceiling: LOOP_CEILINGS.gateMaxRevisions,
    },
    {
      type: "hint",
      key: "hint",
      hint: "The judge emits a structured verdict; malformed output degrades to revise, never to pass.",
    },
  ];
}

export function branchFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "condition",
      label: "Condition",
      path: ["condition"],
      mono: true,
      reference: true,
      cel: true,
      hint: "A CEL boolean over the loop namespace, autocompleted from the compiled reference schema (ADR-020). The visual builder is deferred.",
    },
  ];
}

export function subLoopFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "static",
      key: "body",
      label: "Nested body",
      value: "inline sub-loop",
      badge: "nested",
    },
    {
      type: "hint",
      key: "hint",
      hint: "An in-graph nested body (ADR-004). Cross-definition composition uses the run-loop action instead.",
    },
  ];
}

export function runAgentFields(raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "agent",
      label: "Agent",
      path: ["params", "agent"],
      mono: true,
      required: true,
      reference: true,
      hint: "Interpolable agent profile ref; the profile stays the identity and config authority.",
    },
    {
      type: "textarea",
      key: "prompt",
      label: "Prompt",
      path: ["params", "prompt"],
      mono: true,
      required: true,
      reference: true,
      hint: "The node's work order, a template over the loop namespace. Type {{ to autocomplete references.",
    },
    {
      type: "textarea",
      key: "output_schema",
      label: "Output schema",
      path: ["params", "output_schema"],
      mono: true,
      json: true,
      optionalLabel: "optional · JSON Schema",
      hint: "Structured harvest. A validation failure feeds one FREE retry before the retry budget is touched.",
    },
    {
      type: "fold",
      key: "overrides",
      label: "Overrides",
      subLabel: "model · allowed_tools · max_turns",
      fields: [
        {
          type: "text",
          key: "model",
          label: "model",
          path: ["params", "model"],
          mono: true,
          placeholder: "profile default",
        },
        {
          type: "text",
          key: "allowed_tools",
          label: "allowed_tools",
          path: ["params", "allowed_tools"],
          mono: true,
          json: true,
          placeholder: "profile default",
        },
        { type: "number", key: "max_turns", label: "max_turns", path: ["params", "max_turns"] },
      ],
    },
    {
      type: "text",
      key: "cwd",
      label: "Working dir",
      path: ["params", "cwd"],
      mono: true,
      reference: true,
      optionalLabel: "optional",
      placeholder: "session default",
      hint: "Optional; part of the session binding key (ADR-021).",
    },
    {
      type: "switch",
      key: "isolated",
      label: "Isolated session",
      path: ["session", "isolated"],
      subLabel: "Each task runs in its own one-shot session.",
    },
    { type: "text", key: "timeout", label: "Timeout", path: ["timeout"], mono: true },
    {
      type: "number",
      key: "retry",
      label: "Retry (max)",
      path: ["retry", "max"],
      hint: "Plus one FREE schema-validation retry outside this budget (ADR-021).",
    },
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${str(raw.id)}.output`,
      badge: "from schema",
    },
  ];
}

export function runLoopFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "loop",
      label: "Loop",
      path: ["params", "loop"],
      mono: true,
      required: true,
      reference: true,
      hint: "The child loop name, interpolable so the executor swaps per run. Ancestry is cycle-guarded (depth ≤ 8).",
    },
    {
      type: "select",
      key: "mode",
      label: "Mode",
      path: ["params", "mode"],
      options: ["await", "detach"],
    },
    {
      type: "textarea",
      key: "inputs",
      label: "Inputs",
      path: ["params", "inputs"],
      mono: true,
      json: true,
      optionalLabel: "optional · template-interpolated map",
      hint: "The child loop's declared inputs, each value template-interpolated over this loop's namespace.",
    },
    {
      type: "hint",
      key: "hint",
      hint: "await yields-and-rewakes (never a held lease) until the child terminates; detach returns the child run id immediately.",
    },
  ];
}

export function transformFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "textarea",
      key: "map",
      label: "Map",
      path: ["params", "map"],
      mono: true,
      json: true,
      hint: "Pure declarative reshaping: each output key is { from } | { value } | { template }. No side effects.",
    },
  ];
}

export function toolActionFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "text",
      key: "kind",
      label: "Tool",
      path: ["kind"],
      mono: true,
      required: true,
      hint: "The literal ToolID this action calls (e.g. agh__network_send). Reserved names are run-agent / run-loop / transform.",
    },
    {
      type: "textarea",
      key: "params",
      label: "Params",
      path: ["params"],
      mono: true,
      json: true,
      hint: "The tool's input arguments, template-interpolated. A schema-generated form renders here once the registry input-schema read is wired.",
    },
    {
      type: "hint",
      key: "hint",
      hint: "An action's kind IS the tool it calls. Optional harvest (e.g. channel_result on agh__network_send) is declared in the DSL view.",
    },
  ];
}

/** The id + kind static fallback for an unrecognized class/kind. */
export function fallbackFields(raw: RawLoopNode): FieldSpec[] {
  const nodeClass = str(raw.class);
  const kind = str(raw.kind);
  return [
    idField(),
    { type: "static", key: "kind", label: "Kind", value: kind || "—", badge: nodeClass || "node" },
  ];
}
