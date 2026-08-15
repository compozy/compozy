import { LOOP_WATCH_EVENT_KINDS } from "@/generated/loop-enums";

import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import { effectsField } from "./loop-node-lifecycle-fields";
import { idField } from "./loop-node-fields";
import { WAIT_MODES, type FieldSpec, type LoopWaitMode } from "./loop-node-schema-types";

/**
 * Wait and gate-expiry fields. A wait mode switch writes its chosen discriminator and clears the
 * other two in one transition because the DSL requires exactly one of for, until, or event.
 */

const DEFAULT_WAIT_FOR = "1h";
const DEFAULT_EVENT_KIND = LOOP_WATCH_EVENT_KINDS[0];

/** Presence selects the mode; the daemon judges whether its value is complete. */
function declared(raw: RawLoopNode, mode: LoopWaitMode): boolean {
  const value = getAtPath(raw, ["params", mode]);
  return value !== undefined && value !== null;
}

/** The discriminator this node declares, or `null` when it declares none (or several). */
export function waitMode(raw: RawLoopNode): LoopWaitMode | null {
  const present = WAIT_MODES.filter(mode => declared(raw, mode));
  return present.length === 1 ? present[0] : null;
}

/** Keeps the selected wait value (or seeds it) and clears both competing discriminators. */
export function waitModeEdits(raw: RawLoopNode, mode: LoopWaitMode): NodeFieldEdit[] {
  return WAIT_MODES.map(candidate => {
    if (candidate !== mode) return { path: ["params", candidate], value: undefined };
    const carried = getAtPath(raw, ["params", candidate]);
    if (carried !== undefined && carried !== null) {
      return { path: ["params", candidate], value: carried };
    }
    if (candidate === "for") return { path: ["params", "for"], value: DEFAULT_WAIT_FOR };
    if (candidate === "until") return { path: ["params", "until"], value: "" };
    return { path: ["params", "event"], value: { kind: DEFAULT_EVENT_KIND } };
  });
}

function expiryFields(basePath: string[], owner: string): FieldSpec[] {
  return [
    {
      type: "text",
      key: `${owner}_expires_after`,
      label: "expires.after",
      path: [...basePath, "after"],
      mono: true,
      placeholder: "24h",
      hint: "Required once an expiry is declared. Positive duration.",
    },
    effectsField(`${owner}_expires_escalate`, "expires.escalate", [...basePath, "escalate"]),
    {
      type: "text",
      key: `${owner}_expires_route`,
      label: "expires.route",
      path: [...basePath, "route"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "node id",
      hint: "With neither escalate nor route, expiry only flags needs-attention — the linter warns (wait_expiry_without_path) but never blocks Publish.",
    },
  ];
}

/** The `expires` fold on a human gate (`dsl.Node.Expires`, node root — not params). */
export function gateExpiryFold(_raw: RawLoopNode): FieldSpec {
  return {
    type: "fold",
    key: "gate_expiry",
    label: "Expiry & escalation",
    subLabel: "expires.after · escalate · route",
    fields: expiryFields(["expires"], "gate"),
  };
}

function waitModeInputs(mode: LoopWaitMode | null): FieldSpec[] {
  if (mode === "for") {
    return [
      {
        type: "text",
        key: "wait_for",
        label: "for",
        path: ["params", "for"],
        mono: true,
        required: true,
        placeholder: "24h",
        hint: "A relative timer. The run parks here with its clocks suspended.",
      },
    ];
  }
  if (mode === "until") {
    return [
      {
        type: "text",
        key: "wait_until",
        label: "until",
        path: ["params", "until"],
        mono: true,
        required: true,
        reference: true,
        placeholder: "2026-09-01T00:00:00Z",
        hint: "An absolute timestamp, interpolable over the loop namespace.",
      },
    ];
  }
  if (mode === "event") {
    return [
      {
        type: "select",
        key: "wait_event_kind",
        label: "event.kind",
        path: ["params", "event", "kind"],
        options: [...LOOP_WATCH_EVENT_KINDS],
        hint: "One supported internal event kind — the same catalog Watch events subscribes to.",
      },
      {
        type: "text",
        key: "wait_event_filter",
        label: "event.filter",
        path: ["params", "event", "filter"],
        mono: true,
        cel: true,
        reference: true,
        optionalLabel: "optional · CEL",
        placeholder: "event.payload.deploy_id != ''",
      },
    ];
  }
  return [
    {
      type: "hint",
      key: "wait_mode_missing",
      hint: "Pick a wait mode. A wait node must declare exactly one of for, until or event (wait_shape_invalid).",
    },
  ];
}

/** The full `wait` inspector: mode XOR, its input, expect, ahead_arrival, and expiry. */
export function waitFields(raw: RawLoopNode): FieldSpec[] {
  const mode = waitMode(raw);
  return [
    idField(),
    {
      type: "wait-mode",
      key: "wait_mode",
      label: "Wait on",
      basePath: ["params"],
      mode,
      hint: "Exactly one of for · until · event. Switching clears the other two in the same edit.",
    },
    ...waitModeInputs(mode),
    {
      type: "textarea",
      key: "wait_expect",
      label: "expect",
      path: ["params", "expect"],
      mono: true,
      json: true,
      optionalLabel: "optional · JSON Schema",
      hint: "Resume payloads — event or manual resume — validate against this schema. Single-claim.",
    },
    {
      type: "select",
      key: "wait_ahead_arrival",
      label: "ahead_arrival",
      path: ["params", "ahead_arrival"],
      options: ["consume_on_entry", "reject"],
      clearOption: { value: "", label: "engine default" },
      hint: "What happens when the event already arrived before the cell entered the wait.",
    },
    {
      type: "fold",
      key: "wait_expiry",
      label: "Expiry",
      subLabel: "expires.after · escalate · route",
      defaultOpen: true,
      fields: expiryFields(["params", "expires"], "wait"),
    },
    {
      type: "hint",
      key: "wait_hint",
      hint: "A durable pause: the run parks here with clocks suspended and stays visible on the run page.",
    },
  ];
}
