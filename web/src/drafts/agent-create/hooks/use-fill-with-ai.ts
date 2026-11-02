/** Fill with AI's popover state. The model follows the agent's until someone picks another. */
import { useEffect, useRef, useState } from "react";

import { FILL_SYSTEM_PROMPT, writeInstructions, type Draft } from "../model";

const WRITE_MS = 1600;

export type FillStatus = "idle" | "generating" | "applied" | "failed";

export interface FillPreset {
  status: FillStatus;
  brief: string;
}

export function useFillWithAi({
  value,
  onChange,
  runtime,
  preset,
}: {
  value: string;
  onChange: (next: string) => void;
  runtime: Draft["runtime"];
  preset?: FillPreset;
}) {
  const [open, setOpen] = useState(preset !== undefined);
  const [status, setStatus] = useState<FillStatus>(preset?.status ?? "idle");
  const [brief, setBrief] = useState(preset?.brief ?? "");
  const [picked, setPicked] = useState<Draft["runtime"] | null>(null);
  const [previous, setPrevious] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const model = picked ?? runtime;

  useEffect(() => () => clearTimeout(timer.current), []);

  const finish = () => {
    setStatus("idle");
    setBrief("");
    setOpen(false);
  };

  return {
    open,
    status,
    brief,
    model,
    setBrief,
    setModel: setPicked,
    onOpenChange: (next: boolean) => (next || status !== "applied" ? setOpen(next) : finish()),
    start: () => {
      setPrevious(value);
      setStatus("generating");
      timer.current = setTimeout(() => {
        onChange(writeInstructions({ system: FILL_SYSTEM_PROMPT, brief, runtime: model }));
        setStatus("applied");
      }, WRITE_MS);
    },
    stop: () => {
      clearTimeout(timer.current);
      setStatus("idle");
    },
    undo: () => {
      onChange(previous);
      setStatus("idle");
    },
    finish,
  };
}
