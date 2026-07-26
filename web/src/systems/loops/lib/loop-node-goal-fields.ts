import type { RawLoopNode } from "./codec";
import type { FieldSpec } from "./loop-node-schema-types";

function nodeID(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

export function goalFields(raw: RawLoopNode): FieldSpec[] {
  return [
    { type: "text", key: "id", label: "Node id", path: ["id"], mono: true },
    {
      type: "text",
      key: "agent",
      label: "Agent",
      path: ["params", "agent"],
      mono: true,
      required: true,
      reference: true,
    },
    {
      type: "textarea",
      key: "objective",
      label: "Objective",
      path: ["params", "objective"],
      required: true,
      reference: true,
    },
    {
      type: "criteria",
      key: "judge",
      label: "Judge criteria",
      path: ["params", "judge"],
      allowedTypes: ["command", "agent-judge", "extension"],
      hint: "Goal v1 rejects human criteria; use command, agent-judge, or extension.",
    },
    {
      type: "number",
      key: "max_turns",
      label: "Max turns",
      path: ["params", "max_turns"],
    },
    {
      type: "select",
      key: "on_exhausted",
      label: "On exhausted",
      path: ["params", "on_exhausted"],
      options: ["halt", "escalate"],
    },
    {
      type: "select",
      key: "session_mode",
      label: "Session mode",
      path: ["session", "mode"],
      options: ["continuous"],
      hint: "Goal turns reuse one durable continuous session binding.",
    },
    {
      type: "fold",
      key: "retry",
      label: "Operational retry",
      subLabel: "pre-submit only",
      fields: [
        {
          type: "number",
          key: "max_attempts",
          label: "Max attempts",
          path: ["retry", "max_attempts"],
        },
        {
          type: "select",
          key: "on_failure",
          label: "On failure",
          path: ["retry", "on_failure"],
          options: ["fresh_session"],
        },
      ],
    },
    {
      type: "textarea",
      key: "output_schema",
      label: "Output schema",
      path: ["params", "output_schema"],
      mono: true,
      json: true,
      optionalLabel: "optional · JSON Schema",
    },
    {
      type: "static",
      key: "produces",
      label: "Produces",
      value: `nodes.${nodeID(raw.id)}.output`,
      badge: "goal result",
    },
  ];
}
