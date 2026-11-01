import { useRef, useState } from "react";

import type { RuntimeSelectorValue } from "@/systems/runtime";

import { draftAgentInstructions } from "../adapters/agent-instance-api";

export type AgentFillStatus = "idle" | "generating" | "applied" | "failed";

/** Fill with AI's popover state. The model follows the agent's until someone picks another. */
export function useAgentFillWithAi({
  value,
  onChange,
  runtime,
}: {
  value: string;
  onChange: (next: string) => void;
  runtime: RuntimeSelectorValue;
}) {
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<AgentFillStatus>("idle");
  const [brief, setBrief] = useState("");
  const [picked, setPicked] = useState<RuntimeSelectorValue | null>(null);
  const [previous, setPrevious] = useState("");
  const pending = useRef<AbortController | null>(null);
  const model = picked ?? runtime;

  const finish = () => {
    setStatus("idle");
    setBrief("");
    setOpen(false);
  };

  const start = async () => {
    const controller = new AbortController();
    pending.current = controller;
    setPrevious(value);
    setStatus("generating");
    try {
      const instructions = await draftAgentInstructions(
        {
          goal: brief.trim(),
          runtime: { provider: model.provider, ...(model.model ? { model: model.model } : {}) },
        },
        controller.signal
      );
      if (controller.signal.aborted) return;
      onChange(instructions);
      setStatus("applied");
    } catch {
      if (!controller.signal.aborted) setStatus("failed");
    }
  };

  return {
    open,
    status,
    brief,
    model,
    setBrief,
    setModel: setPicked,
    onOpenChange: (next: boolean) => (next || status !== "applied" ? setOpen(next) : finish()),
    start,
    stop: () => {
      pending.current?.abort();
      setStatus("idle");
    },
    undo: () => {
      onChange(previous);
      setStatus("idle");
    },
    finish,
  };
}
