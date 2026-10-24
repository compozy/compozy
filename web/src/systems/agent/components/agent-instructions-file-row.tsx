import type { ReactNode } from "react";

import { Eyebrow, PillGroup, type PillGroupItem } from "@compozy/ui";

import type { AgentInstructionFile } from "../lib/agent-detail-search";

const FILE_ITEMS: PillGroupItem<AgentInstructionFile>[] = [
  { value: "agent", label: <Eyebrow>AGENT.md</Eyebrow>, testId: "agent-file-tab-agent" },
  {
    value: "soul",
    label: <Eyebrow>SOUL.md</Eyebrow>,
    testId: "agent-file-tab-soul",
  },
  {
    value: "heartbeat",
    label: <Eyebrow>HEARTBEAT.md</Eyebrow>,
    testId: "agent-file-tab-heartbeat",
  },
];

interface InstructionsFileRowProps {
  file: AgentInstructionFile;
  onFileChange: (file: AgentInstructionFile) => void;
  children?: ReactNode;
}

/** The authored-file pills with the file's trailing action, shared by the tab and its skeleton. */
export function InstructionsFileRow({ file, onFileChange, children }: InstructionsFileRowProps) {
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <PillGroup
        aria-label="Authored files"
        data-testid="agent-instructions-files"
        items={FILE_ITEMS}
        onChange={onFileChange}
        size="md"
        value={file}
      />
      <span className="min-w-0 flex-1" />
      {children}
    </div>
  );
}
