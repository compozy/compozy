import type { RawLoopNode } from "./codec";
import { idField } from "./loop-node-fields";
import { gateExpiryFold } from "./loop-node-wait-fields";
import type { FieldSpec } from "./loop-node-schema-types";

export function askFields(_raw: RawLoopNode): FieldSpec[] {
  return [
    idField(),
    {
      type: "textarea",
      key: "ask_prompt",
      label: "Prompt",
      path: ["params", "prompt"],
      required: true,
      reference: true,
      hint: "What the responder is being asked. Rendered once when the request opens.",
    },
    {
      type: "text",
      key: "ask_context",
      label: "Context",
      path: ["params", "context"],
      json: true,
      hint: "Extra facts shown with the prompt. Redacted and bounded before it reaches any responder.",
    },
    {
      type: "text",
      key: "ask_expect",
      label: "Expected answer shape",
      path: ["params", "expect"],
      json: true,
      required: true,
      hint: "JSON Schema the answer must satisfy. The run page renders its form from this shape, and the daemon rejects an answer that does not match.",
    },
    {
      type: "switch",
      key: "ask_responders_agents",
      label: "Agents may answer",
      path: ["params", "responders", "agents"],
      subLabel: "Operators may always answer. Agents are denied unless this node opts in.",
    },
    gateExpiryFold(_raw, ["params", "expires"], "ask"),
    {
      type: "hint",
      key: "hint",
      hint: "Parks the run until a human or a permitted agent answers. The answer becomes this node's output and downstream nodes read it.",
    },
  ];
}
