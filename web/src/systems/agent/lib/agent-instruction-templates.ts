export interface AgentInstructionTemplate {
  id: string;
  label: string;
  body: string;
}

export const AGENT_INSTRUCTION_TEMPLATES: readonly AgentInstructionTemplate[] = [
  {
    id: "issues",
    label: "Watch issues",
    body: "Keep an eye on new issues.\n\n- Say what's blocked and who can unblock it\n- Suggest the next step\n- Stay quiet when nothing changed",
  },
  {
    id: "review",
    label: "Review code",
    body: "Read each change before it goes in.\n\n- Say what could break, and why\n- Point to the exact place\n- Skip style opinions",
  },
];
